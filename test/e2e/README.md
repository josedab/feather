# Feather E2E Tests

End-to-end integration tests for the Feather feature store.

## Prerequisites

- Docker and Docker Compose
- Go 1.24+

## Running with Docker Compose

```bash
# Start the services
cd test/e2e
docker compose up -d --build

# Wait for health check to pass
docker compose ps

# Run the tests
FEATHER_E2E_URL=http://localhost:8080 go test -tags e2e -v ./test/e2e/

# Tear down
docker compose down -v
```

## Running against an existing instance

```bash
FEATHER_E2E_URL=http://your-feather-host:8080 go test -tags e2e -v ./test/e2e/
```

## Test Coverage

| Test | Description |
|------|-------------|
| `TestHealthEndpoint` | Verifies `/health` returns 200 |
| `TestStoreAndRetrieveFeature` | Stores and retrieves a feature value |
| `TestBatchFeatures` | Tests batch feature retrieval |
| `TestSchemaGroups` | Lists feature group schemas |
| `TestDriftStatus` | Checks drift monitoring status |
| `TestFeatherQLQuery` | Executes a FeatherQL query |
| `TestOpenAPISpec` | Validates OpenAPI spec availability |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FEATHER_E2E_URL` | Base URL of the Feather instance | *(required)* |
