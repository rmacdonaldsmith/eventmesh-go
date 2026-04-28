package eventlog

import "encoding/binary"

const (
	pebbleEventKeyPrefix  byte = 'e'
	pebbleOffsetKeyPrefix byte = 'o'
)

func pebbleEventTopicPrefix(topic string) []byte {
	return pebbleTopicPrefix(pebbleEventKeyPrefix, topic)
}

func pebbleOffsetKey(topic string) []byte {
	return pebbleTopicPrefix(pebbleOffsetKeyPrefix, topic)
}

func pebbleEventKey(topic string, offset uint64) []byte {
	key := pebbleEventTopicPrefix(topic)
	key = binary.BigEndian.AppendUint64(key, offset)
	return key
}

func pebbleTopicPrefix(prefix byte, topic string) []byte {
	key := make([]byte, 0, 1+4+len(topic))
	key = append(key, prefix)
	key = binary.BigEndian.AppendUint32(key, uint32(len(topic)))
	key = append(key, topic...)
	return key
}

func pebblePrefixUpperBound(prefix []byte) []byte {
	upper := append([]byte(nil), prefix...)
	for i := len(upper) - 1; i >= 0; i-- {
		if upper[i] != 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return nil
}

func encodePebbleOffset(offset uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], offset)
	return encoded[:]
}

func decodePebbleOffset(encoded []byte) uint64 {
	return binary.BigEndian.Uint64(encoded)
}

func decodePebbleOffsetTopicKey(key []byte) (string, bool) {
	if len(key) < 5 || key[0] != pebbleOffsetKeyPrefix {
		return "", false
	}

	topicLen := int(binary.BigEndian.Uint32(key[1:5]))
	if len(key) != 5+topicLen {
		return "", false
	}

	return string(key[5:]), true
}
