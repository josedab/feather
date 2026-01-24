# Managing LLM Features with Feather

Version your prompts, cache LLM responses to cut costs, and store embeddings for similarity search — all from a single feature store.

**Time:** ~15 minutes

## What You'll Learn

- How to create and version prompt templates
- How to render prompts with variable substitution
- How to track prompt quality scores and token usage
- How to cache LLM responses for cost savings
- How to store and search embeddings

## Prerequisites

- Feather running locally (`make run-dev`) — see [Getting Started](01-getting-started.md)
- curl and jq installed

---

## Scenario

You are building a customer support chatbot. You need to:
- Manage prompt versions as you iterate on quality
- Cache common responses to reduce API costs
- Store and search document embeddings for retrieval-augmented generation (RAG)

---

## Step 1: Create a Prompt Template

Create a versioned prompt for your customer support bot:

```bash
$ curl -s -X POST http://localhost:8080/v1/prompts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "customer_support_reply",
    "template": "You are a helpful support agent for {{company_name}}.\n\nCustomer issue: {{issue_description}}\nCustomer sentiment: {{sentiment}}\nAccount tier: {{account_tier}}\n\nProvide a empathetic, solution-oriented response in under 150 words.",
    "model": "gpt-4",
    "tags": ["support", "customer-facing"],
    "metadata": {
      "owner": "support-team",
      "use_case": "ticket_auto_reply"
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "prompt": {
    "id": "customer_support_reply",
    "version": 1,
    "template": "You are a helpful support agent for {{company_name}}.\n\nCustomer issue: {{issue_description}}\nCustomer sentiment: {{sentiment}}\nAccount tier: {{account_tier}}\n\nProvide a empathetic, solution-oriented response in under 150 words.",
    "model": "gpt-4",
    "tags": ["support", "customer-facing"],
    "created_at": "2025-01-15T10:00:00Z"
  }
}
```

---

## Step 2: Version the Prompt

After testing, you decide to improve the prompt. Update it to create version 2:

```bash
$ curl -s -X PUT http://localhost:8080/v1/prompts/customer_support_reply \
  -H "Content-Type: application/json" \
  -d '{
    "template": "You are a senior support specialist at {{company_name}}.\n\nCustomer issue: {{issue_description}}\nSentiment: {{sentiment}}\nAccount tier: {{account_tier}}\nPrevious interactions: {{interaction_count}}\n\nInstructions:\n1. Acknowledge the customer'\''s frustration if sentiment is negative\n2. Provide a clear, actionable solution\n3. If the account tier is \"enterprise\", offer to escalate to a dedicated manager\n4. Keep the response under 200 words\n\nRespond in a warm, professional tone.",
    "model": "gpt-4",
    "tags": ["support", "customer-facing", "v2"]
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "prompt": {
    "id": "customer_support_reply",
    "version": 2,
    "template": "You are a senior support specialist at {{company_name}}...",
    "model": "gpt-4",
    "tags": ["support", "customer-facing", "v2"],
    "created_at": "2025-01-15T10:05:00Z"
  }
}
```

List all versions of the prompt:

```bash
$ curl -s http://localhost:8080/v1/prompts/customer_support_reply/versions | jq .
```

```json
{
  "prompt_id": "customer_support_reply",
  "versions": [
    {
      "version": 1,
      "model": "gpt-4",
      "created_at": "2025-01-15T10:00:00Z"
    },
    {
      "version": 2,
      "model": "gpt-4",
      "created_at": "2025-01-15T10:05:00Z"
    }
  ]
}
```

---

## Step 3: Render the Prompt

Render the prompt with actual values for variable substitution:

```bash
$ curl -s -X POST http://localhost:8080/v1/prompts/customer_support_reply/render \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {
      "company_name": "Acme Corp",
      "issue_description": "My order #12345 has not arrived after 10 days",
      "sentiment": "frustrated",
      "account_tier": "enterprise",
      "interaction_count": "3"
    }
  }' | jq .
```

Expected output:

