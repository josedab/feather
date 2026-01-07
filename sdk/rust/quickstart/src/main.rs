//! Feather Rust Quickstart - Get started in 30 seconds!

use feather_client::{FeatherClient, PutFeaturesRequest};
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 1. Connect to Feather
    let client = FeatherClient::new("http://localhost:8080")?;

    // 2. Store features for an entity
    client
        .put_features(PutFeaturesRequest {
            entity_id: "user:123".to_string(),
            features: serde_json::from_value(json!({
                "score": 0.95,
                "purchases": 42,
                "premium": true
            }))?,
            timestamp: None,
        })
        .await?;
    println!("Stored features for user:123");

    // 3. Retrieve features
    let response = client
        .get_features("user:123", Some(vec!["score", "purchases"]))
        .await?;
    println!("Retrieved features for {}:", response.entity_id);
    for (name, fv) in &response.features {
        println!("  {}: {:?} (updated: {})", name, fv.value, fv.timestamp);
    }

    // 4. Batch retrieval (multiple entities)
    let results = client
        .get_features_batch(vec!["user:123", "user:456"], Some(vec!["score"]))
        .await?;
    println!("\nBatch retrieved {} entities", results.len());

    println!("\nQuickstart complete!");
    Ok(())
}
