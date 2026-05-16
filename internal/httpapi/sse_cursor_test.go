package httpapi

import "testing"

func TestSSECursorRoundTrip(t *testing.T) {
	encoded, err := encodeSSEEventCursor("node-a", "orders.created", 42)
	if err != nil {
		t.Fatalf("encodeSSEEventCursor failed: %v", err)
	}

	cursor, err := decodeSSECursor(encoded)
	if err != nil {
		t.Fatalf("decodeSSECursor failed: %v", err)
	}

	if cursor.Version != sseCursorVersion {
		t.Fatalf("Expected version %d, got %d", sseCursorVersion, cursor.Version)
	}
	if cursor.NodeID != "node-a" {
		t.Fatalf("Expected node-a, got %q", cursor.NodeID)
	}
	if cursor.Topics["orders.created"] != 42 {
		t.Fatalf("Expected orders.created offset 42, got %d", cursor.Topics["orders.created"])
	}
}

func TestSSECursorDecodesMultiTopicCursor(t *testing.T) {
	encoded, err := encodeSSECursor(sseMultiTopicCursor{
		Version: sseCursorVersion,
		NodeID:  "node-a",
		Topics: map[string]int64{
			"orders.created":     42,
			"payments.completed": 9,
		},
	})
	if err != nil {
		t.Fatalf("encodeSSECursor failed: %v", err)
	}

	cursor, err := decodeSSECursor(encoded)
	if err != nil {
		t.Fatalf("decodeSSECursor failed: %v", err)
	}

	if cursor.NodeID != "node-a" {
		t.Fatalf("Expected node-a, got %q", cursor.NodeID)
	}
	if cursor.Topics["orders.created"] != 42 || cursor.Topics["payments.completed"] != 9 {
		t.Fatalf("Unexpected topics: %#v", cursor.Topics)
	}
}

func TestSSECursorRejectsInvalidCursor(t *testing.T) {
	if _, err := decodeSSECursor("not-base64"); err == nil {
		t.Fatal("Expected invalid base64 cursor to fail")
	}

	encoded, err := encodeSSECursor(sseEventCursor{Version: 99, NodeID: "node-a", Topic: "orders.created", Offset: 1})
	if err != nil {
		t.Fatalf("encodeSSECursor failed: %v", err)
	}
	if _, err := decodeSSECursor(encoded); err == nil {
		t.Fatal("Expected unsupported version to fail")
	}
}
