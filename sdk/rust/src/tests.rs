//! Tests for the Feather client.

#[cfg(test)]
mod tests {
    use crate::client::{ClientConfig, FeatherClient};
    use crate::error::FeatherError;
    use crate::models::*;
    use std::collections::HashMap;
    use std::time::Duration;

    // Configuration tests

    #[test]
    fn test_client_config_default() {
        let config = ClientConfig::default();
        assert_eq!(config.base_url, "http://localhost:8080");
        assert_eq!(config.timeout, Duration::from_secs(30));
        assert!(config.api_key.is_none());
        assert!(config.headers.is_empty());
        assert_eq!(config.max_retries, 3);
    }

    #[test]
    fn test_client_config_custom() {
        let config = ClientConfig {
            base_url: "http://custom:9090".to_string(),
            timeout: Duration::from_secs(60),
            api_key: Some("test-key".to_string()),
            headers: HashMap::from([("X-Custom".to_string(), "value".to_string())]),
            max_retries: 5,
            initial_retry_delay: Duration::from_millis(200),
            max_retry_delay: Duration::from_secs(10),
        };

        assert_eq!(config.base_url, "http://custom:9090");
        assert_eq!(config.timeout, Duration::from_secs(60));
        assert_eq!(config.api_key, Some("test-key".to_string()));
        assert_eq!(config.max_retries, 5);
    }

    #[test]
    fn test_client_creation() {
        let config = ClientConfig::default();
        let result = FeatherClient::new(config);
        assert!(result.is_ok());
    }

    #[test]
    fn test_client_with_api_key() {
        let config = ClientConfig {
            api_key: Some("test-api-key".to_string()),
            ..Default::default()
        };
        let result = FeatherClient::new(config);
        assert!(result.is_ok());
    }

    #[test]
    fn test_client_base_url_trailing_slash_removal() {
        let config = ClientConfig {
            base_url: "http://localhost:8080/".to_string(),
            ..Default::default()
        };
        let client = FeatherClient::new(config).unwrap();
        assert_eq!(client.base_url(), "http://localhost:8080");
    }

    // Error tests

    #[test]
    fn test_error_from_response_not_found() {
        let error = FeatherError::from_response(404, "resource not found");
        assert!(matches!(error, FeatherError::NotFound(_)));
        assert!(error.is_not_found());
    }

    #[test]
    fn test_error_from_response_validation() {
        let error = FeatherError::from_response(400, "invalid request");
        assert!(matches!(error, FeatherError::Validation(_)));
    }

    #[test]
    fn test_error_from_response_authentication() {
        let error = FeatherError::from_response(401, "unauthorized");
        assert!(matches!(error, FeatherError::Authentication(_)));
    }

    #[test]
    fn test_error_from_response_rate_limit() {
        let error = FeatherError::from_response(429, "too many requests");
        assert!(matches!(error, FeatherError::RateLimit(_)));
        assert!(error.is_retryable());
    }

    #[test]
    fn test_error_from_response_server_error() {
        let error = FeatherError::from_response(500, "internal server error");
        assert!(matches!(error, FeatherError::Server { status: 500, .. }));
        assert!(error.is_retryable());
    }

    #[test]
    fn test_error_from_response_with_json() {
        let body = r#"{"error": "custom error message"}"#;
        let error = FeatherError::from_response(400, body);
        if let FeatherError::Validation(msg) = error {
            assert_eq!(msg, "custom error message");
        } else {
            panic!("Expected Validation error");
        }
    }

    #[test]
    fn test_error_is_retryable() {
        assert!(FeatherError::Connection("failed".to_string()).is_retryable());
        assert!(FeatherError::Timeout.is_retryable());
        assert!(FeatherError::RateLimit("limited".to_string()).is_retryable());
        assert!(FeatherError::Server {
            status: 503,
            message: "unavailable".to_string()
        }
        .is_retryable());

        assert!(!FeatherError::NotFound("not found".to_string()).is_retryable());
        assert!(!FeatherError::Validation("invalid".to_string()).is_retryable());
        assert!(!FeatherError::Authentication("denied".to_string()).is_retryable());
    }

    #[test]
    fn test_error_display() {
        let error = FeatherError::NotFound("user:123".to_string());
        assert_eq!(error.to_string(), "Not found: user:123");

        let error = FeatherError::Timeout;
        assert_eq!(error.to_string(), "Request timeout");

        let error = FeatherError::Server {
            status: 500,
            message: "internal error".to_string(),
        };
        assert_eq!(error.to_string(), "Server error (500): internal error");
    }

    // Model tests

