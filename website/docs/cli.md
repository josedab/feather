---
sidebar_position: 5
title: CLI Reference
description: Complete reference for the feather-cli command-line tool.
---

# CLI Reference

`feather-cli` is a command-line interface for interacting with Feather feature stores. It provides commands for managing features, schemas, vectors, and data ingestion.

## Installation

### Download Binary

```bash
# Linux (amd64)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-cli-linux-amd64 -o feather-cli
chmod +x feather-cli

# macOS (Apple Silicon)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-cli-darwin-arm64 -o feather-cli
chmod +x feather-cli

# macOS (Intel)
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-cli-darwin-amd64 -o feather-cli
chmod +x feather-cli
```

### Build from Source

```bash
git clone https://github.com/feather-store/feather.git
cd feather
make build-cli
./bin/feather-cli --help
```

## Global Flags

These flags apply to all commands:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--server` | `-s` | `http://localhost:8080` | Feather server URL |
| `--output` | `-o` | `table` | Output format: `table`, `json`, `yaml` |
| `--config` | | `~/.feather-cli.yaml` | Path to config file |
| `--api-key` | | | API key for authentication |
| `--verbose` | `-v` | `false` | Enable verbose output |

## Configuration

Create `~/.feather-cli.yaml` to set defaults:

```yaml
server: http://feather.internal:8080
output: json
api_key: your-api-key
timeout: 30s
```

Environment variables override config file settings:

```bash
export FEATHER_SERVER=http://localhost:8080
export FEATHER_API_KEY=your-api-key
```

---

## Features Commands

### features get

Retrieve features for an entity.

```bash
feather-cli features get <entity> [flags]
```

**Flags:**
- `--feature`, `-f` (repeatable): Feature names to retrieve

**Examples:**

```bash
# Get all features for an entity
feather-cli features get user:123

# Get specific features
feather-cli features get user:123 -f clicks -f purchases

# Output as JSON
feather-cli features get user:123 -o json
```

### features put

Store features for an entity.

```bash
feather-cli features put <entity> <feature>=<value>... [flags]
```

**Examples:**

```bash
# Store a single feature
feather-cli features put user:123 clicks=42

# Store multiple features
feather-cli features put user:123 clicks=42 purchases=5 score=0.95

# Store with verbose output
feather-cli features put user:123 active=true -v
```

### features batch

Batch retrieve features for multiple entities.

```bash
feather-cli features batch [flags]
```

**Flags:**
- `--entity`, `-e` (repeatable): Entity keys
- `--feature`, `-f` (repeatable): Feature names

**Examples:**

```bash
# Get features for multiple entities
feather-cli features batch -e user:123 -e user:456 -f clicks -f purchases
```

### features history

Retrieve point-in-time feature values.

```bash
feather-cli features history <entity> [flags]
```

**Flags:**
- `--as-of`: Timestamp (RFC3339 format)
- `--feature`, `-f` (repeatable): Feature names

**Examples:**

```bash
# Get features as of a specific time
feather-cli features history user:123 --as-of 2024-01-15T10:30:00Z

# Get specific features at a point in time
feather-cli features history user:123 --as-of 2024-01-15T10:30:00Z -f clicks
```

---

## Schema Commands

### schema list

List all feature groups.

```bash
feather-cli schema list [flags]
```

**Examples:**

```bash
# List all schemas
feather-cli schema list

# Output as YAML
feather-cli schema list -o yaml
```

### schema get

Get details of a feature group.

```bash
feather-cli schema get <group-name> [flags]
```

**Examples:**

```bash
# Get schema details
feather-cli schema get user_features

# Output as JSON
feather-cli schema get user_features -o json
```

### schema create

Create a new feature group from a file.

```bash
feather-cli schema create <file> [flags]
```

**Examples:**

```bash
# Create from YAML file
feather-cli schema create user_features.yaml

# Create from JSON file
feather-cli schema create user_features.json
```

**Schema file format (YAML):**

```yaml
name: user_features
entity_type: user
ttl: 24h
description: User behavioral features
features:
  - name: clicks
    data_type: int64
  - name: purchases
    data_type: int64
  - name: score
    data_type: float64
```

---

## Vectors Commands

### vectors index list

List all vector indexes.

```bash
feather-cli vectors index list [flags]
```

### vectors index create

Create a new vector index.

```bash
feather-cli vectors index create <name> [flags]
```

**Flags:**
- `--dimensions`, `-d` (required): Number of dimensions
- `--metric`, `-m`: Distance metric (`cosine`, `euclidean`, `dot`). Default: `cosine`

