package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"

	"github.com/mallardduck/dep-fetch/internal/config"
	"github.com/mallardduck/dep-fetch/internal/receipt"
)

// --- selectTools ---

func TestSelectTools_NoFilter(t *testing.T) {
	tools := []config.Tool{
		{Name: "a"},
		{Name: "b"},
	}
	got, err := selectTools(tools, nil)
	if err != nil {
		t.Fatalf("selectTools() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("selectTools() len = %d, want 2", len(got))
	}
}

func TestSelectTools_WithFilter(t *testing.T) {
	tools := []config.Tool{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	got, err := selectTools(tools, []string{"a", "c"})
	if err != nil {
		t.Fatalf("selectTools() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("selectTools() len = %d, want 2", len(got))
	}
	names := map[string]bool{}
	for _, t := range got {
		names[t.Name] = true
	}
	if !names["a"] || !names["c"] {
		t.Errorf("selectTools() got %v, want a and c", names)
	}
}

func TestSelectTools_UnknownTool(t *testing.T) {
	tools := []config.Tool{{Name: "a"}}
	_, err := selectTools(tools, []string{"unknown"})
	if err == nil {
		t.Error("selectTools() expected error for unknown tool, got nil")
	}
}

func TestSelectTools_EmptyList(t *testing.T) {
	got, err := selectTools(nil, nil)
	if err != nil {
		t.Fatalf("selectTools() unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("selectTools() = %v, want nil", got)
	}
}

// --- ToolStatus ---

func TestToolStatus_IsInstalled(t *testing.T) {
	if (ToolStatus{InstalledVersion: "v1.0.0"}).IsInstalled() != true {
		t.Error("IsInstalled() = false, want true when version is set")
	}
	if (ToolStatus{}).IsInstalled() != false {
		t.Error("IsInstalled() = true, want false when version is empty")
	}
}

func TestToolStatus_IsUpToDate(t *testing.T) {
	tests := []struct {
		name   string
		status ToolStatus
		want   bool
	}{
		{
			name:   "declared version matches installed",
			status: ToolStatus{DeclaredVersion: "v1.0.0", InstalledVersion: "v1.0.0"},
			want:   true,
		},
		{
			name:   "declared version mismatch",
			status: ToolStatus{DeclaredVersion: "v2.0.0", InstalledVersion: "v1.0.0"},
			want:   false,
		},
		{
			name:   "resolved version used when set",
			status: ToolStatus{DeclaredVersion: "latest", ResolvedVersion: "v1.0.0", InstalledVersion: "v1.0.0"},
			want:   true,
		},
		{
			name:   "resolved version mismatch",
			status: ToolStatus{DeclaredVersion: "latest", ResolvedVersion: "v2.0.0", InstalledVersion: "v1.0.0"},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsUpToDate(); got != tt.want {
				t.Errorf("IsUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- extractBinary ---

func TestExtractBinary_DirectBinary(t *testing.T) {
	data := []byte("binary content")
	got, err := extractBinary(data, "mytool", "")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("extractBinary() = %q, want %q", got, data)
	}
}

func TestExtractBinary_GzipDecompress(t *testing.T) {
	original := []byte("binary data")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(original)
	gz.Close()

	got, err := extractBinary(buf.Bytes(), "mytool.gz", "")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("extractBinary() = %q, want %q", got, original)
	}
}

func TestExtractBinary_TarGz(t *testing.T) {
	content := []byte("the binary")
	archive := makeTarGz(t, "bin/mytool", content)

	got, err := extractBinary(archive, "archive.tar.gz", "bin/mytool")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("extractBinary() = %q, want %q", got, content)
	}
}

func TestExtractBinary_TarGz_DotSlashPath(t *testing.T) {
	content := []byte("the binary")
	archive := makeTarGz(t, "./bin/mytool", content)

	// Request path without "./" prefix — should still match.
	got, err := extractBinary(archive, "archive.tar.gz", "bin/mytool")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("extractBinary() = %q, want %q", got, content)
	}
}

func TestExtractBinary_TarGz_NoExtractPath(t *testing.T) {
	archive := makeTarGz(t, "bin/mytool", []byte("data"))
	_, err := extractBinary(archive, "archive.tar.gz", "")
	if err == nil {
		t.Error("extractBinary() expected error for .tar.gz with no extract path, got nil")
	}
}

func TestExtractBinary_TarGz_NotFound(t *testing.T) {
	archive := makeTarGz(t, "bin/othertool", []byte("data"))
	_, err := extractBinary(archive, "archive.tar.gz", "bin/missing")
	if err == nil {
		t.Error("extractBinary() expected error when file not in tar, got nil")
	}
}

func TestExtractBinary_Tgz(t *testing.T) {
	content := []byte("tgz content")
	archive := makeTarGz(t, "mytool", content)

	got, err := extractBinary(archive, "archive.tgz", "mytool")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("extractBinary() = %q, want %q", got, content)
	}
}

func TestExtractBinary_Zip(t *testing.T) {
	content := []byte("zip content")
	archive := makeZip(t, "bin/mytool", content)

	got, err := extractBinary(archive, "archive.zip", "bin/mytool")
	if err != nil {
		t.Fatalf("extractBinary() unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("extractBinary() = %q, want %q", got, content)
	}
}

func TestExtractBinary_Zip_NoExtractPath(t *testing.T) {
	archive := makeZip(t, "mytool", []byte("data"))
	_, err := extractBinary(archive, "archive.zip", "")
	if err == nil {
		t.Error("extractBinary() expected error for .zip with no extract path, got nil")
	}
}

func TestExtractBinary_Zip_NotFound(t *testing.T) {
	archive := makeZip(t, "othertool", []byte("data"))
	_, err := extractBinary(archive, "archive.zip", "missing")
	if err == nil {
		t.Error("extractBinary() expected error when file not in zip, got nil")
	}
}

// --- List ---

func TestList_NoReceipts(t *testing.T) {
	fs := memfs.New()
	cfg := &config.Config{
		Tools: []config.Tool{
			{Name: "tool1", Version: "v1.0.0", Source: "rancher/charts-build-scripts", Mode: config.ModeReleaseChecksums},
		},
	}
	statuses, err := List(fs, cfg, "./bin")
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() len = %d, want 1", len(statuses))
	}
	s := statuses[0]
	if s.Name != "tool1" {
		t.Errorf("List() name = %q, want %q", s.Name, "tool1")
	}
	if s.InstalledVersion != "" {
		t.Errorf("List() InstalledVersion = %q, want empty (no receipt)", s.InstalledVersion)
	}
	if s.IsInstalled() {
		t.Error("List() IsInstalled() = true, want false for tool with no receipt")
	}
}

func TestList_WithReceipt(t *testing.T) {
	fs := memfs.New()
	cfg := &config.Config{
		Tools: []config.Tool{
			{Name: "tool1", Version: "v2.0.0", Source: "rancher/charts-build-scripts", Mode: config.ModeReleaseChecksums},
		},
	}

	// Write a receipt directly so List can find it.
	if err := receipt.Write(fs, "tool1", "v2.0.0", "somechecksum"); err != nil {
		t.Fatal(err)
	}

	statuses, err := List(fs, cfg, "./bin")
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() len = %d, want 1", len(statuses))
	}
	s := statuses[0]
	if s.InstalledVersion != "v2.0.0" {
		t.Errorf("List() InstalledVersion = %q, want %q", s.InstalledVersion, "v2.0.0")
	}
	if !s.IsInstalled() {
		t.Error("List() IsInstalled() = false, want true")
	}
	if !s.IsUpToDate() {
		t.Error("List() IsUpToDate() = false, want true")
	}
}

// --- archive helpers ---

func makeTarGz(t *testing.T, path string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: path, Size: int64(len(content)), Mode: 0755}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, path string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return buf.Bytes()
}
