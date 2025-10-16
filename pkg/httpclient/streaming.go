package httpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StreamClient handles Server-Sent Events streaming
type StreamClient struct {
	client   *Client
	events   chan EventStreamMessage
	errors   chan error
	done     chan struct{}
	cancel   context.CancelFunc
	response *http.Response
}

// StreamConfig configures the streaming client
type StreamConfig struct {
	// Topic to filter events (optional)
	Topic string

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

// Stream creates a new SSE streaming client for events
// This automatically subscribes to the topic and streams events in real-time
func (c *Client) Stream(ctx context.Context, config StreamConfig) (*StreamClient, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	config.SetDefaults()

	// Create cancellable context
	streamCtx, cancel := context.WithCancel(ctx)

	streamClient := &StreamClient{
		client: c,
		events: make(chan EventStreamMessage, config.BufferSize),
		errors: make(chan error, 10),
		done:   make(chan struct{}),
		cancel: cancel,
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

	// Close HTTP response if open
	if sc.response != nil {
		sc.response.Body.Close()
	}

	// Wait for streaming goroutine to finish
	<-sc.done

	return nil
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

	// Add topic filter if specified
	if config.Topic != "" {
		values := streamURL.Query()
		values.Set("topic", config.Topic)
		streamURL.RawQuery = values.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", streamURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create streaming request: %w", err)
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+sc.client.token)

	// Perform request
	resp, err := sc.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to stream: %w", err)
	}

	sc.response = resp
	defer func() {
		resp.Body.Close()
		sc.response = nil
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Process SSE stream
	return sc.processSSEStream(ctx, resp.Body)
}

// processSSEStream reads and parses Server-Sent Events
func (sc *StreamClient) processSSEStream(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Handle SSE format
		if strings.HasPrefix(line, "data: ") {
			// Extract JSON data
			jsonData := strings.TrimPrefix(line, "data: ")

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

			// Send event to channel
			select {
			case sc.events <- event:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Channel full, drop event (could add metrics here)
			}
		} else if strings.HasPrefix(line, ": ") {
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