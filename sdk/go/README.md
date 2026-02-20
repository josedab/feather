# Feather Go SDK

The Go SDK package lives at [`sdk/go/feather/`](./feather/).

## Installation

```bash
go get github.com/feather-store/feather/sdk/go/feather
```

## Requirements

- Go 1.21 or later
- Feather server running (default: `http://localhost:8080`)

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/feather-store/feather/sdk/go/feather"
)

func main() {
    client := feather.NewClient("http://localhost:8080", "your-api-key", nil)
    ctx := context.Background()

    // Store features
    err := client.Features.Put(ctx, &feather.PutRequest{
        EntityID: "user:123",
        Features: map[string]interface{}{
            "purchase_count": 42,
            "avg_order_value": 55.99,
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve features
    resp, err := client.Features.Get(ctx, "user:123", []string{"purchase_count"})
    if err != nil {
        log.Fatal(err)
    }

    for name, val := range resp.Features {
        fmt.Printf("%s: %v\n", name, val.Value)
    }
}
```

## Documentation

- **Full API reference:** [sdk/go/feather/README.md](./feather/README.md)
- **Quickstart guide:** [sdk/go/feather/quickstart/](./feather/quickstart/)
- **Examples:** [sdk/go/feather/examples/](./feather/examples/)
- **GoDoc:** [pkg.go.dev/github.com/feather-store/feather/sdk/go/feather](https://pkg.go.dev/github.com/feather-store/feather/sdk/go/feather)

## License

Apache 2.0 — See [LICENSE](../../LICENSE) for details.
