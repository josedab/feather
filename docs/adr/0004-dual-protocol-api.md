# ADR-0004: Dual Protocol API (gRPC + HTTP REST)

## Status

Accepted

## Context

Feather serves diverse clients:

1. **ML inference services**: Need lowest latency, often in same datacenter
2. **Data pipelines**: Batch operations, streaming reads
3. **Web applications**: Browser-based dashboards, admin UIs
4. **Multi-language clients**: Python, Java, Go, TypeScript, Rust

No single protocol serves all these use cases optimally:
- **gRPC**: Excellent for service-to-service, streaming, code generation
- **HTTP REST**: Universal compatibility, browser support, simpler debugging

We needed to support both without duplicating business logic.

## Decision

We expose **both gRPC and HTTP REST APIs** on separate ports:

### gRPC API (Port 50051)

```protobuf
service FeatureService {
  // Unary RPCs
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);

  // Streaming RPCs
  rpc GetFeaturesStream(GetFeaturesStreamRequest) returns (stream EntityFeaturesResponse);
  rpc PutFeaturesStream(stream PutFeaturesRequest) returns (PutFeaturesResponse);
}
```

**Use cases**:
- High-throughput inference pipelines
- Streaming feature updates
- Generated SDK clients (type-safe)
- Bidirectional streaming for real-time sync

### HTTP REST API (Port 8080)

```
GET  /v1/features?entity={id}&features={list}
POST /v1/features
POST /v1/features/batch
GET  /v1/features/history?entity={id}&as_of={timestamp}
GET  /v1/schema/groups
GET  /health
GET  /ready
GET  /live
```

**Use cases**:
- Browser-based dashboards
- Quick debugging with curl
- Webhooks and integrations
- Environments without gRPC support

### Shared Implementation

Both protocols delegate to the same storage layer:

```
gRPC Handler ─┐
              ├──► Storage Layer (Store)
HTTP Handler ─┘
```

No business logic in handlers; they only translate protocol to domain calls.

## Consequences

### Positive

- **Universal access**: Every client can use their preferred protocol
- **Streaming support**: gRPC enables efficient bulk operations
- **Type safety**: Generated clients from protobuf definitions
- **Browser compatibility**: REST API works without gRPC-web proxy
- **Operational flexibility**: Can disable one protocol if not needed
- **Independent scaling**: Could route protocols to different backends

### Negative

- **Two APIs to maintain**: Documentation, versioning, testing doubled
- **Port management**: Two ports to expose in Kubernetes
- **Consistency risk**: APIs could drift if not careful
- **Learning curve**: Developers must understand both protocols

### Neutral

- **No gRPC-web**: Browser gRPC would require envoy proxy; REST is simpler
- **No GraphQL**: Considered but added complexity without clear benefit

## Implementation Notes

### gRPC Server

Key file: `internal/server/grpc.go`

```go
type GRPCServer struct {
    pb.UnimplementedFeatureServiceServer
    store *storage.Store
}

func (s *GRPCServer) GetFeatures(ctx context.Context, req *pb.GetFeaturesRequest) (*pb.GetFeaturesResponse, error) {
    features, err := s.store.Get(req.EntityId, req.FeatureNames)
    // Convert domain types to protobuf
    return &pb.GetFeaturesResponse{Features: toPbFeatures(features)}, nil
}
```

### HTTP Server

Key file: `internal/server/http.go`

```go
func (s *HTTPServer) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
    entityID := r.URL.Query().Get("entity")
    features := strings.Split(r.URL.Query().Get("features"), ",")

    result, err := s.store.Get(entityID, features)
    // Convert domain types to JSON
    json.NewEncoder(w).Encode(result)
}
```

### Protobuf Definitions

Key file: `api/proto/feather.proto`

The protobuf definitions serve as the contract for:
- gRPC service implementation
- SDK code generation (Go, Python, TypeScript, Rust, Java)
- API documentation

### Port Configuration

```yaml
server:
  http:
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    port: 50051
    max_recv_msg_size: 16MB
    max_send_msg_size: 16MB
```
