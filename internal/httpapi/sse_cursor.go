package httpapi

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

func encodeSSEEventCursor(nodeID, topic string, offset int64) (string, error) {
	return encodeSSECursor(sseEventCursor{
		Version: sseCursorVersion,
		NodeID:  nodeID,
		Topic:   topic,
		Offset:  offset,
	})
}

func encodeSSECursor(cursor interface{}) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal SSE cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeSSECursor(raw string) (sseMultiTopicCursor, error) {
	if raw == "" {
		return sseMultiTopicCursor{}, fmt.Errorf("SSE cursor is required")
	}

	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return sseMultiTopicCursor{}, fmt.Errorf("decode SSE cursor: %w", err)
	}

	var envelope struct {
		Version int              `json:"v"`
		NodeID  string           `json:"nodeId"`
		Topic   string           `json:"topic"`
		Offset  int64            `json:"offset"`
		Topics  map[string]int64 `json:"topics"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return sseMultiTopicCursor{}, fmt.Errorf("parse SSE cursor: %w", err)
	}
	if envelope.Version != sseCursorVersion {
		return sseMultiTopicCursor{}, fmt.Errorf("unsupported SSE cursor version %d", envelope.Version)
	}
	if envelope.NodeID == "" {
		return sseMultiTopicCursor{}, fmt.Errorf("SSE cursor nodeId is required")
	}

	if len(envelope.Topics) > 0 {
		if err := validateSSECursorTopics(envelope.Topics); err != nil {
			return sseMultiTopicCursor{}, err
		}
		return sseMultiTopicCursor{Version: envelope.Version, NodeID: envelope.NodeID, Topics: envelope.Topics}, nil
	}

	if envelope.Topic == "" {
		return sseMultiTopicCursor{}, fmt.Errorf("SSE cursor topic is required")
	}
	if envelope.Offset < 0 {
		return sseMultiTopicCursor{}, fmt.Errorf("SSE cursor offset must be non-negative")
	}

	return sseMultiTopicCursor{
		Version: envelope.Version,
		NodeID:  envelope.NodeID,
		Topics:  map[string]int64{envelope.Topic: envelope.Offset},
	}, nil
}

func validateSSECursorTopics(topics map[string]int64) error {
	for topic, offset := range topics {
		if topic == "" {
			return fmt.Errorf("SSE cursor topic is required")
		}
		if offset < 0 {
			return fmt.Errorf("SSE cursor offset must be non-negative")
		}
	}
	return nil
}
