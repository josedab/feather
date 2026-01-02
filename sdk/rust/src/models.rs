//! Data models for the Feather client.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Feature value type alias.
pub type FeatureValue = serde_json::Value;

/// Data types for features.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum DataType {
    String,
    Int64,
    Float64,
    Bool,
    Bytes,
    Timestamp,
    StringList,
    Int64List,
    Float64List,
    Map,
}

/// Feature specification in a feature group.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeatureSpec {
    pub name: String,
    pub data_type: DataType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_value: Option<FeatureValue>,
}

/// Feature group definition.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeatureGroup {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub entity_type: String,
    pub features: Vec<FeatureSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ttl: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tags: Option<HashMap<String, String>>,
}

/// Response from getting features.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GetFeaturesResponse {
    pub entity_id: String,
    pub features: HashMap<String, FeatureValue>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, FeatureValue>>,
}

/// Features for an entity in batch operations.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EntityFeatures {
    pub entity_id: String,
    pub features: HashMap<String, FeatureValue>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timestamp: Option<DateTime<Utc>>,
}

/// Response from batch get.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchGetResponse {
    pub results: Vec<EntityFeatures>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub errors: Option<HashMap<String, String>>,
}

/// Vector index information.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VectorIndex {
    pub name: String,
    pub dimension: i32,
    pub distance_type: DistanceType,
    pub vector_count: i64,
    pub created_at: DateTime<Utc>,
}

/// Distance metric types for vector search.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum DistanceType {
    Cosine,
    Euclidean,
    DotProduct,
}

impl Default for DistanceType {
    fn default() -> Self {
        DistanceType::Cosine
    }
}

/// A vector record for storage.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VectorRecord {
    pub id: String,
    pub vector: Vec<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, FeatureValue>>,
}

/// Result from vector search.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VectorSearchResult {
    pub id: String,
    pub score: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vector: Option<Vec<f64>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, FeatureValue>>,
}

/// Response from vector search.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VectorSearchResponse {
    pub results: Vec<VectorSearchResult>,
    pub took: i64,
}

/// Response from upsert operation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UpsertResponse {
    pub upserted_count: i32,
}

/// Health status of the server.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthStatus {
    pub status: String,
    pub components: HashMap<String, ComponentHealth>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub uptime: Option<i64>,
}

/// Health of a component.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComponentHealth {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub latency: Option<i64>,
}

/// Aggregation function types.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum AggFunction {
    Count,
    Sum,
    Avg,
    Min,
    Max,
    Last,
}

/// Response from aggregation query.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AggregationResponse {
    pub entity_id: String,
    pub feature: String,
    pub function: AggFunction,
    pub value: f64,
    pub window_start: DateTime<Utc>,
    pub window_end: DateTime<Utc>,
}

/// Request to create a vector index.
#[derive(Debug, Clone, Serialize)]
pub struct CreateIndexRequest {
    pub name: String,
    pub dimension: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub distance_type: Option<DistanceType>,
}

/// Request for vector search.
#[derive(Debug, Clone, Serialize)]
pub struct SearchVectorsRequest {
    pub vector: Vec<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_k: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub filter: Option<HashMap<String, FeatureValue>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include_metadata: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub include_vector: Option<bool>,
}

/// List indexes response.
#[derive(Debug, Clone, Deserialize)]
pub struct ListIndexesResponse {
    pub indexes: Vec<String>,
}

/// List feature groups response.
#[derive(Debug, Clone, Deserialize)]
pub struct ListFeatureGroupsResponse {
    pub groups: Vec<FeatureGroup>,
}
