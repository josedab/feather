// Feather Feature Store - TypeScript Basic Example
//
// Demonstrates core feature store operations using the Feather REST API.
// Start a Feather server first: make run-dev

const FEATHER_URL = process.env.FEATHER_URL || "http://localhost:8080";

interface FeatureValue {
  value: number | string | boolean;
  timestamp: number;
}

interface FeatureResponse {
  entity: string;
  features: Record<string, FeatureValue>;
}

async function main() {
  console.log("🪶 Feather TypeScript Example");
  console.log(`Connecting to ${FEATHER_URL}\n`);

  // 1. Health check
  const health = await fetch(`${FEATHER_URL}/health`);
  const healthData = await health.json();
  console.log("Health:", healthData.status);

  // 2. Store features
  const storeResp = await fetch(`${FEATHER_URL}/v1/features`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      entity: "user:1001",
      features: {
        age: { value: 28, timestamp: Date.now() * 1e6 },
        score: { value: 0.95, timestamp: Date.now() * 1e6 },
        active: { value: true, timestamp: Date.now() * 1e6 },
      },
    }),
  });
  console.log("Store:", storeResp.status === 200 ? "✅" : "❌");

  // 3. Retrieve features
  const getResp = await fetch(
    `${FEATHER_URL}/v1/features?entity=user:1001&feature=age&feature=score`
  );
  if (getResp.ok) {
    const data: FeatureResponse = await getResp.json();
    console.log("Retrieved features:", JSON.stringify(data, null, 2));
  }

  // 4. Batch retrieval
  const batchResp = await fetch(`${FEATHER_URL}/v1/features/batch`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      entities: ["user:1001", "user:1002"],
      features: ["age", "score"],
    }),
  });
  if (batchResp.ok) {
    const batchData = await batchResp.json();
    console.log("Batch results:", JSON.stringify(batchData, null, 2));
  }

  console.log("\n✅ Done!");
}

main().catch(console.error);
