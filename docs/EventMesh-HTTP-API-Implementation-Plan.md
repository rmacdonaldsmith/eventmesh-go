# EventMesh HTTP API Implementation Plan - MVP Focused

## 🎯 Design Philosophy: Simple but Secure
- **Client isolation via JWT tokens** - Each client sees only their data
- **Basic admin API** - Essential for debugging and monitoring
- **Minimal complexity** - Focus on core functionality, defer advanced features
- **Easy to implement** - Leverage existing MeshNode methods directly

## 📝 MVP API Specification

### **Client APIs** (Self-Service, Token-Scoped)
```http
# Authentication
POST /api/v1/auth/login
Request: {"clientId": "my-app"}
Response: {"token": "jwt...", "clientId": "my-app", "expiresAt": "..."}

# Publishing
POST /api/v1/events
Request: {"topic": "orders.created", "payload": {"orderId": 123}}
Response: {"eventId": "evt_123", "offset": 42, "timestamp": "..."}

# Subscription Management (scoped to authenticated client)
GET    /api/v1/subscriptions           # My subscriptions only
POST   /api/v1/subscriptions           # Create my subscription
DELETE /api/v1/subscriptions/{id}      # Delete my subscription

# Event Streaming (Server-Sent Events)
GET /api/v1/events/stream?topic=orders.*
Response: text/event-stream with real-time events
```

### **Admin APIs** (Full System Access - For Debugging)
```http
# System Overview
GET /api/v1/admin/subscriptions        # All subscriptions system-wide
GET /api/v1/admin/clients              # All connected clients
GET /api/v1/admin/stats                # Event counts, throughput, etc.

# Health & Debugging
GET /api/v1/health                     # Same as existing mesh health
```

## 🏗️ Implementation Architecture - Keep It Simple

### **1. Package Structure** (Minimal)
```
internal/httpapi/
├── server.go          # HTTP server setup & routing
├── handlers.go        # All HTTP handlers in one file (MVP)
├── auth.go           # Simple JWT auth
├── types.go          # Request/response structs
└── middleware.go     # Basic auth middleware
```

### **2. Integration with MeshNode** (Direct Method Calls)
```go
// In handlers.go - Simple mapping to existing methods
func (h *Handler) publishEvent(w http.ResponseWriter, r *http.Request) {
    // 1. Parse JSON request
    // 2. Extract clientId from JWT
    // 3. Get/create TrustedClient
    // 4. Call node.PublishEvent(ctx, client, event) directly
    // 5. Return JSON response
}

func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
    // 1. Extract clientId from JWT
    // 2. Get/create TrustedClient
    // 3. Call node.Subscribe(ctx, client, topic) directly
    // 4. Return subscription info
}
```

### **3. Authentication Strategy** (MVP Simplicity)
```go
// Simple JWT with just clientId - no complex claims
type Claims struct {
    ClientID string `json:"client_id"`
    IsAdmin  bool   `json:"is_admin,omitempty"`
    jwt.StandardClaims
}

// Login just creates JWT for any clientId (like TrustedClient approach)
func login(clientId string) string {
    // Create JWT token
    // For MVP: No password validation (same as TrustedClient approach)
}
```

### **4. Server-Sent Events** (Simple Implementation)
```go
func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request) {
    // 1. Set SSE headers
    // 2. Create TrustedClient and subscribe
    // 3. Read from client.GetEventChannel()
    // 4. Write events to HTTP stream
    // 5. Handle client disconnect
}
```

## 🔧 Server Integration (Minimal Changes)

### **Add HTTP Server Option to cmd/eventmesh/main.go**
```go
// Add flags
var (
    enableHTTP = flag.Bool("http", false, "Enable HTTP API")
    httpPort   = flag.String("http-port", "8080", "HTTP API port")
)

// After starting mesh node:
if *enableHTTP {
    apiServer := httpapi.NewServer(node)
    go apiServer.Start(":" + *httpPort)
    log.Printf("🌐 HTTP API started on :%s", *httpPort)
}
```

