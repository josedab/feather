# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

1. **Do NOT** create a public GitHub issue for security vulnerabilities
2. Email your findings to: security@feather-store.io
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes (optional)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Initial Assessment**: We will provide an initial assessment within 7 days
- **Resolution Timeline**: We aim to resolve critical issues within 30 days
- **Disclosure**: We will coordinate with you on public disclosure timing

### Scope

The following are in scope for security reports:

- Authentication and authorization bypasses
- Data exposure or leakage
- Injection vulnerabilities (SQL, command, etc.)
- Cross-site scripting (XSS) in any web interfaces
- Denial of service vulnerabilities
- Cryptographic weaknesses
- Configuration issues leading to security problems

### Out of Scope

- Vulnerabilities in dependencies (report these upstream)
- Social engineering attacks
- Physical security issues
- Issues requiring unlikely user interaction

## Security Best Practices

When deploying Feather in production:

### Network Security

#### Metrics Endpoint Security (Port 9090)

The Prometheus metrics endpoint on port 9090 exposes operational data that could reveal sensitive information about your deployment:

- Request rates and patterns
- Error rates and types
- Cache hit rates and memory usage
- Feature names and access patterns
- Internal service topology

**Recommended mitigations:**

1. **Network isolation**: Bind the metrics port to a private network interface or localhost only
   ```yaml
   # In your configuration
   metrics:
     address: "127.0.0.1:9090"  # Only accessible locally
   ```

2. **Kubernetes NetworkPolicy**: Restrict access to metrics pods
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: feather-metrics-policy
   spec:
     podSelector:
       matchLabels:
         app: feather
     ingress:
       - from:
           - namespaceSelector:
               matchLabels:
                 name: monitoring
         ports:
           - port: 9090
   ```

3. **Firewall rules**: Block external access to port 9090 at the infrastructure level

4. **Service mesh**: Use a service mesh (Istio, Linkerd) to enforce mTLS for metrics scraping

5. **Reverse proxy authentication**: Place metrics behind an authenticated reverse proxy if external access is required

**Never expose port 9090 directly to the public internet.**

#### Other Network Recommendations

- Use TLS for all external-facing endpoints (ports 8080, 8081, 50051)
- Configure firewall rules to restrict access to management ports
- Use private subnets for inter-service communication

### Authentication

- Always configure explicit CORS origins (never use wildcard in production)
- Rotate API keys regularly
- Use strong, unique API keys per client/service

### Configuration

- Set `FEATHER_TRACING_INSECURE=false` in production
- Review and restrict file system permissions on data directories
- Use read-only file systems where possible

### Monitoring

- Enable audit logging for authentication events
- Monitor for unusual access patterns
- Set up alerts for authentication failures

## Security Features

Feather includes several security features:

- **API Key Authentication**: Secure API access with hashed keys
- **RBAC**: Role-based access control for fine-grained permissions
- **Rate Limiting**: Protection against abuse and DoS
- **Security Headers**: Standard security headers on HTTP responses
- **Audit Logging**: Track authentication and authorization events
- **TLS Support**: Encrypted transport for all protocols
