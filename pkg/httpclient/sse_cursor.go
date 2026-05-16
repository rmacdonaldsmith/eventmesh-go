package httpclient

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const sseCursorVersion = 1

type sseEventCursor struct {
	Version int    `json:"v"`
	NodeID  string `json:"nodeId"`
	Topic   string `json:"topic"`
	Offset  int64  `json:"offset"`
}

type sseMultiTopicCursor struct {
	Version int              `json:"v"`
	NodeID  string           `json:"nodeId"`
	Topics  map[string]int64 `json:"topics"`
}

func encodeSSECursor(cursor interface{}) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal SSE cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeSSEEventCursor(raw string) (sseEventCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return sseEventCursor{}, fmt.Errorf("decode SSE cursor: %w", err)
	}

	var cursor sseEventCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return sseEventCursor{}, fmt.Errorf("parse SSE cursor: %w", err)
	}
	if cursor.Version != sseCursorVersion {
		return sseEventCursor{}, fmt.Errorf("unsupported SSE cursor version %d", cursor.Version)
	}
	if cursor.NodeID == "" {
		return sseEventCursor{}, fmt.Errorf("SSE cursor nodeId is required")
	}
	if cursor.Topic == "" {
		return sseEventCursor{}, fmt.Errorf("SSE cursor topic is required")
	}
	if cursor.Offset < 0 {
		return sseEventCursor{}, fmt.Errorf("SSE cursor offset must be non-negative")
	}
	return cursor, nil
}

func encodeSSEMultiTopicCursor(nodeID string, topics map[string]int64) (string, error) {
	if nodeID == "" || len(topics) == 0 {
		return "", nil
	}
	return encodeSSECursor(sseMultiTopicCursor{
		Version: sseCursorVersion,
		NodeID:  nodeID,
		Topics:  topics,
	})
}
