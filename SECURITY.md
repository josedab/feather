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

- Run the metrics endpoint (port 9090) on an internal network only
- Use TLS for all external-facing endpoints
- Configure firewall rules to restrict access to management ports

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
