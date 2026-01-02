//! Error types for the Feather client.

use thiserror::Error;

/// Result type for Feather client operations.
pub type Result<T> = std::result::Result<T, FeatherError>;

/// Errors that can occur when using the Feather client.
#[derive(Error, Debug)]
pub enum FeatherError {
    /// The requested resource was not found.
    #[error("Not found: {0}")]
    NotFound(String),

    /// The request was invalid.
    #[error("Validation error: {0}")]
    Validation(String),

    /// Authentication failed.
    #[error("Authentication error: {0}")]
    Authentication(String),

    /// Rate limit exceeded.
    #[error("Rate limit exceeded: {0}")]
    RateLimit(String),

    /// Server error.
    #[error("Server error ({status}): {message}")]
    Server {
        status: u16,
        message: String,
    },

    /// Connection error.
    #[error("Connection error: {0}")]
    Connection(String),

    /// Request timeout.
    #[error("Request timeout")]
    Timeout,

    /// JSON serialization/deserialization error.
    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    /// HTTP client error.
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),

    /// Invalid configuration.
    #[error("Invalid configuration: {0}")]
    Config(String),
}

impl FeatherError {
    /// Create a new server error from status code and response body.
    pub(crate) fn from_response(status: u16, body: &str) -> Self {
        let message = serde_json::from_str::<serde_json::Value>(body)
            .ok()
            .and_then(|v| v.get("error").and_then(|e| e.as_str()).map(String::from))
            .unwrap_or_else(|| body.to_string());

        match status {
            404 => FeatherError::NotFound(message),
            400 => FeatherError::Validation(message),
            401 => FeatherError::Authentication(message),
            429 => FeatherError::RateLimit(message),
            _ => FeatherError::Server { status, message },
        }
    }

    /// Check if this error indicates the resource was not found.
    pub fn is_not_found(&self) -> bool {
        matches!(self, FeatherError::NotFound(_))
    }

    /// Check if this error is retryable.
    pub fn is_retryable(&self) -> bool {
        matches!(
            self,
            FeatherError::Connection(_)
                | FeatherError::Timeout
                | FeatherError::Server { status: 500..=599, .. }
                | FeatherError::RateLimit(_)
        )
    }
}
