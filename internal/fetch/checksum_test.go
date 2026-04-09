package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVerifyReader_Match(t *testing.T) {
	data := []byte("hello world")
	h := sha256.New()
	h.Write(data)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := verifyReader(strings.NewReader(string(data)), expected)
	if err != nil {
		t.Fatalf("verifyReader() unexpected error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("verifyReader() data mismatch")
	}
}

func TestVerifyReader_CaseInsensitive(t *testing.T) {
	data := []byte("test")
	h := sha256.New()
	h.Write(data)
	lower := hex.EncodeToString(h.Sum(nil))
	upper := strings.ToUpper(lower)

	_, err := verifyReader(strings.NewReader("test"), upper)
	if err != nil {
		t.Errorf("verifyReader() should accept uppercase checksum, got error: %v", err)
	}
}

func TestVerifyReader_Mismatch(t *testing.T) {
	_, err := verifyReader(strings.NewReader("hello"), "wrongchecksum")
	if err == nil {
		t.Error("verifyReader() expected error for checksum mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("verifyReader() error = %q, want to contain 'checksum mismatch'", err.Error())
	}
}

func TestSha256Hex(t *testing.T) {
	data := []byte("deterministic")
	h := sha256.New()
	h.Write(data)
	want := hex.EncodeToString(h.Sum(nil))

	got := sha256Hex(data)
	if got != want {
		t.Errorf("sha256Hex() = %q, want %q", got, want)
	}

	// Calling again should return the same result.
	if sha256Hex(data) != got {
		t.Error("sha256Hex() is not deterministic")
	}
}

func TestParseChecksumFile_Found(t *testing.T) {
	content := []byte("abc123  myfile.tar.gz\ndef456  other.tar.gz\n")
	checksum, err := parseChecksumFile(content, "myfile.tar.gz")
	if err != nil {
		t.Fatalf("parseChecksumFile() unexpected error: %v", err)
	}
	if checksum != "abc123" {
		t.Errorf("parseChecksumFile() = %q, want %q", checksum, "abc123")
	}
}

func TestParseChecksumFile_DotSlashPrefix(t *testing.T) {
	content := []byte("abc123  ./myfile.tar.gz\n")
	checksum, err := parseChecksumFile(content, "myfile.tar.gz")
	if err != nil {
		t.Fatalf("parseChecksumFile() unexpected error for ./ prefix: %v", err)
	}
	if checksum != "abc123" {
		t.Errorf("parseChecksumFile() = %q, want %q", checksum, "abc123")
	}
}

func TestParseChecksumFile_StarPrefix(t *testing.T) {
	content := []byte("abc123  *myfile.tar.gz\n")
	checksum, err := parseChecksumFile(content, "myfile.tar.gz")
	if err != nil {
		t.Fatalf("parseChecksumFile() unexpected error for * prefix: %v", err)
	}
	if checksum != "abc123" {
		t.Errorf("parseChecksumFile() = %q, want %q", checksum, "abc123")
	}
}

func TestParseChecksumFile_PathSuffix(t *testing.T) {
	content := []byte("abc123  dist/myfile.tar.gz\n")
	checksum, err := parseChecksumFile(content, "myfile.tar.gz")
	if err != nil {
		t.Fatalf("parseChecksumFile() unexpected error for path suffix match: %v", err)
	}
	if checksum != "abc123" {
		t.Errorf("parseChecksumFile() = %q, want %q", checksum, "abc123")
	}
}

func TestParseChecksumFile_NotFound(t *testing.T) {
	content := []byte("abc123  otherfile.tar.gz\n")
	_, err := parseChecksumFile(content, "missing.tar.gz")
	if err == nil {
		t.Error("parseChecksumFile() expected error for missing entry, got nil")
	}
}

func TestParseChecksumFile_Empty(t *testing.T) {
	_, err := parseChecksumFile([]byte{}, "myfile.tar.gz")
	if err == nil {
		t.Error("parseChecksumFile() expected error for empty file, got nil")
	}
}

func TestParseChecksumFile_SkipsShortLines(t *testing.T) {
	content := []byte("abc123\nfull-hash  myfile.tar.gz\n")
	checksum, err := parseChecksumFile(content, "myfile.tar.gz")
	if err != nil {
		t.Fatalf("parseChecksumFile() unexpected error: %v", err)
	}
	if checksum != "full-hash" {
		t.Errorf("parseChecksumFile() = %q, want %q", checksum, "full-hash")
	}
}
