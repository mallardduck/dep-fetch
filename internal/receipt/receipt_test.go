package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
)

func TestRead_Missing(t *testing.T) {
	fs := memfs.New()
	r, err := Read(fs, "mytool")
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	if r.Version != "" || r.Checksum != "" {
		t.Errorf("Read() = %+v, want zero Receipt for missing file", r)
	}
}

func TestRead_Malformed(t *testing.T) {
	fs := memfs.New()
	if err := fs.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, _ := fs.Create(stateDir + "/mytool.receipt")
	fmt.Fprint(f, "only-one-line")
	f.Close()

	r, err := Read(fs, "mytool")
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	if r.Version != "" || r.Checksum != "" {
		t.Errorf("Read() = %+v, want zero Receipt for malformed file", r)
	}
}

func TestWriteRead_RoundTrip(t *testing.T) {
	fs := memfs.New()

	if err := Write(fs, "mytool", "v1.2.3", "abc123checksum"); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}

	r, err := Read(fs, "mytool")
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	if r.Version != "v1.2.3" {
		t.Errorf("Read() Version = %q, want %q", r.Version, "v1.2.3")
	}
	if r.Checksum != "abc123checksum" {
		t.Errorf("Read() Checksum = %q, want %q", r.Checksum, "abc123checksum")
	}
}

func TestVerify_MissingReceipt(t *testing.T) {
	fs := memfs.New()
	ok, err := Verify(fs, "./bin", "mytool", "v1.0.0")
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if ok {
		t.Error("Verify() = true, want false for missing receipt")
	}
}

func TestVerify_VersionMismatch(t *testing.T) {
	fs := memfs.New()

	if err := Write(fs, "mytool", "v1.0.0", "somechecksum"); err != nil {
		t.Fatal(err)
	}

	ok, err := Verify(fs, "./bin", "mytool", "v2.0.0")
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if ok {
		t.Error("Verify() = true, want false for version mismatch")
	}
}

func TestVerify_BinaryMissing(t *testing.T) {
	fs := memfs.New()

	// Receipt exists with correct version but binary doesn't.
	if err := Write(fs, "mytool", "v1.0.0", "somechecksum"); err != nil {
		t.Fatal(err)
	}

	ok, err := Verify(fs, "./bin", "mytool", "v1.0.0")
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if ok {
		t.Error("Verify() = true, want false when binary is missing")
	}
}

func TestVerify_OK(t *testing.T) {
	fs := memfs.New()
	binDir := "./bin"

	// Write binary content to bin dir.
	binContent := []byte("fake binary content")
	if err := fs.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := fs.Create(binDir + "/mytool")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(binContent)
	f.Close()

	// Compute correct checksum.
	h := sha256.New()
	h.Write(binContent)
	checksum := hex.EncodeToString(h.Sum(nil))

	if err := Write(fs, "mytool", "v1.0.0", checksum); err != nil {
		t.Fatal(err)
	}

	ok, err := Verify(fs, binDir, "mytool", "v1.0.0")
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if !ok {
		t.Error("Verify() = false, want true for correct binary and receipt")
	}
}

func TestVerify_Tampered(t *testing.T) {
	fs := memfs.New()
	binDir := "./bin"

	// Write binary.
	if err := fs.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	f, _ := fs.Create(binDir + "/mytool")
	f.Write([]byte("original content"))
	f.Close()

	// Write receipt with wrong checksum (simulates tampering).
	if err := Write(fs, "mytool", "v1.0.0", "wrongchecksum"); err != nil {
		t.Fatal(err)
	}

	ok, err := Verify(fs, binDir, "mytool", "v1.0.0")
	if err == nil {
		t.Error("Verify() expected error for tampered binary, got nil")
	}
	if ok {
		t.Error("Verify() = true, want false for tampered binary")
	}
}

func TestManager(t *testing.T) {
	fs := memfs.New()
	binDir := "./bin"
	m := NewManager(fs, binDir)

	// Write a receipt via manager.
	if err := m.Write("tool", "v1.0.0", "abc"); err != nil {
		t.Fatalf("Manager.Write() unexpected error: %v", err)
	}

	// Read it back.
	r, err := m.Read("tool")
	if err != nil {
		t.Fatalf("Manager.Read() unexpected error: %v", err)
	}
	if r.Version != "v1.0.0" || r.Checksum != "abc" {
		t.Errorf("Manager.Read() = %+v, want {v1.0.0, abc}", r)
	}

	// Verify with missing binary → false, nil.
	ok, err := m.Verify("tool", "v1.0.0")
	if err != nil {
		t.Fatalf("Manager.Verify() unexpected error: %v", err)
	}
	if ok {
		t.Error("Manager.Verify() = true, want false when binary is missing")
	}
}
