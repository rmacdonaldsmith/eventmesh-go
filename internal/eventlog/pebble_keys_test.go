package eventlog

import (
	"bytes"
	"testing"
)

func TestPebbleEventKeyEncodingOrdersOffsets(t *testing.T) {
	topic := "orders/created"

	first := pebbleEventKey(topic, 1)
	second := pebbleEventKey(topic, 2)
	tenth := pebbleEventKey(topic, 10)

	if bytes.Compare(first, second) >= 0 {
		t.Fatalf("Expected offset 1 key to sort before offset 2 key")
	}
	if bytes.Compare(second, tenth) >= 0 {
		t.Fatalf("Expected offset 2 key to sort before offset 10 key")
	}
}

func TestPebbleEventKeyEncodingKeepsTopicsUnambiguous(t *testing.T) {
	topicWithSeparator := "orders/created"
	topicWithoutSeparator := "orders"

	withSeparatorPrefix := pebbleEventTopicPrefix(topicWithSeparator)
	withoutSeparatorPrefix := pebbleEventTopicPrefix(topicWithoutSeparator)

	if bytes.HasPrefix(pebbleEventKey(topicWithSeparator, 0), withoutSeparatorPrefix) {
		t.Fatalf("Expected topic %q not to be within topic %q prefix", topicWithSeparator, topicWithoutSeparator)
	}
	if !bytes.HasPrefix(pebbleEventKey(topicWithSeparator, 0), withSeparatorPrefix) {
		t.Fatalf("Expected event key to begin with exact length-prefixed topic prefix")
	}
}

func TestPebbleEventKeyPrefixBounds(t *testing.T) {
	topic := "orders/created"
	prefix := pebbleEventTopicPrefix(topic)
	upper := pebblePrefixUpperBound(prefix)

	for _, offset := range []uint64{0, 1, 42} {
		key := pebbleEventKey(topic, offset)
		if bytes.Compare(key, prefix) < 0 {
			t.Fatalf("Expected key for offset %d to be at or after lower prefix", offset)
		}
		if upper != nil && bytes.Compare(key, upper) >= 0 {
			t.Fatalf("Expected key for offset %d to be before upper prefix", offset)
		}
	}
}
