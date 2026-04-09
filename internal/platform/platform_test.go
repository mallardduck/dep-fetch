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
