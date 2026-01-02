//! Vector operations client.

use crate::client::FeatherClient;
use crate::error::{FeatherError, Result};
use crate::models::*;
use serde::Serialize;
use std::collections::HashMap;

/// Client for vector similarity search operations.
pub struct VectorClient<'a> {
    client: &'a FeatherClient,
}

impl<'a> VectorClient<'a> {
    pub(crate) fn new(client: &'a FeatherClient) -> Self {
        Self { client }
    }

    /// List all vector indexes.
    pub async fn list_indexes(&self) -> Result<Vec<String>> {
        let response: ListIndexesResponse = self
            .client
            .get(&format!("{}/v1/vectors", self.client.base_url()))
            .await?;
        Ok(response.indexes)
    }

    /// Create a new vector index.
    pub async fn create_index(
        &self,
        name: &str,
        dimension: i32,
        distance_type: Option<DistanceType>,
    ) -> Result<VectorIndex> {
        let request = CreateIndexRequest {
            name: name.to_string(),
            dimension,
            distance_type,
        };
        self.client
            .post(&format!("{}/v1/vectors", self.client.base_url()), &request)
            .await
    }

    /// Get information about a vector index.
    pub async fn get_index(&self, name: &str) -> Result<VectorIndex> {
        self.client
            .get(&format!("{}/v1/vectors/{}", self.client.base_url(), name))
            .await
    }

    /// Delete a vector index.
    pub async fn delete_index(&self, name: &str) -> Result<()> {
        self.client
            .delete(&format!("{}/v1/vectors/{}", self.client.base_url(), name))
            .await
    }

    /// Upsert vectors into an index.
    pub async fn upsert(&self, index: &str, vectors: Vec<VectorRecord>) -> Result<i32> {
        #[derive(Serialize)]
        struct Request {
            vectors: Vec<VectorRecord>,
        }

        let request = Request { vectors };
        let response: UpsertResponse = self
            .client
            .post(
                &format!("{}/v1/vectors/{}/upsert", self.client.base_url(), index),
                &request,
            )
            .await?;
        Ok(response.upserted_count)
    }

    /// Search for similar vectors.
    pub async fn search(
        &self,
        index: &str,
        vector: Vec<f64>,
        top_k: Option<i32>,
        filter: Option<HashMap<String, FeatureValue>>,
        include_metadata: Option<bool>,
        include_vector: Option<bool>,
    ) -> Result<Vec<VectorSearchResult>> {
        let request = SearchVectorsRequest {
            vector,
            top_k,
            filter,
            include_metadata,
            include_vector,
        };

        let response: VectorSearchResponse = self
            .client
            .post(
                &format!("{}/v1/vectors/{}/search", self.client.base_url(), index),
                &request,
            )
            .await?;
        Ok(response.results)
    }

    /// Get a vector by ID.
    pub async fn get(&self, index: &str, id: &str) -> Result<Option<VectorRecord>> {
        match self
            .client
            .get::<VectorRecord>(&format!(
                "{}/v1/vectors/{}/{}",
                self.client.base_url(),
                index,
                id
            ))
            .await
        {
            Ok(record) => Ok(Some(record)),
            Err(FeatherError::NotFound(_)) => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Delete a vector by ID.
    pub async fn delete(&self, index: &str, id: &str) -> Result<()> {
        self.client
            .delete(&format!(
                "{}/v1/vectors/{}/{}",
                self.client.base_url(),
                index,
                id
            ))
            .await
    }
}
