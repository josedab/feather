// Package clientip provides secure client IP resolution for HTTP requests.
// It validates proxy headers against a list of trusted proxy CIDRs to prevent
// IP spoofing attacks.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// Resolver resolves client IP addresses from HTTP requests.
// It only trusts X-Forwarded-For and X-Real-IP headers when the
// request comes from a trusted proxy.
type Resolver struct {
	trustedProxies []*net.IPNet
}

// NewResolver creates a new IP resolver with the given trusted proxy CIDRs.
// Common values include:
//   - "127.0.0.1/8" for localhost
//   - "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16" for private networks
//   - "::1/128" for IPv6 localhost
//
// If no CIDRs are provided, proxy headers are never trusted and RemoteAddr
// is always used (safest default).
func NewResolver(trustedCIDRs []string) (*Resolver, error) {
	r := &Resolver{
		trustedProxies: make([]*net.IPNet, 0, len(trustedCIDRs)),
	}

	for _, cidr := range trustedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try parsing as a single IP
			ip := net.ParseIP(cidr)
			if ip == nil {
				return nil, &InvalidCIDRError{CIDR: cidr, Err: err}
			}
			// Convert single IP to /32 or /128 CIDR
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			network = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		r.trustedProxies = append(r.trustedProxies, network)
	}

	return r, nil
}

// InvalidCIDRError is returned when a CIDR string cannot be parsed.
type InvalidCIDRError struct {
	CIDR string
	Err  error
}

func (e *InvalidCIDRError) Error() string {
	return "invalid CIDR '" + e.CIDR + "': " + e.Err.Error()
}

func (e *InvalidCIDRError) Unwrap() error {
	return e.Err
}

// GetClientIP returns the client's IP address from the request.
// If the request comes from a trusted proxy, it parses the X-Forwarded-For
// or X-Real-IP headers. Otherwise, it returns the RemoteAddr.
func (r *Resolver) GetClientIP(req *http.Request) string {
	remoteIP := extractIP(req.RemoteAddr)

	// If no trusted proxies configured, always use RemoteAddr (safe default)
	if len(r.trustedProxies) == 0 {
		return remoteIP
	}

	// Check if RemoteAddr is from a trusted proxy
	if !r.isTrustedProxy(remoteIP) {
		return remoteIP
	}

	// Request is from a trusted proxy, check headers
	// X-Forwarded-For takes precedence (standard header)
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs: client, proxy1, proxy2, ...
		// The leftmost non-trusted IP is the client
		return r.parseXForwardedFor(xff, remoteIP)
	}

	// Fall back to X-Real-IP
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	return remoteIP
}

// parseXForwardedFor parses the X-Forwarded-For header and returns the
// leftmost IP that is not from a trusted proxy.
func (r *Resolver) parseXForwardedFor(xff string, fallback string) string {
	// Split by comma and iterate from left to right
	parts := strings.Split(xff, ",")
	for _, part := range parts {
		ip := strings.TrimSpace(part)
		if ip == "" {
			continue
		}
		// Validate it's a valid IP
		if net.ParseIP(ip) == nil {
			continue
		}
		// Return the first non-trusted IP (the real client)
		if !r.isTrustedProxy(ip) {
			return ip
		}
	}
	return fallback
}

// isTrustedProxy checks if the given IP is in the trusted proxy list.
func (r *Resolver) isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, network := range r.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// extractIP extracts the IP address from an address string (host:port format).
func extractIP(addr string) string {
	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") {
		if idx := strings.LastIndex(addr, "]"); idx != -1 {
			return addr[1:idx]
		}
	}

	// Try to split host:port
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port, return as-is
		return addr
	}
	return host
}

// DefaultPrivateNetworks returns the standard private network CIDRs.
// Use this as a starting point for trusted proxies in private deployments.
func DefaultPrivateNetworks() []string {
	return []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918 Class A private
		"172.16.0.0/12",  // RFC1918 Class B private
		"192.168.0.0/16", // RFC1918 Class C private
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
}
