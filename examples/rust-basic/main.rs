// Feather Feature Store - Rust Basic Example
//
// Demonstrates core feature store operations using the Feather REST API.
// Start a Feather server first: make run-dev

use std::collections::HashMap;

const FEATHER_URL: &str = "http://localhost:8080";

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("🪶 Feather Rust Example");
    println!("Connecting to {}\n", FEATHER_URL);

    // 1. Health check
    let health_url = format!("{}/health", FEATHER_URL);
    let health_resp = ureq::get(&health_url).call()?;
    let health: serde_json::Value = health_resp.into_json()?;
    println!("Health: {}", health["status"]);

    // 2. Store features
    let store_url = format!("{}/v1/features", FEATHER_URL);
    let mut features = HashMap::new();
    features.insert("age", serde_json::json!({"value": 32, "timestamp": 0}));
    features.insert("score", serde_json::json!({"value": 0.87, "timestamp": 0}));

    let body = serde_json::json!({
        "entity": "user:2001",
        "features": features,
    });

    let store_resp = ureq::post(&store_url)
        .set("Content-Type", "application/json")
        .send_json(body)?;
    println!("Store: {}", if store_resp.status() == 200 { "✅" } else { "❌" });

    // 3. Retrieve features
    let get_url = format!("{}/v1/features?entity=user:2001&feature=age&feature=score", FEATHER_URL);
    let get_resp = ureq::get(&get_url).call()?;
    let data: serde_json::Value = get_resp.into_json()?;
    println!("Retrieved: {}", serde_json::to_string_pretty(&data)?);

    println!("\n✅ Done!");
    Ok(())
}
