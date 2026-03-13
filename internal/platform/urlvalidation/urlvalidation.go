// Package urlvalidation provides URL validation to prevent SSRF attacks.
package urlvalidation

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateRanges defines CIDR ranges that should be blocked for outbound requests.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
	}
	for _, cidr := range cidrs {
		_, block, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, block)
	}
}

// isPrivateIP checks if an IP address falls within a private/reserved range.
func isPrivateIP(ip net.IP) bool {
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateWebhookURL checks that a URL is safe for outbound HTTP requests.
// It rejects URLs targeting private/reserved IP ranges to prevent SSRF.
func ValidateWebhookURL(rawURL string) error {
	return ValidateWebhookURLWithAllowlist(rawURL, nil)
}

// ValidateWebhookURLWithAllowlist checks URL safety while allowing specific CIDRs.
// Allowed CIDRs bypass the private IP check (useful for testing and internal services).
func ValidateWebhookURLWithAllowlist(rawURL string, allowedCIDRs []string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("URL must use http or https scheme")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL must contain a hostname")
	}

	// Parse allowed CIDRs
	var allowed []*net.IPNet
	for _, cidr := range allowedCIDRs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			allowed = append(allowed, block)
		}
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve hostname %q: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) && !isAllowed(ip, allowed) {
			return fmt.Errorf("URL resolves to private/reserved IP address")
		}
	}

	return nil
}

func isAllowed(ip net.IP, allowed []*net.IPNet) bool {
	for _, block := range allowed {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}