    #[test]
    fn test_distance_type_default() {
        let dt = DistanceType::default();
        assert_eq!(dt, DistanceType::Cosine);
    }

    #[test]
    fn test_distance_type_serialization() {
        let cosine = DistanceType::Cosine;
        let json = serde_json::to_string(&cosine).unwrap();
        assert_eq!(json, "\"cosine\"");

        let euclidean = DistanceType::Euclidean;
        let json = serde_json::to_string(&euclidean).unwrap();
        assert_eq!(json, "\"euclidean\"");

        let dot = DistanceType::DotProduct;
        let json = serde_json::to_string(&dot).unwrap();
        assert_eq!(json, "\"dot_product\"");
    }

    #[test]
    fn test_distance_type_deserialization() {
        let cosine: DistanceType = serde_json::from_str("\"cosine\"").unwrap();
        assert_eq!(cosine, DistanceType::Cosine);

        let euclidean: DistanceType = serde_json::from_str("\"euclidean\"").unwrap();
        assert_eq!(euclidean, DistanceType::Euclidean);
    }

    #[test]
    fn test_data_type_serialization() {
        let dt = DataType::String;
        let json = serde_json::to_string(&dt).unwrap();
        assert_eq!(json, "\"string\"");

        let dt = DataType::Int64;
        let json = serde_json::to_string(&dt).unwrap();
        assert_eq!(json, "\"int64\"");

        let dt = DataType::Float64List;
        let json = serde_json::to_string(&dt).unwrap();
        assert_eq!(json, "\"float64_list\"");
    }

    #[test]
    fn test_agg_function_serialization() {
        let func = AggFunction::Sum;
        let json = serde_json::to_string(&func).unwrap();
        assert_eq!(json, "\"sum\"");

        let func = AggFunction::Avg;
        let json = serde_json::to_string(&func).unwrap();
        assert_eq!(json, "\"avg\"");
    }

    #[test]
    fn test_vector_record_creation() {
        let record = VectorRecord {
            id: "vec1".to_string(),
            vector: vec![0.1, 0.2, 0.3],
            metadata: Some(HashMap::from([(
                "title".to_string(),
                serde_json::json!("test"),
            )])),
        };

        assert_eq!(record.id, "vec1");
        assert_eq!(record.vector.len(), 3);
        assert!(record.metadata.is_some());
    }

    #[test]
    fn test_vector_record_serialization() {
        let record = VectorRecord {
            id: "vec1".to_string(),
            vector: vec![0.1, 0.2, 0.3],
            metadata: None,
        };

        let json = serde_json::to_string(&record).unwrap();
        assert!(json.contains("\"id\":\"vec1\""));
        assert!(json.contains("\"vector\":[0.1,0.2,0.3]"));
        // metadata should be skipped when None
        assert!(!json.contains("\"metadata\""));
    }

    #[test]
    fn test_vector_search_result() {
        let json = r#"{"id": "vec1", "score": 0.95, "metadata": {"key": "value"}}"#;
        let result: VectorSearchResult = serde_json::from_str(json).unwrap();

        assert_eq!(result.id, "vec1");
        assert_eq!(result.score, 0.95);
        assert!(result.metadata.is_some());
    }

    #[test]
    fn test_feature_group_deserialization() {
        let json = r#"{
            "name": "user_features",
            "entity_type": "user",
            "features": [
                {"name": "age", "data_type": "int64"},
                {"name": "country", "data_type": "string"}
            ]
        }"#;

