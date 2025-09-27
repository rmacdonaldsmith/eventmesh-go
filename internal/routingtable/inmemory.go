package routingtable

// TODO: InMemoryRoutingTable implementation
// This will implement the routingtable.RoutingTable interface
//
// Key features to implement:
// - Thread-safe in-memory storage of topic subscriptions
// - Wildcard pattern matching (REQ-RT-001)
// - Efficient topic lookup with O(log n) or better performance (REQ-RT-003)
// - Support for gossip-based rebuild (REQ-RT-002)
// - Support for both local clients and peer node subscribers
//
// Data structures to consider:
// - Trie or prefix tree for efficient wildcard matching
// - Hash maps for direct topic lookups
// - Concurrent-safe data structures using sync.RWMutex
//
// Pattern matching strategy:
// - Compile topic patterns into efficient matching rules
// - Support single-level wildcards (*) initially
// - Future: multi-level wildcards (**)