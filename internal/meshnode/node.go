package meshnode

// TODO: MeshNode orchestrator implementation
// This will implement the meshnode.MeshNode interface
//
// Key features to implement:
// - Orchestration of EventLog, RoutingTable, and PeerLink components
// - Client authentication and session management (REQ-MNODE-001)
// - Event publishing with local persistence first (REQ-MNODE-002)
// - Subscription propagation via gossip protocol (REQ-MNODE-003)
// - Health monitoring and status reporting
// - Graceful startup and shutdown procedures
//
// Component interactions:
// 1. EventLog: Store events locally before forwarding
// 2. RoutingTable: Determine which subscribers need each event
// 3. PeerLink: Forward events to remote mesh nodes
// 4. Gossip: Propagate subscription and membership changes
//
// Client handling:
// - Accept gRPC client connections with mTLS
// - Authenticate clients using certificates or tokens
// - Maintain client session state and subscriptions
// - Handle client disconnections gracefully
//
// Event flow:
// 1. Client publishes event to MeshNode
// 2. MeshNode persists event to local EventLog
// 3. MeshNode queries RoutingTable for interested subscribers
// 4. For local subscribers: deliver directly
// 5. For remote subscribers: forward via PeerLink
// 6. Propagate subscription changes via gossip
//
// Configuration needs:
// - Node ID and cluster membership
// - mTLS certificates and CA
// - Storage configuration (RocksDB path, etc.)
// - Network addresses and ports
// - Health check intervals
// - Gossip protocol settings