        let group: FeatureGroup = serde_json::from_str(json).unwrap();
        assert_eq!(group.name, "user_features");
        assert_eq!(group.entity_type, "user");
        assert_eq!(group.features.len(), 2);
        assert_eq!(group.features[0].name, "age");
        assert_eq!(group.features[0].data_type, DataType::Int64);
    }

    #[test]
    fn test_feature_spec_with_default_value() {
        let spec = FeatureSpec {
            name: "count".to_string(),
            data_type: DataType::Int64,
            default_value: Some(serde_json::json!(0)),
        };

        let json = serde_json::to_string(&spec).unwrap();
        assert!(json.contains("\"default_value\":0"));
    }

    #[test]
    fn test_health_status_deserialization() {
        let json = r#"{
            "status": "healthy",
            "components": {
                "storage": {"status": "healthy", "latency": 5}
            },
            "version": "1.0.0",
            "uptime": 3600
        }"#;

        let health: HealthStatus = serde_json::from_str(json).unwrap();
        assert_eq!(health.status, "healthy");
        assert!(health.components.contains_key("storage"));
        assert_eq!(health.version, Some("1.0.0".to_string()));
        assert_eq!(health.uptime, Some(3600));
    }

    #[test]
    fn test_get_features_response_deserialization() {
        let json = r#"{
            "entity_id": "user:123",
            "features": {
                "age": 25,
                "name": "Alice"
            }
        }"#;

        let response: GetFeaturesResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.entity_id, "user:123");
        assert_eq!(response.features.len(), 2);
        assert_eq!(response.features.get("age").unwrap(), &serde_json::json!(25));
    }

    #[test]
    fn test_batch_get_response_deserialization() {
        let json = r#"{
            "results": [
                {"entity_id": "user:1", "features": {"age": 25}},
                {"entity_id": "user:2", "features": {"age": 30}}
            ]
        }"#;

        let response: BatchGetResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.results.len(), 2);
        assert_eq!(response.results[0].entity_id, "user:1");
    }

    #[test]
    fn test_aggregation_response_deserialization() {
        let json = r#"{
            "entity_id": "user:123",
            "feature": "purchases",
            "function": "sum",
            "value": 1500.50,
            "window_start": "2024-01-01T00:00:00Z",
            "window_end": "2024-01-01T01:00:00Z"
        }"#;

        let response: AggregationResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.entity_id, "user:123");
        assert_eq!(response.feature, "purchases");
        assert_eq!(response.function, AggFunction::Sum);
        assert_eq!(response.value, 1500.50);
    }

    #[test]
    fn test_create_index_request_serialization() {
        let request = CreateIndexRequest {
            name: "embeddings".to_string(),
            dimension: 384,
            distance_type: Some(DistanceType::Cosine),
        };

        let json = serde_json::to_string(&request).unwrap();
        assert!(json.contains("\"name\":\"embeddings\""));
        assert!(json.contains("\"dimension\":384"));
        assert!(json.contains("\"distance_type\":\"cosine\""));
    }

    #[test]
    fn test_search_vectors_request_serialization() {
        let request = SearchVectorsRequest {
            vector: vec![0.1, 0.2, 0.3],
            top_k: Some(10),
            filter: None,
            include_metadata: Some(true),
            include_vector: Some(false),
        };

        let json = serde_json::to_string(&request).unwrap();
        assert!(json.contains("\"vector\":[0.1,0.2,0.3]"));
        assert!(json.contains("\"top_k\":10"));
        assert!(json.contains("\"include_metadata\":true"));
        // filter should be skipped when None
        assert!(!json.contains("\"filter\""));
    }

    #[test]
    fn test_vector_index_deserialization() {
        let json = r#"{
            "name": "embeddings",
            "dimension": 384,
            "distance_type": "cosine",
            "vector_count": 10000,
            "created_at": "2024-01-01T00:00:00Z"
        }"#;

        let index: VectorIndex = serde_json::from_str(json).unwrap();
        assert_eq!(index.name, "embeddings");
        assert_eq!(index.dimension, 384);
        assert_eq!(index.distance_type, DistanceType::Cosine);
        assert_eq!(index.vector_count, 10000);
    }

    #[test]
    fn test_upsert_response_deserialization() {
        let json = r#"{"upserted_count": 100}"#;
        let response: UpsertResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.upserted_count, 100);
    }

    #[test]
    fn test_list_indexes_response_deserialization() {
        let json = r#"{"indexes": ["index1", "index2", "index3"]}"#;
        let response: ListIndexesResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.indexes.len(), 3);
        assert_eq!(response.indexes[0], "index1");
    }

    #[test]
    fn test_component_health_deserialization() {
        let json = r#"{"status": "healthy", "message": "OK", "latency": 10}"#;
        let health: ComponentHealth = serde_json::from_str(json).unwrap();
        assert_eq!(health.status, "healthy");
        assert_eq!(health.message, Some("OK".to_string()));
        assert_eq!(health.latency, Some(10));
    }

    #[test]
    fn test_entity_features_with_timestamp() {
        let json = r#"{
            "entity_id": "user:123",
            "features": {"age": 25},
            "timestamp": "2024-01-01T12:00:00Z"
        }"#;

        let entity: EntityFeatures = serde_json::from_str(json).unwrap();
        assert_eq!(entity.entity_id, "user:123");
        assert!(entity.timestamp.is_some());
    }

    // Vector search response tests

    #[test]
    fn test_vector_search_response_deserialization() {
        let json = r#"{
            "results": [
                {"id": "vec1", "score": 0.95},
                {"id": "vec2", "score": 0.85}
            ],
            "took": 15
        }"#;

        let response: VectorSearchResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.results.len(), 2);
        assert_eq!(response.took, 15);
        assert_eq!(response.results[0].id, "vec1");
        assert_eq!(response.results[0].score, 0.95);
    }
}