```json
{
  "rendered": "You are a senior support specialist at Acme Corp.\n\nCustomer issue: My order #12345 has not arrived after 10 days\nSentiment: frustrated\nAccount tier: enterprise\nPrevious interactions: 3\n\nInstructions:\n1. Acknowledge the customer's frustration if sentiment is negative\n2. Provide a clear, actionable solution\n3. If the account tier is \"enterprise\", offer to escalate to a dedicated manager\n4. Keep the response under 200 words\n\nRespond in a warm, professional tone.",
  "version": 2,
  "model": "gpt-4",
  "variables_used": ["company_name", "issue_description", "sentiment", "account_tier", "interaction_count"]
}
```

---

## Step 4: Track Token Usage and Quality

After calling your LLM with the rendered prompt, record the usage metrics:

```bash
$ curl -s -X POST http://localhost:8080/v1/prompts/customer_support_reply/score \
  -H "Content-Type: application/json" \
  -d '{
    "version": 2,
    "score": 4.5,
    "tokens_in": 185,
    "tokens_out": 142,
    "latency_ms": 1250,
    "metadata": {
      "ticket_id": "TICKET-5678",
      "agent_approved": true
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "prompt_id": "customer_support_reply",
  "version": 2,
  "score_recorded": 4.5
}
```

Check aggregate usage stats:

```bash
$ curl -s http://localhost:8080/v1/prompts/customer_support_reply/usage | jq .
```

```json
{
  "prompt_id": "customer_support_reply",
  "versions": {
    "2": {
      "invocations": 1,
      "avg_score": 4.5,
      "avg_tokens_in": 185,
      "avg_tokens_out": 142,
      "avg_latency_ms": 1250,
      "total_tokens": 327
    }
  }
}
```

---

## Step 5: Cache LLM Responses

Use the semantic LLM cache to avoid redundant API calls. When similar prompts are sent, Feather can return cached responses:

**Store a response in the cache:**

```bash
$ curl -s -X POST http://localhost:8080/v1/llm/cache/store \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "You are a senior support specialist at Acme Corp.\n\nCustomer issue: My order has not arrived after 10 days\nSentiment: frustrated\nAccount tier: enterprise",
    "response": "I completely understand your frustration, and I sincerely apologize for the delay with your order. Let me look into this right away.\n\nI can see your order is currently in transit and appears to have been delayed at our distribution center. Here is what I will do:\n1. Escalate this to our priority fulfillment team immediately\n2. Arrange express shipping at no additional cost\n3. Apply a 15% discount to your next order as a gesture of goodwill\n\nAs an enterprise customer, I am also assigning you a dedicated account manager who will follow up within 24 hours. You can reach them directly at enterprise-support@acme.com.",
    "model": "gpt-4",
    "provider": "openai",
    "tokens_in": 85,
    "tokens_out": 142,
    "cost_usd": 0.0068,
    "ttl": "24h"
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "cache_key": "a1b2c3d4e5f6",
  "ttl": "24h0m0s"
}
```

**Look up a cached response:**

```bash
$ curl -s -X POST http://localhost:8080/v1/llm/cache/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "You are a senior support specialist at Acme Corp.\n\nCustomer issue: My order has not arrived after 10 days\nSentiment: frustrated\nAccount tier: enterprise",
    "model": "gpt-4",
    "similarity_threshold": 0.92
  }' | jq .
```

Expected output:

```json
{
  "hit": true,
  "match_type": "exact",
  "similarity": 1.0,
  "response": "I completely understand your frustration...",
  "cached_at": "2025-01-15T10:15:00Z",
  "cost_saved_usd": 0.0068
}
```

**Check cache statistics:**

```bash
$ curl -s http://localhost:8080/v1/llm/cache/stats | jq .
```

```json
{
  "total_lookups": 1,
  "cache_hits": 1,
  "cache_misses": 0,
  "hit_rate": 1.0,
  "entries_stored": 1,
  "avg_similarity": 1.0
}
```

**Check cost savings by provider:**

```bash
$ curl -s http://localhost:8080/v1/llm/cache/costs | jq .
```

```json
{
  "providers": {
    "openai": {
      "total_cost_usd": 0.0068,
      "savings_usd": 0.0068,
      "cached_requests": 1,
      "tokens_saved": 227
    }
  },
  "total_savings_usd": 0.0068
}
```

