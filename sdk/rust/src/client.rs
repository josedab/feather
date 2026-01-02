//! Feather client implementation.

use crate::error::{FeatherError, Result};
use crate::models::*;
use crate::vectors::VectorClient;
use chrono::{DateTime, Utc};
use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE};
use reqwest::Client;
use serde::de::DeserializeOwned;
use serde::Serialize;
use std::collections::HashMap;
use std::time::Duration;

/// Configuration for the Feather client.
#[derive(Debug, Clone)]
pub struct ClientConfig {
    /// Base URL of the Feather server.
    pub base_url: String,
    /// Request timeout.
    pub timeout: Duration,
    /// API key for authentication.
    pub api_key: Option<String>,
    /// Additional headers.
    pub headers: HashMap<String, String>,
    /// Maximum number of retries.
    pub max_retries: u32,
    /// Initial retry delay.
    pub initial_retry_delay: Duration,
    /// Maximum retry delay.
    pub max_retry_delay: Duration,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            base_url: "http://localhost:8080".to_string(),
            timeout: Duration::from_secs(30),
            api_key: None,
            headers: HashMap::new(),
            max_retries: 3,
            initial_retry_delay: Duration::from_millis(100),
            max_retry_delay: Duration::from_secs(5),
        }
    }
}

/// Feather Feature Store client.
///
/// # Example
///
/// ```rust,no_run
/// use feather_client::{FeatherClient, ClientConfig};
///
/// #[tokio::main]
/// async fn main() -> Result<(), Box<dyn std::error::Error>> {
///     let client = FeatherClient::new(ClientConfig::default())?;
///
///     // Check health
///     let health = client.health().await?;
///     println!("Server status: {}", health.status);
///
///     Ok(())
/// }
/// ```
pub struct FeatherClient {
    config: ClientConfig,
    http: Client,
    base_url: String,
}