## 🧪 Testing Plan

### **1. Manual Testing with curl**
```bash
# Start server with HTTP API
./bin/eventmesh --http --http-port 8080

# Authenticate
curl -X POST localhost:8080/api/v1/auth/login -d '{"clientId":"test-client"}'
# Returns: {"token":"eyJ..."}

# Subscribe (in background)
curl -H "Authorization: Bearer eyJ..." \
     localhost:8080/api/v1/events/stream?topic=test.* &

# Publish event
curl -X POST localhost:8080/api/v1/events \
     -H "Authorization: Bearer eyJ..." \
     -d '{"topic":"test.event","payload":{"msg":"hello"}}'

# Should see event appear in stream
```

### **2. CLI Client** (Phase 2)
```bash
# CLI stores token after auth
./bin/eventmesh-cli auth --server localhost:8080 --client-id publisher-1

# Use stored token
./bin/eventmesh-cli publish --topic orders.created --data '{"orderId":123}'
./bin/eventmesh-cli subscribe --topic orders.*
```

## 🎯 MVP Scope - What We're NOT Building Yet

### **Deferred for Later:**
- ❌ **Complex authentication** - No passwords, roles, permissions
- ❌ **Rate limiting** - Simple throttling only
- ❌ **Webhook delivery** - SSE only for now
- ❌ **Event history API** - Real-time streaming only
- ❌ **Advanced filtering** - Basic topic patterns only
- ❌ **Metrics/monitoring** - Basic health only
- ❌ **HTTPS/TLS** - HTTP only for MVP testing

### **MVP Focus:**
- ✅ **JWT-based client isolation** - Secure but simple
- ✅ **Real-time event streaming** - Core EventMesh value
- ✅ **Publish/Subscribe** - Essential functionality
- ✅ **Admin debugging APIs** - Critical for development
- ✅ **Direct MeshNode integration** - Leverage existing code

## 📈 Expected Implementation Time
- **HTTP server setup** - 1-2 hours
- **Authentication middleware** - 1 hour
- **Event publishing endpoint** - 1 hour
- **Subscription management** - 1-2 hours
- **Server-sent events streaming** - 2-3 hours
- **Admin endpoints** - 1 hour
- **Basic CLI client** - 2-3 hours

**Total: ~8-12 hours of focused development**

This plan delivers a **fully functional HTTP API** while keeping complexity minimal and maintaining the option to migrate to gRPC later. The admin APIs provide essential debugging capabilities without overcomplicating the client experience.

## 🚀 Implementation Phases

### **Phase 1: Core HTTP Infrastructure**
1. Create `internal/httpapi/` package
2. Basic HTTP server with routing
3. Simple JWT authentication
4. Request/response types

### **Phase 2: Event APIs**
1. Publish event endpoint
2. Server-sent events streaming
3. Basic error handling
4. Integration with MeshNode

### **Phase 3: Subscription Management**
1. Subscription CRUD endpoints
2. Client isolation logic
3. Admin debugging endpoints
4. Health endpoint

### **Phase 4: CLI Client**
1. Create `cmd/eventmesh-cli/`
2. Authentication command
3. Publish command
4. Subscribe command with real-time output

### **Phase 5: Testing & Polish**
1. Manual testing with curl
2. Integration tests
3. Error handling improvements
4. Documentation updates

## 📊 Success Criteria

- ✅ **Client can authenticate** and receive JWT token
- ✅ **Events can be published** via HTTP POST
- ✅ **Events stream in real-time** via Server-Sent Events
- ✅ **Subscriptions are client-isolated** (no data leakage)
- ✅ **Admin APIs work** for debugging
- ✅ **CLI client connects** to HTTP API successfully
- ✅ **End-to-end flow works** - publish on one client, receive on another
- ✅ **Multiple clients can connect** simultaneously
- ✅ **Server handles client disconnects** gracefully

This implementation provides a **production-ready HTTP API** for EventMesh while maintaining simplicity and keeping options open for future gRPC migration.