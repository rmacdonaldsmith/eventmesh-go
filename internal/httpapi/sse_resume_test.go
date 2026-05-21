package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestStreamEventsRejectsInvalidResumeCursor(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	claims, err := setup.Auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream?since=not-base64", nil)
	req.Header.Set("Accept", "text/event-stream")
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, claims))

	recorder := httptest.NewRecorder()
	setup.Server.handlers.StreamEvents(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Invalid SSE resume cursor") {
		t.Fatalf("Expected invalid cursor message, got %s", recorder.Body.String())
	}
}

func TestStreamEventsRejectsResumeCursorWithTooManyTopics(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	claims, err := setup.Auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	topics := make(map[string]int64, maxSSEResumeTopics+1)
	for i := 0; i <= maxSSEResumeTopics; i++ {
		topics[fmt.Sprintf("orders.%d", i)] = 0
	}
	cursor, err := encodeSSECursor(sseMultiTopicCursor{Version: sseCursorVersion, NodeID: setup.Node.GetNodeID(), Topics: topics})
	if err != nil {
		t.Fatalf("encodeSSECursor failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", cursor)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, claims))

	recorder := httptest.NewRecorder()
	setup.Server.handlers.StreamEvents(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	}
	if setup.Server.handlers.sseResumeReplayLimitExceeded.Load() != 1 {
		t.Fatalf("Expected replay limit counter to increment")
	}
}

func TestValidateSSEReplayPlanRejectsReplayEventBudgetOverflow(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	createSubscriptionForTest(t, setup.Server.handlers, setup.Auth, token, "orders.*")
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 1}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 2}`)

	cursor := sseMultiTopicCursor{Version: sseCursorVersion, NodeID: setup.Node.GetNodeID(), Topics: map[string]int64{"orders.created": -1}}
	if err := setup.Server.handlers.validateSSEReplayPlan(context.Background(), cursor, []string{"orders.*"}, 1, maxSSEResumeTopics); !errors.Is(err, errSSEReplayLimitExceeded) {
		t.Fatalf("Expected replay limit error, got %v", err)
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

func TestEventToSSEMessageUsesLocalEventLogCursor(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	published := publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 1}`)
	events, err := setup.Node.GetEventLog().ReadEvents(context.Background(), "orders.created", published.Offset, 1)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 persisted event, got %d", len(events))
	}

	message, err := setup.Server.handlers.eventToSSEMessage(events[0])
	if err != nil {
		t.Fatalf("eventToSSEMessage failed: %v", err)
	}

	cursor, err := decodeSSECursor(message.EventID)
	if err != nil {
		t.Fatalf("decodeSSECursor failed: %v", err)
	}
	if cursor.NodeID != setup.Node.GetNodeID() {
		t.Fatalf("Expected cursor node %q, got %q", setup.Node.GetNodeID(), cursor.NodeID)
	}
	if cursor.Topics["orders.created"] != published.Offset {
		t.Fatalf("Expected cursor offset %d, got %d", published.Offset, cursor.Topics["orders.created"])
	}
}

func TestStreamEventsResumeReplaysMultiTopicCursor(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	claims, err := setup.Auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	createSubscriptionForTest(t, setup.Server.handlers, setup.Auth, token, "orders.*")
	createSubscriptionForTest(t, setup.Server.handlers, setup.Auth, token, "payments.*")
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 1}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "orders.created", `{"sequence": 2}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "payments.completed", `{"sequence": 1}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "payments.completed", `{"sequence": 2}`)

	cursor, err := encodeSSECursor(sseMultiTopicCursor{
		Version: sseCursorVersion,
		NodeID:  setup.Node.GetNodeID(),
		Topics: map[string]int64{
			"orders.created":     0,
			"payments.completed": 0,
		},
	})
	if err != nil {
		t.Fatalf("encodeSSECursor failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ClaimsKey, claims))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", cursor)

	recorder := NewStreamingRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		setup.Server.handlers.StreamEvents(recorder, req)
	}()

	replayedBody := waitForSSEDataMessages(t, recorder, 2)
	cancel()
	<-done

	messages := parseSSEDataMessages(t, replayedBody)
	seen := map[string]int64{}
	for _, message := range messages {
		seen[message.Topic] = message.Offset
	}
	if seen["orders.created"] != 1 || seen["payments.completed"] != 1 {
		t.Fatalf("Expected replay of both topics at offset 1, got %#v from body %s", seen, replayedBody)
	}
}

func TestStreamEventsResumeSkipsUnauthorizedCursorTopics(t *testing.T) {
	setup := NewTestServerSetup(t)
	defer setup.Close()

	token := setup.GenerateTestToken(t, "resume-client", false)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "payments.completed", `{"sequence": 1}`)
	publishEventForTest(t, setup.Server.handlers, setup.Auth, token, "payments.completed", `{"sequence": 2}`)

	cursor := sseMultiTopicCursor{
		Version: sseCursorVersion,
		NodeID:  setup.Node.GetNodeID(),
		Topics:  map[string]int64{"payments.completed": 0},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	recorder := httptest.NewRecorder()

	if err := setup.Server.handlers.replaySSECursor(recorder, req, cursor, []string{"orders.*"}); err != nil {
		t.Fatalf("replaySSECursor failed: %v", err)
	}
	if messages := parseSSEDataMessages(t, recorder.Body.String()); len(messages) != 0 {
		t.Fatalf("Expected no replayed messages for unauthorized cursor topic, got %#v", messages)
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
