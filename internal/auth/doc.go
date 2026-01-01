// Package auth provides authentication and authorization for the feature store.
//
// It implements API key-based authentication with role-based access control (RBAC),
// multi-tenant isolation, rate limiting, and audit logging. The package supports
// hierarchical permissions for fine-grained access control over features and namespaces.
//
// Key components:
//   - AccessController: Manages API keys, roles, tenants, and permissions
//   - Middleware: HTTP middleware for authenticating and authorizing requests
//   - AuditLogger: Records authentication and authorization events
//
// Example usage:
//
//	controller := auth.NewAccessController()
//	middleware := auth.NewMiddleware(controller)
//	http.Handle("/v1/features", middleware.Authenticate(handler))
package auth
