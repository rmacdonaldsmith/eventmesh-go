package meshnode

import "errors"

var (
	// ErrSubscriptionNotFound indicates that a requested subscription does not exist.
	ErrSubscriptionNotFound = errors.New("subscription not found")
	// ErrClientNotFound indicates that a requested client does not exist.
	ErrClientNotFound = errors.New("client not found")
)
