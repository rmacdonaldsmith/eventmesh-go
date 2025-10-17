package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client provides HTTP client for EventMesh API
type Client struct {
	config     Config
	httpClient *http.Client
	token      string
	baseURL    *url.URL
}

// NewClient creates a new EventMesh HTTP client
func NewClient(config Config) (*Client, error) {
	config.SetDefaults()

	// Validate required config
	if config.ServerURL == "" {
		return nil, fmt.Errorf("ServerURL is required")
	}
	if config.ClientID == "" {
		return nil, fmt.Errorf("ClientID is required")
	}

	// Parse base URL
	baseURL, err := url.Parse(config.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid ServerURL: %w", err)
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	client := &Client{
		config:     config,
		httpClient: httpClient,
		baseURL:    baseURL,
	}

	return client, nil
}

// Authenticate authenticates with the EventMesh server and stores the token
func (c *Client) Authenticate(ctx context.Context) error {
	authReq := map[string]string{
		"clientId": c.config.ClientID,
	}

	var authResp AuthResponse
	err := c.doRequest(ctx, "POST", "/api/v1/auth/login", authReq, &authResp, false)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	c.token = authResp.Token
	return nil
}

// PublishEvent publishes an event to a topic
func (c *Client) PublishEvent(ctx context.Context, topic string, payload interface{}) (*PublishResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	req := PublishRequest{
		Topic:   topic,
		Payload: payload,
	}

	var resp PublishResponse
	err := c.doRequest(ctx, "POST", "/api/v1/events", req, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	return &resp, nil
}

// CreateSubscription creates an explicit subscription to a topic
func (c *Client) CreateSubscription(ctx context.Context, topic string) (*SubscriptionResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	req := SubscriptionRequest{
		Topic: topic,
	}

	var resp SubscriptionResponse
	err := c.doRequest(ctx, "POST", "/api/v1/subscriptions", req, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return &resp, nil
}

// ListSubscriptions returns all subscriptions for this client
func (c *Client) ListSubscriptions(ctx context.Context) ([]SubscriptionResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	var subscriptions []SubscriptionResponse
	err := c.doRequest(ctx, "GET", "/api/v1/subscriptions", nil, &subscriptions, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}

	return subscriptions, nil
}

// DeleteSubscription removes a subscription by ID
func (c *Client) DeleteSubscription(ctx context.Context, subscriptionID string) error {
	if c.token == "" {
		return fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	path := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionID)
	err := c.doRequest(ctx, "DELETE", path, nil, nil, true)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	return nil
}

// GetHealth returns the health status of the EventMesh server
func (c *Client) GetHealth(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	err := c.doRequest(ctx, "GET", "/api/v1/health", nil, &resp, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get health status: %w", err)
	}

	return &resp, nil
}

// Admin Methods (require admin token)

// AdminListClients returns all connected clients (admin only)
func (c *Client) AdminListClients(ctx context.Context) (*AdminClientsResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	var resp AdminClientsResponse
	err := c.doRequest(ctx, "GET", "/api/v1/admin/clients", nil, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}

	return &resp, nil
}

// AdminListSubscriptions returns all subscriptions (admin only)
func (c *Client) AdminListSubscriptions(ctx context.Context) (*AdminSubscriptionsResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	var resp AdminSubscriptionsResponse
	err := c.doRequest(ctx, "GET", "/api/v1/admin/subscriptions", nil, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list all subscriptions: %w", err)
	}

	return &resp, nil
}

// AdminGetStats returns system statistics (admin only)
func (c *Client) AdminGetStats(ctx context.Context) (*AdminStatsResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	var resp AdminStatsResponse
	err := c.doRequest(ctx, "GET", "/api/v1/admin/stats", nil, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return &resp, nil
}

// doRequestWithQuery performs an HTTP request with query parameters and optional authentication
func (c *Client) doRequestWithQuery(ctx context.Context, method, path string, queryParams url.Values, reqBody interface{}, respBody interface{}, requireAuth bool) error {
	// Build full URL with query parameters
	u := &url.URL{Path: path}
	if len(queryParams) > 0 {
		u.RawQuery = queryParams.Encode()
	}
	fullURL := c.baseURL.ResolveReference(u)

	// Prepare request body
	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requireAuth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(bodyBytes))
		}
		return fmt.Errorf("API error (%d): %s - %s", resp.StatusCode, resp.Status, errResp.Error)
	}

	// Parse successful response
	if respBody != nil {
		if err := json.Unmarshal(bodyBytes, respBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// doRequest performs an HTTP request with optional authentication
func (c *Client) doRequest(ctx context.Context, method, path string, reqBody interface{}, respBody interface{}, requireAuth bool) error {
	return c.doRequestWithQuery(ctx, method, path, nil, reqBody, respBody, requireAuth)
}

// IsAuthenticated returns whether the client has a valid token
func (c *Client) IsAuthenticated() bool {
	return c.token != ""
}

// GetToken returns the current authentication token
func (c *Client) GetToken() string {
	return c.token
}

// SetToken sets the authentication token (useful for testing or token reuse)
func (c *Client) SetToken(token string) {
	c.token = token
}

// ReadEvents reads events from a topic starting at a given offset
func (c *Client) ReadEvents(ctx context.Context, topic string, offset int64, limit int) (*ReadEventsResponse, error) {
	if c.token == "" {
		return nil, fmt.Errorf("client not authenticated - call Authenticate() first")
	}

	// Build URL with query parameters
	path := fmt.Sprintf("/api/v1/topics/%s/events", url.PathEscape(topic))
	queryParams := url.Values{}
	if offset >= 0 {
		queryParams.Set("offset", fmt.Sprintf("%d", offset))
	}
	if limit > 0 {
		queryParams.Set("limit", fmt.Sprintf("%d", limit))
	}

	var resp ReadEventsResponse
	err := c.doRequestWithQuery(ctx, "GET", path, queryParams, nil, &resp, true)
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}

	return &resp, nil
}
