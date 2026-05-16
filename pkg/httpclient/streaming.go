package httpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StreamClient handles Server-Sent Events streaming
type StreamClient struct {
	client                 *Client
	events                 chan EventStreamMessage
	errors                 chan error
	done                   chan struct{}
	cancel                 context.CancelFunc
	temporarySubscriptions map[string]string // topic -> subscription ID created by this stream
	resumeMu               sync.Mutex
	resumeNodeID           string
	resumeOffsets          map[string]int64
}

// StreamConfig configures the streaming client
type StreamConfig struct {
	// Topics to subscribe to and filter locally. When set, the client ensures
	// these subscriptions exist before each connection attempt and removes any
	// subscriptions it created when the stream closes.
	Topics []string

	// BufferSize for the event channel
	BufferSize int

	// ReconnectDelay for automatic reconnection
	ReconnectDelay time.Duration

	// MaxReconnectAttempts (0 = infinite)
	MaxReconnectAttempts int
}

// SetDefaults sets reasonable default values for StreamConfig
func (sc *StreamConfig) SetDefaults() {
	if sc.BufferSize == 0 {
		sc.BufferSize = 100
	}
	if sc.ReconnectDelay == 0 {
		sc.ReconnectDelay = 2 * time.Second
	}
}

// Stream creates a new SSE streaming client for events.
// If Topics is set, the client creates temporary subscriptions for missing
// topics, filters the unified SSE stream locally, re-ensures the subscriptions
// before reconnects, and removes subscriptions it created on Close.
func (c *Client) Stream(ctx context.Context, config StreamConfig) (*StreamClient, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	config.SetDefaults()

	temporarySubscriptions, err := c.ensureStreamSubscriptions(ctx, config.Topics)
	if err != nil {
		return nil, err
	}

	// Create cancellable context
	streamCtx, cancel := context.WithCancel(ctx)

	streamClient := &StreamClient{
		client:                 c,
		events:                 make(chan EventStreamMessage, config.BufferSize),
		errors:                 make(chan error, 10),
		done:                   make(chan struct{}),
		cancel:                 cancel,
		temporarySubscriptions: temporarySubscriptions,
		resumeOffsets:          make(map[string]int64),
	}

	// Start streaming in background
	go streamClient.startStreaming(streamCtx, config)

	return streamClient, nil
}

// Events returns the channel for receiving events
func (sc *StreamClient) Events() <-chan EventStreamMessage {
	return sc.events
}

// Errors returns the channel for receiving errors
func (sc *StreamClient) Errors() <-chan error {
	return sc.errors
}

// Done returns a channel that's closed when streaming ends
func (sc *StreamClient) Done() <-chan struct{} {
	return sc.done
}

// Close stops the streaming client and cleans up resources
func (sc *StreamClient) Close() error {
	sc.cancel()

	// Wait for streaming goroutine to finish
	<-sc.done

	if len(sc.temporarySubscriptions) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), sc.client.config.Timeout)
	defer cancel()

	var cleanupErr error
	for _, subscriptionID := range sc.temporarySubscriptions {
		if err := sc.client.DeleteSubscription(ctx, subscriptionID); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("subscription %s: %w", subscriptionID, err))
		}
	}
	sc.temporarySubscriptions = nil

	if cleanupErr != nil {
		return fmt.Errorf("failed to remove temporary stream subscriptions: %w", cleanupErr)
	}
	return nil
}

func uniqueStreamTopics(topics []string) []string {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		unique = append(unique, topic)
	}
	return unique
}

func (c *Client) ensureStreamSubscriptions(ctx context.Context, topics []string) (map[string]string, error) {
	uniqueTopics := uniqueStreamTopics(topics)
	if len(uniqueTopics) == 0 {
		return nil, nil
	}

	createdSubscriptions := make(map[string]string)
	for _, topic := range uniqueTopics {
		subscriptionID, err := c.ensureStreamSubscription(ctx, topic)
		if err != nil {
			return nil, err
		}
		if subscriptionID != "" {
			createdSubscriptions[topic] = subscriptionID
		}
	}
	return createdSubscriptions, nil
}

func (c *Client) ensureStreamSubscription(ctx context.Context, topic string) (string, error) {
	subscriptions, err := c.ListSubscriptions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to check existing subscriptions for stream topic %q: %w", topic, err)
	}

	for _, subscription := range subscriptions {
		if subscription.Topic == topic {
			return "", nil
		}
	}

	subscription, err := c.CreateSubscription(ctx, topic)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary stream subscription for topic %q: %w", topic, err)
	}

	return subscription.ID, nil
}