impl FeatherClient {
    /// Create a new Feather client.
    pub fn new(config: ClientConfig) -> Result<Self> {
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));

        if let Some(ref api_key) = config.api_key {
            let auth_value = format!("Bearer {}", api_key);
            headers.insert(
                AUTHORIZATION,
                HeaderValue::from_str(&auth_value)
                    .map_err(|e| FeatherError::Config(format!("Invalid API key: {}", e)))?,
            );
        }

        for (key, value) in &config.headers {
            headers.insert(
                reqwest::header::HeaderName::try_from(key.as_str())
                    .map_err(|e| FeatherError::Config(format!("Invalid header name: {}", e)))?,
                HeaderValue::from_str(value)
                    .map_err(|e| FeatherError::Config(format!("Invalid header value: {}", e)))?,
            );
        }

        let http = Client::builder()
            .timeout(config.timeout)
            .default_headers(headers)
            .build()
            .map_err(|e| FeatherError::Config(format!("Failed to create HTTP client: {}", e)))?;

        let base_url = config.base_url.trim_end_matches('/').to_string();

        Ok(Self {
            config,
            http,
            base_url,
        })
    }

    /// Get the vector client.
    pub fn vectors(&self) -> VectorClient<'_> {
        VectorClient::new(self)
    }

    /// Get features for an entity.
    pub async fn get_features(
        &self,
        entity_id: &str,
        feature_names: Option<&[&str]>,
    ) -> Result<GetFeaturesResponse> {
        let mut url = format!("{}/v1/features?entity={}", self.base_url, entity_id);
        if let Some(names) = feature_names {
            for name in names {
                url.push_str(&format!("&feature={}", name));
            }
        }
        self.get(&url).await
    }

    /// Store features for an entity.
    pub async fn put_features(
        &self,
        entity_id: &str,
        features: HashMap<String, FeatureValue>,
        timestamp: Option<DateTime<Utc>>,
    ) -> Result<()> {
        #[derive(Serialize)]
        struct Request {
            entity_id: String,
            features: HashMap<String, FeatureValue>,
            #[serde(skip_serializing_if = "Option::is_none")]
            timestamp: Option<DateTime<Utc>>,
        }

        let request = Request {
            entity_id: entity_id.to_string(),
            features,
            timestamp,
        };

        self.post::<_, ()>(&format!("{}/v1/features", self.base_url), &request)
            .await
    }

    /// Get features for multiple entities.
    pub async fn batch_get(
        &self,
        entity_ids: &[&str],
        feature_names: Option<&[&str]>,
    ) -> Result<BatchGetResponse> {
        #[derive(Serialize)]
        struct Request<'a> {
            entities: &'a [&'a str],
            #[serde(skip_serializing_if = "Option::is_none")]
            features: Option<&'a [&'a str]>,
        }

        let request = Request {
            entities: entity_ids,
            features: feature_names,
        };

        self.post(&format!("{}/v1/features/batch", self.base_url), &request)
            .await
    }

    /// Get features at a specific point in time.
    pub async fn get_features_as_of(
        &self,
        entity_id: &str,
        as_of: DateTime<Utc>,
        feature_names: Option<&[&str]>,
    ) -> Result<GetFeaturesResponse> {
        let mut url = format!(
            "{}/v1/features/history?entity={}&as_of={}",
            self.base_url,
            entity_id,
            as_of.to_rfc3339()
        );
        if let Some(names) = feature_names {
            for name in names {
                url.push_str(&format!("&feature={}", name));
            }
        }
        self.get(&url).await
    }

    /// Get aggregated feature value.
    pub async fn get_aggregation(
        &self,
        entity_id: &str,
        feature: &str,
        function: AggFunction,
        window_seconds: i64,
    ) -> Result<AggregationResponse> {
        let function_str = match function {
            AggFunction::Count => "count",
            AggFunction::Sum => "sum",
            AggFunction::Avg => "avg",
            AggFunction::Min => "min",
            AggFunction::Max => "max",
            AggFunction::Last => "last",
        };

        let url = format!(
            "{}/v1/aggregation?entity={}&feature={}&function={}&window={}",
            self.base_url, entity_id, feature, function_str, window_seconds
        );
        self.get(&url).await
    }

    /// List all feature groups.
    pub async fn list_feature_groups(&self) -> Result<Vec<FeatureGroup>> {
        let response: ListFeatureGroupsResponse =
            self.get(&format!("{}/v1/schema/groups", self.base_url)).await?;
        Ok(response.groups)
    }

    /// Get a feature group by name.
    pub async fn get_feature_group(&self, name: &str) -> Result<Option<FeatureGroup>> {
        match self
            .get::<FeatureGroup>(&format!("{}/v1/schema/groups/{}", self.base_url, name))
            .await
        {
            Ok(group) => Ok(Some(group)),
            Err(FeatherError::NotFound(_)) => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Check server health.
    pub async fn health(&self) -> Result<HealthStatus> {
        self.get(&format!("{}/health", self.base_url)).await
    }

    /// Check if server is ready.
    pub async fn ready(&self) -> bool {
        self.get::<serde_json::Value>(&format!("{}/ready", self.base_url))
            .await
            .is_ok()
    }

    // HTTP helpers

    pub(crate) async fn get<T: DeserializeOwned>(&self, url: &str) -> Result<T> {
        self.request(reqwest::Method::GET, url, Option::<()>::None)
            .await
    }

    pub(crate) async fn post<B: Serialize, T: DeserializeOwned>(
        &self,
        url: &str,
        body: &B,
    ) -> Result<T> {
        self.request(reqwest::Method::POST, url, Some(body)).await
    }

    pub(crate) async fn delete(&self, url: &str) -> Result<()> {
        self.request(reqwest::Method::DELETE, url, Option::<()>::None)
            .await
    }

    async fn request<B: Serialize, T: DeserializeOwned>(
        &self,
        method: reqwest::Method,
        url: &str,
        body: Option<B>,
    ) -> Result<T> {
        let mut delay = self.config.initial_retry_delay;

        for attempt in 0..=self.config.max_retries {
            let mut request = self.http.request(method.clone(), url);

            if let Some(ref b) = body {
                request = request.json(b);
            }

            match request.send().await {
                Ok(response) => {
                    let status = response.status();

                    if status.is_success() {
                        let text = response.text().await?;
                        if text.is_empty() || std::any::type_name::<T>() == "()" {
                            // Return unit type for empty responses
                            return Ok(serde_json::from_str("null").unwrap_or_else(|_| {
                                unsafe { std::mem::zeroed() }
                            }));
                        }
                        return serde_json::from_str(&text).map_err(Into::into);
                    }

                    let body = response.text().await.unwrap_or_default();
                    let error = FeatherError::from_response(status.as_u16(), &body);

                    if error.is_retryable() && attempt < self.config.max_retries {
                        tokio::time::sleep(delay).await;
                        delay = std::cmp::min(delay * 2, self.config.max_retry_delay);
                        continue;
                    }

                    return Err(error);
                }
                Err(e) => {
                    let error = if e.is_timeout() {
                        FeatherError::Timeout
                    } else if e.is_connect() {
                        FeatherError::Connection(e.to_string())
                    } else {
                        FeatherError::Http(e)
                    };

                    if error.is_retryable() && attempt < self.config.max_retries {
                        tokio::time::sleep(delay).await;
                        delay = std::cmp::min(delay * 2, self.config.max_retry_delay);
                        continue;
                    }

                    return Err(error);
                }
            }
        }

        Err(FeatherError::Connection("Request failed after retries".to_string()))
    }

    pub(crate) fn base_url(&self) -> &str {
        &self.base_url
    }
}