---

## Step 6: Store and Search Embeddings

Create an embedding collection for RAG (retrieval-augmented generation):

```bash
$ curl -s -X POST http://localhost:8080/v1/embeddings/collections \
  -H "Content-Type: application/json" \
  -d '{
    "name": "support_docs",
    "dimensions": 128,
    "distance_metric": "cosine",
    "metadata": {
      "source": "help_center",
      "model": "text-embedding-3-small"
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "collection": {
    "name": "support_docs",
    "dimensions": 128,
    "distance_metric": "cosine",
    "count": 0
  }
}
```

**Upsert document embeddings:**

```bash
$ curl -s -X POST http://localhost:8080/v1/embeddings/collections/support_docs/upsert \
  -H "Content-Type: application/json" \
  -d '{
    "embeddings": [
      {
        "id": "doc-shipping-policy",
        "vector": [0.12, -0.34, 0.56, 0.78, -0.91, 0.23, 0.45, -0.67, 0.89, -0.12, 0.34, -0.56, 0.78, 0.91, -0.23, 0.45, 0.67, -0.89, 0.12, 0.34, -0.56, 0.78, -0.91, 0.23, -0.45, 0.67, 0.89, -0.12, 0.34, 0.56, -0.78, 0.91, 0.12, -0.34, 0.56, 0.78, -0.91, 0.23, 0.45, -0.67, 0.89, -0.12, 0.34, -0.56, 0.78, 0.91, -0.23, 0.45, 0.67, -0.89, 0.12, 0.34, -0.56, 0.78, -0.91, 0.23, -0.45, 0.67, 0.89, -0.12, 0.34, 0.56, -0.78, 0.91, 0.12, -0.34, 0.56, 0.78, -0.91, 0.23, 0.45, -0.67, 0.89, -0.12, 0.34, -0.56, 0.78, 0.91, -0.23, 0.45, 0.67, -0.89, 0.12, 0.34, -0.56, 0.78, -0.91, 0.23, -0.45, 0.67, 0.89, -0.12, 0.34, 0.56, -0.78, 0.91, 0.12, -0.34, 0.56, 0.78, -0.91, 0.23, 0.45, -0.67, 0.89, -0.12, 0.34, -0.56, 0.78, 0.91, -0.23, 0.45, 0.67, -0.89, 0.12, 0.34, -0.56, 0.78, -0.91, 0.23, -0.45, 0.67, 0.89, -0.12, 0.34, 0.56, -0.78, 0.91],
        "metadata": {
          "title": "Shipping Policy",
          "content": "Orders are delivered within 5-7 business days..."
        }
      },
      {
        "id": "doc-return-policy",
        "vector": [0.22, -0.44, 0.66, 0.88, -0.81, 0.13, 0.55, -0.77, 0.99, -0.22, 0.44, -0.66, 0.88, 0.81, -0.13, 0.55, 0.77, -0.99, 0.22, 0.44, -0.66, 0.88, -0.81, 0.13, -0.55, 0.77, 0.99, -0.22, 0.44, 0.66, -0.88, 0.81, 0.22, -0.44, 0.66, 0.88, -0.81, 0.13, 0.55, -0.77, 0.99, -0.22, 0.44, -0.66, 0.88, 0.81, -0.13, 0.55, 0.77, -0.99, 0.22, 0.44, -0.66, 0.88, -0.81, 0.13, -0.55, 0.77, 0.99, -0.22, 0.44, 0.66, -0.88, 0.81, 0.22, -0.44, 0.66, 0.88, -0.81, 0.13, 0.55, -0.77, 0.99, -0.22, 0.44, -0.66, 0.88, 0.81, -0.13, 0.55, 0.77, -0.99, 0.22, 0.44, -0.66, 0.88, -0.81, 0.13, -0.55, 0.77, 0.99, -0.22, 0.44, 0.66, -0.88, 0.81, 0.22, -0.44, 0.66, 0.88, -0.81, 0.13, 0.55, -0.77, 0.99, -0.22, 0.44, -0.66, 0.88, 0.81, -0.13, 0.55, 0.77, -0.99, 0.22, 0.44, -0.66, 0.88, -0.81, 0.13, -0.55, 0.77, 0.99, -0.22, 0.44, 0.66, -0.88, 0.81],
        "metadata": {
          "title": "Return Policy",
          "content": "Items can be returned within 30 days..."
        }
      }
    ]
  }' | jq .
```

