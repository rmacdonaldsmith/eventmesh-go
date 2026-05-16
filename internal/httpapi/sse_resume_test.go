package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteSSEMessageIncludesID(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	recorder := httptest.NewRecorder()
	message := EventStreamMessage{EventID: "cursor-1", Topic: "orders.created", Offset: 1}

	if err := setup.Server.handlers.writeSSEMessage(recorder, message); err != nil {
		t.Fatalf("writeSSEMessage failed: %v", err)
	}

	body := recorder.Body.String()
	if !strings.HasPrefix(body, "id: cursor-1\n") {
		t.Fatalf("Expected SSE id line, got %q", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Fatalf("Expected SSE data line, got %q", body)
	}
}

func TestWriteSSEMessageRequiresID(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	if err := setup.Server.handlers.writeSSEMessage(httptest.NewRecorder(), EventStreamMessage{Topic: "orders.created"}); err == nil {
		t.Fatal("Expected missing EventID to fail")
	}
}

func TestStreamEventsRejectsCursorForDifferentNode(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	claims, err := setup.Auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	cursor, err := encodeSSEEventCursor("other-node", "orders.created", 0)
	if err != nil {
		t.Fatalf("encodeSSEEventCursor failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", cursor)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, claims))

	recorder := httptest.NewRecorder()
	setup.Server.handlers.StreamEvents(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusConflict, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "different node") {
		t.Fatalf("Expected node mismatch message, got %s", recorder.Body.String())
	}
}

func TestStreamEventsResumeReplaysFromCursor(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	claims, err := setup.Auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	createSubscriptionForTest(t, setup.Server.handlers, setup.Auth, token, "orders.*")
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 1}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 2}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 3}`)

	cursor, err := encodeSSEEventCursor(setup.Node.GetNodeID(), "orders.created", 0)
	if err != nil {
		t.Fatalf("encodeSSEEventCursor failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ClaimsKey, claims))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream?since="+cursor, nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")

	recorder := NewStreamingRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		setup.Server.handlers.StreamEvents(recorder, req)
	}()

	replayedBody := waitForSSEDataMessages(t, recorder, 2)
	cancel()
	<-done

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	messages := parseSSEDataMessages(t, replayedBody)
	if len(messages) != 2 {
		t.Fatalf("Expected 2 replayed messages, got %d. Body: %s", len(messages), recorder.Body.String())
	}
	if messages[0].Offset != 1 || messages[1].Offset != 2 {
		t.Fatalf("Expected replay offsets [1,2], got [%d,%d]", messages[0].Offset, messages[1].Offset)
	}
	if messages[0].EventID == "" || messages[1].EventID == "" {
		t.Fatalf("Expected replay messages to include EventID: %#v", messages)
	}
}

func createSubscriptionForTest(t *testing.T, handlers *Handlers, auth *JWTAuth, token, topic string) {
	t.Helper()

	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(`{"topic":"`+topic+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, claims))

	recorder := httptest.NewRecorder()
	handlers.CreateSubscription(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("Expected subscription status %d, got %d. Body: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}
}

func waitForSSEDataMessages(t *testing.T, recorder *StreamingRecorder, expected int) string {
	t.Helper()

	var body strings.Builder
	for len(parseSSEDataMessages(t, body.String())) < expected {
		select {
		case chunk := <-recorder.Data:
			body.WriteString(chunk)
		case <-time.After(1 * time.Second):
			t.Fatalf("Timed out waiting for %d SSE data messages. Body: %s", expected, body.String())
		}
	}
	return body.String()
}

func parseSSEDataMessages(t *testing.T, body string) []EventStreamMessage {
	t.Helper()

	var messages []EventStreamMessage
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var message EventStreamMessage
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &message); err != nil {
			t.Fatalf("Failed to parse SSE data line %q: %v", line, err)
		}
		messages = append(messages, message)
	}
	return messages
}
