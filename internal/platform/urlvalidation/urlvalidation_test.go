package urlvalidation

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 other", "127.0.0.2", true},
		{"RFC1918 10.x", "10.0.0.1", true},
		{"RFC1918 172.16.x", "172.16.0.1", true},
		{"RFC1918 172.31.x", "172.31.255.255", true},
		{"RFC1918 192.168.x", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"loopback v6", "::1", true},
		{"link-local v6", "fe80::1", true},
		{"unique local v6", "fd00::1", true},
		{"public IP", "8.8.8.8", false},
		{"public IP 2", "1.1.1.1", false},
		{"public 172.15", "172.15.255.255", false},
		{"public 172.32", "172.32.0.1", false},
		{"public v6", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/webhook", false},
		{"valid http", "http://example.com/webhook", false},
		{"no scheme", "example.com/webhook", true},
		{"ftp scheme", "ftp://example.com/webhook", true},
		{"empty URL", "", true},
		{"no hostname", "http:///path", true},
		// Private IPs resolved via DNS are harder to test without mocking,
		// but we can test localhost which should resolve to 127.0.0.1
		{"localhost", "http://localhost/webhook", true},
		{"loopback IP", "http://127.0.0.1/webhook", true},
		{"private 10.x", "http://10.0.0.1/webhook", true},
		{"private 192.168.x", "http://192.168.1.1/webhook", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWebhookURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWebhookURLSchemeCheck(t *testing.T) {
	// These should fail before even trying to resolve
	badSchemes := []string{
		"ftp://example.com",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"data:text/html,<h1>hi</h1>",
	}

	for _, url := range badSchemes {
		t.Run(url, func(t *testing.T) {
			err := ValidateWebhookURL(url)
			if err == nil {
				t.Errorf("expected error for URL with non-http(s) scheme: %s", url)
			}
		})
	}
}
