# TypeScript Basic Example

A minimal TypeScript example demonstrating Feather feature store operations.

## Prerequisites

- Node.js 18+
- A running Feather server (`make run-dev`)

## Setup

```bash
npm install
```

## Run

```bash
npx ts-node index.ts
```

## What it does

1. Creates a feature group schema
2. Stores feature values for entities
3. Retrieves features by entity ID
4. Performs batch feature retrieval