**Examples:**

```bash
# Create an index for 384-dimensional embeddings
feather-cli vectors index create embeddings -d 384

# Create with Euclidean distance
feather-cli vectors index create my-index -d 768 -m euclidean
```

### vectors index delete

Delete a vector index.

```bash
feather-cli vectors index delete <name> [flags]
```

### vectors search

Search for similar vectors.

```bash
feather-cli vectors search <index> [flags]
```

**Flags:**
- `--vector`, `-V` (required): Query vector as comma-separated values
- `--top-k`, `-k`: Number of results. Default: `10`

**Examples:**

```bash
# Search for similar vectors
feather-cli vectors search embeddings --vector "0.1,0.2,0.3,0.4" --top-k 5

# Output as JSON
feather-cli vectors search embeddings -V "0.1,0.2,0.3" -k 10 -o json
```

### vectors upsert

Insert or update a vector.

```bash
feather-cli vectors upsert <index> <id> <vector> [flags]
```

**Examples:**

```bash
# Upsert a single vector
feather-cli vectors upsert embeddings doc-1 "0.1,0.2,0.3,0.4"
```

---

## Ingest Commands

### ingest file

Ingest features from a file.

```bash
feather-cli ingest file <path> [flags]
```

**Flags:**
- `--format`: File format (`json`, `csv`). Auto-detected from extension.
- `--entity-column`: Column name for entity key (CSV). Default: `entity`
- `--batch-size`: Batch size for ingestion. Default: `1000`

**Examples:**

```bash
# Ingest from JSON file
feather-cli ingest file features.json

# Ingest from CSV with custom entity column
feather-cli ingest file features.csv --entity-column user_id

# Ingest with custom batch size
feather-cli ingest file features.json --batch-size 500
```

**JSON file format:**

```json
[
  {"entity_key": "user:123", "features": {"clicks": 42, "score": 0.95}},
  {"entity_key": "user:456", "features": {"clicks": 10, "score": 0.85}}
]
```

**CSV file format:**

```csv
entity,clicks,score
user:123,42,0.95
user:456,10,0.85
```

### ingest stream

Stream features from stdin.

```bash
feather-cli ingest stream [flags]
```

**Flags:**
- `--format`: Input format (`json`, `jsonl`). Default: `jsonl`

**Examples:**

```bash
# Stream JSONL from stdin
cat features.jsonl | feather-cli ingest stream

# Stream from another command
kafka-console-consumer --topic features | feather-cli ingest stream
```

---

## Health Commands

### health

Check server health.

```bash
feather-cli health [flags]
```

**Flags:**
- `--deep`: Perform deep health check (checks all components)

**Examples:**

```bash
# Quick health check
feather-cli health

# Deep health check
feather-cli health --deep

# Check remote server
feather-cli health -s http://feather.prod:8080
```

**Output:**

```
Server: http://localhost:8080
Status: healthy
Components:
  hot_tier: ok
  warm_tier: ok
  aggregation: ok
```

---

## Version Commands

### version

Show version information.

```bash
feather-cli version [flags]
```

**Examples:**

```bash
feather-cli version
```

**Output:**

```json
{
  "cli": {
    "version": "1.0.0",
    "git_commit": "abc1234",
    "build_date": "2024-01-15T10:30:00Z",
    "go_version": "go1.24",
    "platform": "darwin/arm64"
  },
  "server": {
    "version": "1.0.0",
    "status": "connected"
  }
}
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Connection error |
| 3 | Authentication error |
| 4 | Not found |
| 5 | Validation error |

---

## Examples

### Common Workflows

**Feature development cycle:**

```bash
# Create a schema
feather-cli schema create user_features.yaml

# Store test features
feather-cli features put user:test clicks=10 purchases=2

# Verify retrieval
feather-cli features get user:test

# Check with point-in-time
feather-cli features history user:test --as-of $(date -u +%Y-%m-%dT%H:%M:%SZ)
```

**Vector search workflow:**

```bash
# Create index
feather-cli vectors index create products -d 384 -m cosine

# Upsert vectors
feather-cli vectors upsert products prod-1 "0.1,0.2,..."
feather-cli vectors upsert products prod-2 "0.3,0.4,..."

# Search
feather-cli vectors search products -V "0.15,0.25,..." -k 5
```

**Bulk data loading:**

```bash
# Export from your data warehouse
bq query --format=json 'SELECT * FROM features' > features.json

# Load into Feather
feather-cli ingest file features.json --batch-size 5000

# Verify
feather-cli features get user:123
```
