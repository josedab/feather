package io.feather.client

/**
 * Base exception for all Feather client errors.
 */
open class FeatherException(
    message: String,
    val statusCode: Int? = null,
    val errorCode: String? = null,
    cause: Throwable? = null
) : RuntimeException(message, cause)

/**
 * Thrown when a requested resource is not found.
 */
class NotFoundException(
    message: String
) : FeatherException(message, 404, "NOT_FOUND")

/**
 * Thrown when a request is invalid.
 */
class ValidationException(
    message: String
) : FeatherException(message, 400, "VALIDATION_ERROR")

/**
 * Thrown when a connection error occurs.
 */
class ConnectionException(
    message: String,
    cause: Throwable? = null
) : FeatherException(message, null, "CONNECTION_ERROR", cause)

/**
 * Thrown when a request times out.
 */
class TimeoutException(
    message: String,
    cause: Throwable? = null
) : FeatherException(message, null, "TIMEOUT", cause)

/**
 * Thrown when authentication fails.
 */
class AuthenticationException(
    message: String
) : FeatherException(message, 401, "AUTHENTICATION_ERROR")

/**
 * Thrown when rate limit is exceeded.
 */
class RateLimitException(
    message: String,
    val retryAfter: Long? = null
) : FeatherException(message, 429, "RATE_LIMIT_EXCEEDED")
