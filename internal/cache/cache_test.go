package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
)

func TestLatestVersion_Miss(t *testing.T) {
	fs := memfs.New()
	version, hit, err := LatestVersion(fs, "owner", "repo")
	if err != nil {
		t.Fatalf("LatestVersion() unexpected error: %v", err)
	}
	if hit {
		t.Error("LatestVersion() hit = true, want false for empty cache")
	}
	if version != "" {
		t.Errorf("LatestVersion() version = %q, want empty", version)
	}
}

func TestLatestVersion_Hit(t *testing.T) {
	fs := memfs.New()

	if err := WriteLatestVersion(fs, "owner", "repo", "v1.2.3"); err != nil {
		t.Fatalf("WriteLatestVersion() unexpected error: %v", err)
	}

	version, hit, err := LatestVersion(fs, "owner", "repo")
	if err != nil {
		t.Fatalf("LatestVersion() unexpected error: %v", err)
	}
	if !hit {
		t.Error("LatestVersion() hit = false, want true after write")
	}
	if version != "v1.2.3" {
		t.Errorf("LatestVersion() version = %q, want %q", version, "v1.2.3")
	}
}

func TestLatestVersion_Expired(t *testing.T) {
	fs := memfs.New()

	// Write a cache entry with a timestamp 25h in the past.
	if err := fs.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := cacheDir + "/owner-repo"
	f, err := fs.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	oldTS := time.Now().Add(-25 * time.Hour).Unix()
	fmt.Fprintf(f, "%d\nv1.0.0\n", oldTS)
	f.Close()

	version, hit, err := LatestVersion(fs, "owner", "repo")
	if err != nil {
		t.Fatalf("LatestVersion() unexpected error: %v", err)
	}
	if hit {
		t.Error("LatestVersion() hit = true, want false for expired entry")
	}
	if version != "" {
		t.Errorf("LatestVersion() version = %q, want empty for expired entry", version)
	}
}

func TestLatestVersion_Malformed(t *testing.T) {
	fs := memfs.New()

	if err := fs.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := fs.Create(cacheDir + "/owner-repo")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(f, "not-a-timestamp")
	f.Close()

	version, hit, err := LatestVersion(fs, "owner", "repo")
	if err != nil {
		t.Fatalf("LatestVersion() unexpected error: %v", err)
	}
	if hit {
		t.Error("LatestVersion() hit = true, want false for malformed file")
	}
	if version != "" {
		t.Errorf("LatestVersion() version = %q, want empty for malformed file", version)
	}
}

func TestLatestVersion_SkipEnv(t *testing.T) {
	t.Setenv("DEP_FETCH_SKIP_CACHE", "1")

	fs := memfs.New()
	if err := WriteLatestVersion(fs, "owner", "repo", "v9.9.9"); err != nil {
		t.Fatalf("WriteLatestVersion() unexpected error: %v", err)
	}

	version, hit, err := LatestVersion(fs, "owner", "repo")
	if err != nil {
		t.Fatalf("LatestVersion() unexpected error: %v", err)
	}
	if hit {
		t.Error("LatestVersion() hit = true, want false when DEP_FETCH_SKIP_CACHE=1")
	}
	if version != "" {
		t.Errorf("LatestVersion() version = %q, want empty when cache skipped", version)
	}
}

func TestWriteLatestVersion_MultipleOwners(t *testing.T) {
	fs := memfs.New()

	if err := WriteLatestVersion(fs, "ownerA", "repo", "v1.0.0"); err != nil {
		t.Fatalf("WriteLatestVersion() ownerA error: %v", err)
	}
	if err := WriteLatestVersion(fs, "ownerB", "repo", "v2.0.0"); err != nil {
		t.Fatalf("WriteLatestVersion() ownerB error: %v", err)
	}

	v1, hit1, _ := LatestVersion(fs, "ownerA", "repo")
	v2, hit2, _ := LatestVersion(fs, "ownerB", "repo")

	if !hit1 || v1 != "v1.0.0" {
		t.Errorf("ownerA: hit=%v version=%q, want true/v1.0.0", hit1, v1)
	}
	if !hit2 || v2 != "v2.0.0" {
		t.Errorf("ownerB: hit=%v version=%q, want true/v2.0.0", hit2, v2)
	}
}
