package lib

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPluginDisplayName(t *testing.T) {
	// Release artifacts are named <plugin>_<version>_<os>_<arch>, and the TUI has
	// to show just the plugin name.
	tests := []struct {
		artifact string
		expected string
	}{
		{"osHealth_devel_linux_amd64", "osHealth"},
		{"osHealth_1.2.3_linux_arm64", "osHealth"},
		{"osHealth_devel_windows_amd64.exe", "osHealth"},
		{"osHealth.exe", "osHealth"},
		{"osHealth", "osHealth"},
		{"zimbraLdap_devel_freebsd_amd64", "zimbraLdap"},
	}

	for _, tt := range tests {
		if got := extractPluginDisplayName(tt.artifact); got != tt.expected {
			t.Errorf("extractPluginDisplayName(%q) = %q, want %q", tt.artifact, got, tt.expected)
		}
	}
}

func TestIsPluginForCurrentPlatform(t *testing.T) {
	// Installing an arm64 binary on an amd64 host produces an exec format error
	// at run time, long after the install looked successful — so this filter is
	// the only thing standing between the user and a confusing failure.
	tests := []struct {
		artifact string
		os, arch string
		expected bool
		reason   string
	}{
		{"osHealth_devel_linux_amd64", "linux", "amd64", true, "exact match"},
		{"osHealth_devel_linux_arm64", "linux", "amd64", false, "wrong arch"},
		{"osHealth_devel_darwin_amd64", "linux", "amd64", false, "wrong os"},
		{"osHealth_devel_windows_amd64.exe", "windows", "amd64", true, ".exe suffix is stripped first"},
		{"osHealth_devel_windows_amd64.exe", "linux", "amd64", false, "windows binary on linux"},
		{"osHealth_devel_freebsd_amd64", "freebsd", "amd64", true, "freebsd is a release target"},
		{"osHealth", "linux", "amd64", true, "no platform suffix counts as generic"},
		{"osHealth_devel", "linux", "amd64", true, "a version but no platform suffix is generic"},
	}

	for _, tt := range tests {
		got := isPluginForCurrentPlatform(tt.artifact, tt.os, tt.arch)
		if got != tt.expected {
			t.Errorf("%s: isPluginForCurrentPlatform(%q, %q, %q) = %v, want %v",
				tt.reason, tt.artifact, tt.os, tt.arch, got, tt.expected)
		}
	}
}

func TestCalculateFileHash(t *testing.T) {
	contents := "monokit plugin binary contents"
	path := filepath.Join(t.TempDir(), "plugin")

	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	sum := sha256.Sum256([]byte(contents))
	expected := hex.EncodeToString(sum[:])

	if got := calculateFileHash(path); got != expected {
		t.Errorf("calculateFileHash = %q, want %q", got, expected)
	}
}

func TestCalculateFileHashEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	// The empty-file digest is a real hash, not an error — the callers use ""
	// to mean "could not hash", so these must not collide.
	sum := sha256.Sum256(nil)
	if got := calculateFileHash(path); got != hex.EncodeToString(sum[:]) {
		t.Errorf("expected the empty-string digest, got %q", got)
	}
}

func TestCalculateFileHashUnreadableFile(t *testing.T) {
	// An empty string is the signal isChecksumMatch and verifyChecksum key off,
	// so an unreadable file must not return a plausible-looking digest.
	if got := calculateFileHash(filepath.Join(t.TempDir(), "absent")); got != "" {
		t.Errorf("expected an empty hash for a missing file, got %q", got)
	}
}
