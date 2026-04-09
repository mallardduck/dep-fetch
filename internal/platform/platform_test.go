package platform

import (
	"runtime"
	"testing"
)

func TestCurrent(t *testing.T) {
	os, arch := Current()
	if os != runtime.GOOS {
		t.Errorf("Current() os = %q, want %q", os, runtime.GOOS)
	}
	if arch != runtime.GOARCH {
		t.Errorf("Current() arch = %q, want %q", arch, runtime.GOARCH)
	}
}

func TestAltArch(t *testing.T) {
	tests := []struct {
		arch string
		want string
	}{
		{"amd64", "x86_64"},
		{"arm64", "arm64"},
		{"386", "386"},
		{"s390x", "s390x"},
		{"", ""},
	}
	for _, tt := range tests {
		got := AltArch(tt.arch)
		if got != tt.want {
			t.Errorf("AltArch(%q) = %q, want %q", tt.arch, got, tt.want)
		}
	}
}