// startStreaming handles the SSE streaming loop with reconnection
func (sc *StreamClient) startStreaming(ctx context.Context, config StreamConfig) {
	defer close(sc.done)
	defer close(sc.events)
	defer close(sc.errors)

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if subscriptions, err := sc.client.ensureStreamSubscriptions(ctx, config.Topics); err != nil {
			select {
			case sc.errors <- fmt.Errorf("stream subscription error: %w", err):
			case <-ctx.Done():
				return
			default:
			}
		} else {
			if sc.temporarySubscriptions == nil {
				sc.temporarySubscriptions = make(map[string]string)
			}
			for topic, subscriptionID := range subscriptions {
				sc.temporarySubscriptions[topic] = subscriptionID
			}
		}

		err := sc.connectAndStream(ctx, config)
		if err != nil {
			select {
			case sc.errors <- fmt.Errorf("streaming error: %w", err):
			case <-ctx.Done():
				return
			default:
			}
		}

		// Check if we should reconnect
		if config.MaxReconnectAttempts > 0 && attempts >= config.MaxReconnectAttempts {
			select {
			case sc.errors <- fmt.Errorf("max reconnect attempts (%d) exceeded", config.MaxReconnectAttempts):
			case <-ctx.Done():
			}
			return
		}

		attempts++

		// Wait before reconnecting
		select {
		case <-time.After(config.ReconnectDelay):
		case <-ctx.Done():
			return
		}
	}
}

// connectAndStream establishes SSE connection and processes events
func (sc *StreamClient) connectAndStream(ctx context.Context, config StreamConfig) error {
	// Build streaming URL
	streamURL := sc.client.baseURL.ResolveReference(&url.URL{Path: "/api/v1/events/stream"})

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", streamURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create streaming request: %w", err)
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+sc.client.token)
	if resumeCursor, err := sc.resumeCursorHeader(); err != nil {
		return err
	} else if resumeCursor != "" {
		req.Header.Set("Last-Event-ID", resumeCursor)
	}

	// Perform request
	resp, err := sc.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to stream: %w", err)
	}

	defer func() {
		_ = resp.Body.Close() // Ignore close errors
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Process SSE stream
	return sc.processSSEStream(ctx, resp.Body, config)
}

// processSSEStream reads and parses Server-Sent Events
func (sc *StreamClient) processSSEStream(ctx context.Context, reader io.Reader, config StreamConfig) error {
	scanner := bufio.NewScanner(reader)
	var currentEventID string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Handle SSE format
		if fieldValue, ok := parseSSEField(line, "id"); ok {
			currentEventID = strings.TrimSpace(fieldValue)
			continue
		}

		if fieldValue, ok := parseSSEField(line, "data"); ok {
			// Extract JSON data
			jsonData := fieldValue

			// Parse event message
			var event EventStreamMessage
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				// Send error but continue processing
				select {
				case sc.errors <- fmt.Errorf("failed to parse event: %w", err):
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				continue
			}

			if event.EventID == "" {
				event.EventID = currentEventID
			}
			sc.recordResumeEventID(event.EventID)
			currentEventID = ""

			if len(config.Topics) > 0 && !matchesAnyStreamTopic(config.Topics, event.Topic) {
				continue
			}

			// Send event to channel
			select {
			case sc.events <- event:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Channel full, drop event (could add metrics here)
			}
		} else if strings.HasPrefix(line, ":") {
			// Keepalive comment - ignore but could log for debugging
			continue
		} else if line == "" {
			// Empty line separates events - ignore
			continue
		}
		// Other SSE fields (id:, event:, retry:) - ignore for now
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading SSE stream: %w", err)
	}

	return nil
}

func parseSSEField(line, field string) (string, bool) {
	prefix := field + ":"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}

	value := strings.TrimPrefix(line, prefix)
	value = strings.TrimPrefix(value, " ")
	return value, true
}

func (sc *StreamClient) resumeCursorHeader() (string, error) {
	sc.resumeMu.Lock()
	defer sc.resumeMu.Unlock()

	if len(sc.resumeOffsets) == 0 {
		return "", nil
	}

	offsets := make(map[string]int64, len(sc.resumeOffsets))
	for topic, offset := range sc.resumeOffsets {
		offsets[topic] = offset
	}
	return encodeSSEMultiTopicCursor(sc.resumeNodeID, offsets)
}

func (sc *StreamClient) recordResumeEventID(eventID string) {
	if eventID == "" {
		return
	}

	cursor, err := decodeSSEEventCursor(eventID)
	if err != nil {
		return
	}

	sc.resumeMu.Lock()
	defer sc.resumeMu.Unlock()

	if sc.resumeNodeID == "" {
		sc.resumeNodeID = cursor.NodeID
	}
	if sc.resumeNodeID != cursor.NodeID {
		sc.resumeNodeID = cursor.NodeID
		sc.resumeOffsets = make(map[string]int64)
	}
	if current, ok := sc.resumeOffsets[cursor.Topic]; !ok || cursor.Offset > current {
		sc.resumeOffsets[cursor.Topic] = cursor.Offset
	}
}

func matchesAnyStreamTopic(patterns []string, topic string) bool {
	for _, pattern := range uniqueStreamTopics(patterns) {
		if matchesStreamTopic(pattern, topic) {
			return true
		}
	}
	return false
}

func matchesStreamTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}

	patternParts := strings.Split(pattern, ".")
	topicParts := strings.Split(topic, ".")
	if len(patternParts) != len(topicParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != topicParts[i] {
			return false
		}
	}

	return true
}
