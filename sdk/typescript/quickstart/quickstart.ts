/**
 * Feather TypeScript Quickstart - Get started in 30 seconds!
 */

import { FeatherClient } from '@feather-store/client';

async function main() {
  // 1. Connect to Feather
  const client = new FeatherClient({
    baseUrl: 'http://localhost:8080',
  });

  // 2. Store features for an entity
  await client.putFeatures({
    entityId: 'user:123',
    features: {
      score: 0.95,
      purchases: 42,
      premium: true,
    },
  });
  console.log('Stored features for user:123');

  // 3. Retrieve features
  const response = await client.getFeatures('user:123', ['score', 'purchases']);
  console.log(`Retrieved features for ${response.entityId}:`);
  for (const [name, fv] of Object.entries(response.features)) {
    console.log(`  ${name}: ${fv.value} (updated: ${fv.timestamp})`);
  }

  // 4. Batch retrieval (multiple entities)
  const results = await client.getFeaturesBatch({
    entityIds: ['user:123', 'user:456'],
    features: ['score'],
  });
  console.log(`\nBatch retrieved ${Object.keys(results).length} entities`);

  console.log('\nQuickstart complete!');
}

main().catch(console.error);
