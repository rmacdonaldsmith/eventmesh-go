# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of an event mesh system for secure, distributed event routing. This project was ported from a C#/.NET implementation and follows the same architecture and design principles. See `design.md` for complete architecture and design details.

## Development Workflow

### Build Commands
- **Build all packages**: `go build ./...` (from project root)
- **Run tests**: `go test ./...` (from project root)
- **Run specific test package**: `go test ./tests` or `go test ./internal/eventlog`
- **Run tests with verbose output**: `go test ./tests -v`
- **Format code**: `go fmt ./...`
- **Lint code**: `go vet ./...`
- **Clean module cache**: `go clean -modcache`

### Project Structure (Go Packages)

```
eventmesh-go/
├── go.mod                           # Go module definition
├── go.sum                          # Go module checksums (auto-generated)
├── design.md            # System design document
├── pkg/                           # Public API packages
│   └── eventlog/
│       ├── interfaces.go           # EventRecord + EventLog interfaces
│       ├── doc.go                  # Package documentation with examples
│       └── interfaces_test.go      # Basic interface compilation tests
├── internal/                      # Private implementation packages
│   └── eventlog/
│       ├── record.go              # Record struct (implements EventRecord)
│       ├── memory.go              # InMemoryEventLog implementation
│       └── memory_test.go         # Basic implementation tests
├── tests/                         # Integration/comprehensive tests
│   └── eventlog_test.go          # Full test suite (24 tests)
├── cmd/                          # Future executable entry points
│   └── eventmesh/
│       └── main.go               # (Future) main executable
└── CLAUDE.md                     # This file
```

### Package Dependencies
- **pkg/eventlog**: No dependencies (pure interfaces)
- **internal/eventlog**: pkg/eventlog (implementations of public interfaces)
- **tests**: pkg/eventlog + internal/eventlog (comprehensive testing)
- **cmd**: pkg/eventlog + internal/eventlog (future executables)

**Benefits of this Structure:**
- **Clean API Separation**: Public interfaces in `pkg/`, implementations in `internal/`
- **No Circular Dependencies**: Unidirectional dependency flow
- **Interface-Based Design**: Easy testing and mocking via interfaces
- **Go Conventions**: Follows standard Go project layout

## Key References

- **Design Document**: `design.md` - Complete system design, architecture, and requirements
- **License**: MIT License (see LICENSE file)
- **Original C# Implementation**: Available in separate repository for reference

## Development Guidelines

### Test-Driven Development (TDD)
This project follows strict TDD practices (ported from C# implementation):
- **Comprehensive Test Coverage**: 24 tests covering all functionality and edge cases
- **Red-Green-Refactor cycle**: All code was developed test-first
- **Error Handling**: All error conditions are tested with explicit Go error types
- **Concurrency Testing**: Thread safety validated with goroutines and sync primitives
- **Interface Testing**: Both public interfaces and private implementations are tested

### Go Best Practices
- **Explicit Error Handling**: All methods return explicit errors instead of exceptions
- **Context Cancellation**: Use `context.Context` for cancellation and timeouts
- **Channel-Based Async**: Use Go channels instead of async/await patterns
- **Thread Safety**: Use `sync.RWMutex` for concurrent access protection
- **Interface Compliance**: Compile-time interface verification with `var _ Interface = (*Type)(nil)`
- **Defensive Copying**: Return copies of slices/maps to prevent mutation

### Go Idioms Applied
- **Constructor Functions**: `NewRecord()`, `NewInMemoryEventLog()` instead of constructors
- **Method Receivers**: Interface methods implemented on struct types
- **Channel Patterns**: `Replay()` returns `(<-chan EventRecord, <-chan error)` for streaming
- **Resource Management**: `io.Closer` interface instead of IDisposable
- **Error Types**: Predefined error variables (`ErrNegativeOffset`, etc.)

### Technical Architecture
- Follow the requirements defined in `design.md`
- Current implementation: In-memory storage (future: RocksDB)
- Future communication: gRPC/mTLS (as per design document)
- Event Log Requirements: REQ-LOG-001 through REQ-LOG-004 implemented
- Security: mTLS and client authentication (future implementation)

## Port from C# Summary

This Go implementation was systematically ported from a C#/.NET codebase:

**Ported Components:**
- ✅ `IEventRecord` interface → `EventRecord` interface
- ✅ `EventRecord` class → `Record` struct
- ✅ `IEventLog` interface → `EventLog` interface
- ✅ `InMemoryEventLog` class → `InMemoryEventLog` struct
- ✅ Comprehensive test suite (23 C# tests → 24 Go tests)

**Key Adaptations:**
- **Async Patterns**: `IAsyncEnumerable<T>` → Go channels
- **Error Handling**: Exceptions → explicit error returns
- **Concurrency**: `Task`/`async`/`await` → goroutines/channels/contexts
- **Resource Management**: `IDisposable` → `io.Closer`
- **Type System**: Records/properties → structs/methods
- **Thread Safety**: Built-in locks → `sync.RWMutex`

**Test Coverage:**
- All 23 original C# test cases successfully ported
- 1 additional test case added for Go-specific behavior
- 100% pass rate with comprehensive edge case and error handling coverage
- Concurrent access patterns validated with Go concurrency primitives

## Post-MVP Backlog

**Performance Testing and Optimization:**
- RoutingTable performance benchmarks for topic matching operations
- Load testing for concurrent subscription operations
- Memory usage profiling for large-scale topic/subscriber counts
- Comparison benchmarks between different data structures (maps vs tries)

## Future Development

As development progresses, consider adding:
- RocksDB-backed EventLog implementation
- gRPC communication layer with mTLS
- Additional event mesh components (Routing Table, Peer Links)
- Performance benchmarks
- Integration tests with external systems
- CLI tools in `cmd/` directory