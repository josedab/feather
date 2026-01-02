# ADR-0011: Multi-Language SDK Generation from Protobuf

## Status

Accepted

## Context

Feather serves clients written in many languages:

- **Python**: Data scientists, ML pipelines, Jupyter notebooks
- **Go**: Backend services, infrastructure tools
- **TypeScript/JavaScript**: Web applications, Node.js services
- **Java/Kotlin**: Enterprise applications, Android
- **Rust**: Performance-critical systems, embedded

Maintaining separate SDK implementations for each language would be:
- **Expensive**: 5x the development and maintenance effort
- **Error-prone**: APIs could drift between languages
- **Slow**: New features would take months to propagate

We needed a strategy that:
1. Ensures API consistency across all languages
2. Reduces maintenance burden
3. Enables rapid feature propagation
4. Provides type-safe, idiomatic clients

## Decision

We use **Protocol Buffers (protobuf) as the single source of truth** and generate SDKs for all supported languages.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   api/proto/feather.proto                   │
│                   (Single Source of Truth)                  │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ protoc + language plugins
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
   ┌─────────┐       ┌─────────┐       ┌─────────┐
   │ sdk/go  │       │sdk/python│      │sdk/rust │
   │         │       │         │       │         │
   │ • Types │       │ • Types │       │ • Types │
   │ • gRPC  │       │ • gRPC  │       │ • gRPC  │
   │ • HTTP  │       │ • HTTP  │       │ • HTTP  │
   └─────────┘       └─────────┘       └─────────┘
        │                 │                 │
        ▼                 ▼                 ▼
   ┌─────────┐       ┌─────────┐       ┌─────────┐
   │Idiomatic│       │Idiomatic│       │Idiomatic│
   │ Wrapper │       │ Wrapper │       │ Wrapper │
   └─────────┘       └─────────┘       └─────────┘
```

### Protobuf Definitions

Key file: `api/proto/feather.proto`

```protobuf
syntax = "proto3";
package feather.v1;

service FeatureService {
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);
  rpc GetFeaturesStream(GetFeaturesStreamRequest) returns (stream EntityFeaturesResponse);
}

message GetFeaturesRequest {
  string entity_id = 1;
  repeated string feature_names = 2;
}

message FeatureValue {
  oneof value {
    int64 int_value = 1;
    double float_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    bytes bytes_value = 5;
  }
  int64 timestamp = 10;
  int64 version = 11;
}
```

### SDK Structure

Each SDK follows this pattern:

```
sdk/{language}/
├── generated/          # Auto-generated from protobuf
│   ├── feather.pb.*    # Message types
│   └── feather_grpc.*  # gRPC client stubs
├── client.{ext}        # High-level client wrapper
├── types.{ext}         # Language-idiomatic type aliases
├── examples/           # Usage examples
└── README.md           # Language-specific docs
```

### Supported Languages

| Language | Package | Notes |
|----------|---------|-------|
| Go | `sdk/go/feather` | Native gRPC, context support |
| Python | `sdk/python/feather` | NumPy/Pandas integration |
| TypeScript | `sdk/typescript` | Browser + Node.js |
| Rust | `sdk/rust` | Async/await, Tokio runtime |
| Java/Kotlin | `sdk/java` | Android compatible |

## Consequences

### Positive

- **Consistency**: All SDKs have identical API surface
- **Type safety**: Generated code catches errors at compile time
- **Rapid propagation**: New API → regenerate → all languages updated
- **Low maintenance**: Fix bugs once in proto, regenerate
- **Documentation**: Proto comments become SDK docs
- **Streaming support**: gRPC streaming works in all languages

### Negative

- **Protobuf learning curve**: Teams must understand proto3 syntax
- **Build complexity**: Need protoc and plugins for each language
- **Generated code**: Can be verbose, sometimes not idiomatic
- **Wrapper maintenance**: Idiomatic wrappers still need per-language work

### Neutral

- **Version coupling**: SDK versions tied to proto versions
- **Breaking changes**: Proto changes require coordinated releases

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Manual SDK per language | 5x maintenance; APIs would drift |
| OpenAPI/Swagger | No streaming support; less efficient |
| GraphQL | Overkill for feature store; added complexity |
| Thrift | Less ecosystem support than protobuf |

## Implementation Notes

### Code Generation

Makefile target:
```makefile
generate-protos:
	protoc --proto_path=api/proto \
		--go_out=sdk/go --go_opt=paths=source_relative \
		--go-grpc_out=sdk/go --go-grpc_opt=paths=source_relative \
		--python_out=sdk/python \
		--grpc_python_out=sdk/python \
		--plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
		--ts_out=sdk/typescript \
		api/proto/feather.proto
```

### Go SDK Example

```go
// sdk/go/feather/client.go
package feather

import (
    "context"
    pb "github.com/feather-store/feather/sdk/go/feather/generated"
    "google.golang.org/grpc"
)

type Client struct {
    conn   *grpc.ClientConn
    client pb.FeatureServiceClient
}

func NewClient(addr string, opts ...grpc.DialOption) (*Client, error) {
    conn, err := grpc.Dial(addr, opts...)
    if err != nil {
        return nil, err
    }
    return &Client{
        conn:   conn,
        client: pb.NewFeatureServiceClient(conn),
    }, nil
}

func (c *Client) GetFeatures(ctx context.Context, entityID string, features []string) (map[string]interface{}, error) {
    resp, err := c.client.GetFeatures(ctx, &pb.GetFeaturesRequest{
        EntityId:     entityID,
        FeatureNames: features,
    })
    if err != nil {
        return nil, err
    }
    return convertFeatures(resp.Features), nil
}
```

### Python SDK Example

```python
# sdk/python/feather/client.py
import grpc
from feather.generated import feather_pb2, feather_pb2_grpc

class FeatherClient:
    def __init__(self, address: str):
        self.channel = grpc.insecure_channel(address)
        self.stub = feather_pb2_grpc.FeatureServiceStub(self.channel)

    def get_features(self, entity_id: str, features: list[str]) -> dict:
        request = feather_pb2.GetFeaturesRequest(
            entity_id=entity_id,
            feature_names=features,
        )
        response = self.stub.GetFeatures(request)
        return self._convert_features(response.features)

    def get_features_df(self, entity_ids: list[str], features: list[str]) -> pd.DataFrame:
        """Pandas-friendly batch retrieval"""
        # ...
```

### TypeScript SDK Example

```typescript
// sdk/typescript/src/client.ts
import { FeatureServiceClient } from './generated/feather_grpc_pb';
import { GetFeaturesRequest } from './generated/feather_pb';

export class FeatherClient {
  private client: FeatureServiceClient;

  constructor(address: string) {
    this.client = new FeatureServiceClient(address, grpc.credentials.createInsecure());
  }

  async getFeatures(entityId: string, features: string[]): Promise<Record<string, any>> {
    const request = new GetFeaturesRequest();
    request.setEntityId(entityId);
    request.setFeatureNamesList(features);

    return new Promise((resolve, reject) => {
      this.client.getFeatures(request, (err, response) => {
        if (err) reject(err);
        else resolve(this.convertFeatures(response));
      });
    });
  }
}
```
