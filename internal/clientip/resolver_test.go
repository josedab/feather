package clientip

import (
	"net/http"
	"testing"
)

func TestNewResolver(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		wantErr bool
	}{
		{
			name:    "empty CIDRs",
			cidrs:   nil,
			wantErr: false,
		},
		{
			name:    "valid CIDR",
			cidrs:   []string{"10.0.0.0/8"},
			wantErr: false,
		},
		{
			name:    "valid single IP",
			cidrs:   []string{"192.168.1.1"},
			wantErr: false,
		},
		{
			name:    "valid IPv6 CIDR",
			cidrs:   []string{"::1/128"},
			wantErr: false,
		},
		{
			name:    "multiple valid CIDRs",
			cidrs:   []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			wantErr: false,
		},
		{
			name:    "invalid CIDR",
			cidrs:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "invalid CIDR mask",
			cidrs:   []string{"10.0.0.0/99"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewResolver(tt.cidrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResolver() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolver_GetClientIP(t *testing.T) {
	tests := []struct {
		name          string
		trustedCIDRs  []string
		remoteAddr    string
		xff           string
		xRealIP       string
		expectedIP    string
	}{
		{
			name:         "no trusted proxies - uses RemoteAddr",
			trustedCIDRs: nil,
			remoteAddr:   "1.2.3.4:12345",
			xff:          "10.0.0.1",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "untrusted proxy - ignores XFF",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "8.8.8.8:12345",
			xff:          "1.2.3.4",
			expectedIP:   "8.8.8.8",
		},
		{
			name:         "trusted proxy - uses XFF",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "1.2.3.4",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "trusted proxy - uses X-Real-IP when no XFF",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "",
			xRealIP:      "1.2.3.4",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "trusted proxy - XFF takes precedence over X-Real-IP",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "1.2.3.4",
			xRealIP:      "5.6.7.8",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "trusted proxy - multiple IPs in XFF, first untrusted is client",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "1.2.3.4, 10.0.0.2, 10.0.0.3",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "trusted proxy - all IPs in XFF are trusted, use first",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "10.0.0.5, 10.0.0.2, 10.0.0.3",
			expectedIP:   "10.0.0.1",
		},
		{
			name:         "localhost trusted",
			trustedCIDRs: []string{"127.0.0.0/8"},
			remoteAddr:   "127.0.0.1:12345",
			xff:          "1.2.3.4",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "IPv6 localhost trusted",
			trustedCIDRs: []string{"::1/128"},
			remoteAddr:   "[::1]:12345",
			xff:          "2001:db8::1",
			expectedIP:   "2001:db8::1",
		},
		{
			name:         "private network trusted",
			trustedCIDRs: DefaultPrivateNetworks(),
			remoteAddr:   "192.168.1.1:12345",
			xff:          "1.2.3.4",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "XFF with spaces",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "  1.2.3.4  ,  10.0.0.2  ",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "invalid XFF IP - uses RemoteAddr",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "not-an-ip",
			expectedIP:   "10.0.0.1",
		},
		{
			name:         "empty XFF - falls back to X-Real-IP",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "",
			xRealIP:      "1.2.3.4",
			expectedIP:   "1.2.3.4",
		},
		{
			name:         "no headers - uses RemoteAddr",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:12345",
			xff:          "",
			xRealIP:      "",
			expectedIP:   "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewResolver(tt.trustedCIDRs)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}

			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			got := resolver.GetClientIP(req)
			if got != tt.expectedIP {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.expectedIP)
			}
		})
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"192.168.1.1", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"::1", "::1"},
		{"[2001:db8::1]:443", "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := extractIP(tt.addr)
			if got != tt.expected {
				t.Errorf("extractIP(%q) = %q, want %q", tt.addr, got, tt.expected)
			}
		})
	}
}
