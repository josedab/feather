//! # Feather Feature Store Rust Client
//!
//! A Rust client for [Feather Feature Store](https://github.com/feather-store/feather).
//!
//! ## Quick Start
//!
//! ```rust,no_run
//! use feather_client::{FeatherClient, ClientConfig};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let client = FeatherClient::new(ClientConfig {
//!         base_url: "http://localhost:8080".to_string(),
//!         ..Default::default()
//!     })?;
//!
//!     // Get features
//!     let features = client.get_features("user:123", Some(&["age", "country"])).await?;
//!     println!("{:?}", features);
//!
//!     // Store features
//!     let mut features_map = std::collections::HashMap::new();
//!     features_map.insert("age".to_string(), serde_json::json!(25));
//!     features_map.insert("country".to_string(), serde_json::json!("US"));
//!     client.put_features("user:123", features_map, None).await?;
//!
//!     Ok(())
//! }
//! ```

mod client;
mod error;
mod models;
#[cfg(test)]
mod tests;
mod vectors;

pub use client::{ClientConfig, FeatherClient};
pub use error::{FeatherError, Result};
pub use models::*;
pub use vectors::VectorClient;
