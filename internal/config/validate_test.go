package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	validChecksums := map[string]string{"linux/amd64": "abc123"}

	pinnedTool := func(name, version string, checksums map[string]string) Tool {
		return Tool{Name: name, Version: version, Source: "rancher/" + name, Mode: ModePinned, Checksums: checksums}
	}
	rcTool := func(source, version string) Tool {
		return Tool{Name: "tool", Version: version, Source: source, Mode: ModeReleaseChecksums}
	}

	tests := []struct {
		name    string
		tools   []Tool
		wantErr string
	}{
		{
			name:  "valid pinned tool",
			tools: []Tool{pinnedTool("mytool", "v1.0.0", validChecksums)},
		},
		{
			name:  "valid release-checksums tool",
			tools: []Tool{rcTool("rancher/charts-build-scripts", "v1.0.0")},
		},
		{
			name:  "release-checksums latest for latest-permitted source",
			tools: []Tool{rcTool("rancher/charts-build-scripts", "latest")},
		},
		{
			name:    "missing name",
			tools:   []Tool{{Version: "v1.0.0", Source: "a/b", Mode: ModePinned, Checksums: validChecksums}},
			wantErr: "name is required",
		},
		{
			name:    "missing version",
			tools:   []Tool{{Name: "tool", Source: "a/b", Mode: ModePinned, Checksums: validChecksums}},
			wantErr: `"tool": version is required`,
		},
		{
			name:    "missing source",
			tools:   []Tool{{Name: "tool", Version: "v1.0.0", Mode: ModePinned, Checksums: validChecksums}},
			wantErr: `"tool": source is required`,
		},
		{
			name:    "invalid source format missing slash",
			tools:   []Tool{{Name: "tool", Version: "v1.0.0", Source: "nodomain", Mode: ModePinned, Checksums: validChecksums}},
			wantErr: "owner/repo format",
		},
		{
			name:    "pinned with latest version",
			tools:   []Tool{{Name: "tool", Version: "latest", Source: "a/b", Mode: ModePinned, Checksums: validChecksums}},
			wantErr: "latest is not valid with mode: pinned",
		},
		{
			name:    "pinned without checksums",
			tools:   []Tool{{Name: "tool", Version: "v1.0.0", Source: "a/b", Mode: ModePinned}},
			wantErr: "requires checksums",
		},
		{
			name:    "release-checksums non-allowlisted source",
			tools:   []Tool{rcTool("unknown/repo", "v1.0.0")},
			wantErr: "not in the release-checksums allowlist",
		},
		{
			name:    "release-checksums latest for non-latest-permitted source",
			tools:   []Tool{rcTool("rancher/dep-fetch", "latest")},
			wantErr: "version: latest is only valid for allowlisted internal tool repos",
		},
		{
			name:    "missing mode",
			tools:   []Tool{{Name: "tool", Version: "v1.0.0", Source: "a/b"}},
			wantErr: "mode is required",
		},
		{
			name:    "unknown mode",
			tools:   []Tool{{Name: "tool", Version: "v1.0.0", Source: "a/b", Mode: "magic"}},
			wantErr: "unknown mode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&Config{Tools: tt.tools})
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validate() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