Expected output:

```json
{
  "status": "ok",
  "upserted": 2
}
```

**Search for similar documents:**

```bash
$ curl -s -X POST http://localhost:8080/v1/embeddings/collections/support_docs/search \
  -H "Content-Type: application/json" \
  -d '{
    "vector": [0.15, -0.36, 0.58, 0.80, -0.89, 0.21, 0.47, -0.69, 0.91, -0.14, 0.36, -0.58, 0.80, 0.89, -0.21, 0.47, 0.69, -0.91, 0.14, 0.36, -0.58, 0.80, -0.89, 0.21, -0.47, 0.69, 0.91, -0.14, 0.36, 0.58, -0.80, 0.89, 0.14, -0.36, 0.58, 0.80, -0.89, 0.21, 0.47, -0.69, 0.91, -0.14, 0.36, -0.58, 0.80, 0.89, -0.21, 0.47, 0.69, -0.91, 0.14, 0.36, -0.58, 0.80, -0.89, 0.21, -0.47, 0.69, 0.91, -0.14, 0.36, 0.58, -0.80, 0.89, 0.14, -0.36, 0.58, 0.80, -0.89, 0.21, 0.47, -0.69, 0.91, -0.14, 0.36, -0.58, 0.80, 0.89, -0.21, 0.47, 0.69, -0.91, 0.14, 0.36, -0.58, 0.80, -0.89, 0.21, -0.47, 0.69, 0.91, -0.14, 0.36, 0.58, -0.80, 0.89, 0.14, -0.36, 0.58, 0.80, -0.89, 0.21, 0.47, -0.69, 0.91, -0.14, 0.36, -0.58, 0.80, 0.89, -0.21, 0.47, 0.69, -0.91, 0.14, 0.36, -0.58, 0.80, -0.89, 0.21, -0.47, 0.69, 0.91, -0.14, 0.36, 0.58, -0.80, 0.89],
    "top_k": 2
  }' | jq .
```

Expected output:

```json
{
  "results": [
    {
      "id": "doc-shipping-policy",
      "score": 0.98,
      "metadata": {
        "title": "Shipping Policy",
        "content": "Orders are delivered within 5-7 business days..."
      }
    },
    {
      "id": "doc-return-policy",
      "score": 0.91,
      "metadata": {
        "title": "Return Policy",
        "content": "Items can be returned within 30 days..."
      }
    }
  ]
}
```

---

## End-to-End RAG Flow

Here's how the pieces connect in a production RAG pipeline:

```
User Question
      │
      ▼
┌─────────────┐    ┌────────────────────────────┐
│ Embedding   │───▶│ Feather Embedding Search    │
│ Model       │    │ POST /v1/embeddings/.../search │
└─────────────┘    └────────────────────────────┘
                           │
                     Top-K documents
                           │
                           ▼
                   ┌──────────────┐
                   │ Prompt Store │ ── render template with context
                   │ GET /v1/prompts/…/render │
                   └──────────────┘
                           │
                           ▼
                   ┌──────────────┐
                   │ LLM Cache    │ ── check for cached response
                   │ POST /v1/llm/cache/lookup │
                   └──────────────┘
                        │       │
                     HIT      MISS
                      │         │
                      │    ┌────▼─────┐
                      │    │ Call LLM  │
                      │    │ (OpenAI)  │
                      │    └────┬─────┘
                      │         │
                      │    Store in cache
                      │         │
                      ▼         ▼
                   Return response to user
```

---

## What's Next?

- **[Deploying on Kubernetes](05-kubernetes-deployment.md)** — Run this LLM pipeline in production
- **[Real-Time Fraud Detection](02-fraud-detection.md)** — See Feather's real-time features in action
