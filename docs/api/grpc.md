# gRPC API

## gRPC API

### Service Definition

```protobuf
syntax = "proto3";

package feather.v1;

service FeatureService {
  // Retrieve features for one or more entities.
  rpc GetFeatures(GetFeaturesRequest) returns (GetFeaturesResponse);

  // Retrieve features with server-side streaming for large results.
  rpc GetFeaturesStream(GetFeaturesRequest) returns (stream EntityFeaturesResponse);

  // Retrieve features as of a specific timestamp.
  rpc GetFeaturesAsOf(GetFeaturesAsOfRequest) returns (GetFeaturesResponse);

  // Store features for an entity.
  rpc PutFeatures(PutFeaturesRequest) returns (PutFeaturesResponse);
}

message GetFeaturesRequest {
  repeated string entities = 1;
  repeated string features = 2;
}

message GetFeaturesAsOfRequest {
  string entity_key = 1;
  repeated string features = 2;
  int64 as_of_timestamp = 3;  // Unix nanoseconds
}

message GetFeaturesResponse {
  map<string, EntityFeatures> entities = 1;
}

message EntityFeaturesResponse {
  string entity_key = 1;
  EntityFeatures features = 2;
}

message EntityFeatures {
  map<string, FeatureValue> features = 1;
}

message FeatureValue {
  oneof value {
    int64 int_value = 1;
    double double_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    bytes bytes_value = 5;
    VectorValue vector_value = 6;
  }
  int64 timestamp = 10;  // Unix nanoseconds
}

message VectorValue {
  repeated float values = 1;
}

message PutFeaturesRequest {
  string entity_key = 1;
  map<string, FeatureValue> features = 2;
  int64 version = 3;  // Optional, for conflict resolution
}

message PutFeaturesResponse {
  bool success = 1;
  string error = 2;
}

// Health check service (gRPC health checking protocol)
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}

message HealthCheckRequest {
  string service = 1;
}

message HealthCheckResponse {
  enum ServingStatus {
    UNKNOWN = 0;
    SERVING = 1;
    NOT_SERVING = 2;
    SERVICE_UNKNOWN = 3;
  }
  ServingStatus status = 1;
}
```

### Go Client Example

```go
package main

import (
    "context"
    "log"

    pb "github.com/feather-store/feather/api/featherpb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewFeatureServiceClient(conn)

    // Get features
    resp, err := client.GetFeatures(context.Background(), &pb.GetFeaturesRequest{
        Entities: []string{"user:123"},
        Features: []string{"click_count", "purchase_total"},
    })
    if err != nil {
        log.Fatal(err)
    }

    for entity, feats := range resp.Entities {
        for name, value := range feats.Features {
            log.Printf("%s/%s: %v\n", entity, name, value)
        }
    }
}
```

### Python Client Example

```python
import grpc
from feather.proto import feather_pb2 as pb
from feather.proto import feather_pb2_grpc as pb_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = pb_grpc.FeatureServiceStub(channel)

# Get features
response = stub.GetFeatures(pb.GetFeaturesRequest(
    entities=["user:123"],
    features=["click_count", "purchase_total"]
))

for entity, feats in response.entities.items():
    for name, value in feats.features.items():
        print(f"{entity}/{name}: {value}")
```

---

