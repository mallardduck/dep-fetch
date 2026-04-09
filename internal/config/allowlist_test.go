package config

import "testing"

func TestInReleaseChecksumAllowlist(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"rancher/dep-fetch", true},
		{"rancher/charts-build-scripts", true},
		{"rancher/ob-charts-tool", true},
		{"rancher/unknown-tool", false},
		{"other/dep-fetch", false},
		{"", false},
	}
	for _, tt := range tests {
		got := inReleaseChecksumAllowlist(tt.source)
		if got != tt.want {
			t.Errorf("inReleaseChecksumAllowlist(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestInLatestPermitted(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"rancher/charts-build-scripts", true},
		{"rancher/ob-charts-tool", true},
		{"rancher/dep-fetch", false}, // in allowlist but latestAllowed=false
		{"rancher/unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		got := inLatestPermitted(tt.source)
		if got != tt.want {
			t.Errorf("inLatestPermitted(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}
