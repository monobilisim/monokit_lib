package lib

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		version             string
		major, minor, patch int
		isDevel             bool
		description         string
	}{
		{"1.2.3", 1, 2, 3, false, "plain semver"},
		{"v1.2.3", 1, 2, 3, false, "release tags carry a v prefix"},
		{"  1.2.3  ", 1, 2, 3, false, "a binary's -v output can have whitespace"},
		{"10.20.30", 10, 20, 30, false, "multi-digit components"},
		{"1.2.3-rc1", 1, 2, 3, false, "prerelease suffix is ignored"},
		{"devel", 0, 0, 0, true, "devel builds"},
		{"v1.2.3-devel", 0, 0, 0, true, "devel wins over the numbers"},
		{"1.2", -1, -1, -1, false, "two components is not semver"},
		{"garbage", -1, -1, -1, false, "unparseable"},
		{"", -1, -1, -1, false, "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, patch, isDevel := parseSemver(tt.version)

			if major != tt.major || minor != tt.minor || patch != tt.patch || isDevel != tt.isDevel {
				t.Errorf("%s: parseSemver(%q) = (%d, %d, %d, %v), want (%d, %d, %d, %v)",
					tt.description, tt.version,
					major, minor, patch, isDevel,
					tt.major, tt.minor, tt.patch, tt.isDevel)
			}
		})
	}
}

func TestIsMajorVersionChange(t *testing.T) {
	// This is the guard that stops an unattended auto-update from crossing a
	// breaking release, so both the false negatives and the false positives
	// matter.
	tests := []struct {
		old, new string
		expected bool
		reason   string
	}{
		{"1.0.0", "2.0.0", true, "a major bump must be refused"},
		{"1.9.9", "2.0.0", true, "a major bump from a late minor must be refused"},
		{"1.0.0", "1.5.0", false, "a minor bump is allowed"},
		{"1.0.0", "1.0.1", false, "a patch bump is allowed"},
		{"2.0.0", "1.0.0", false, "a downgrade is not a major change"},
		{"2.0.0", "2.0.0", false, "the same version is not a change"},
		{"devel", "2.0.0", false, "a devel build can always move"},
		{"1.0.0", "devel", false, "moving to devel is always allowed"},
		{"garbage", "2.0.0", false, "an unparseable current version must not block updates"},
		{"1.0.0", "garbage", false, "an unparseable target version must not block updates"},
	}

	for _, tt := range tests {
		if got := isMajorVersionChange(tt.old, tt.new); got != tt.expected {
			t.Errorf("%s: isMajorVersionChange(%q, %q) = %v, want %v",
				tt.reason, tt.old, tt.new, got, tt.expected)
		}
	}
}

func TestIsVersionMatch(t *testing.T) {
	tests := []struct {
		current, target string
		expected        bool
		reason          string
	}{
		{"1.2.3", "1.2.3", true, "identical"},
		{"v1.2.3", "1.2.3", true, "the v prefix is not significant"},
		{"1.2.3", "v1.2.3", true, "the v prefix is not significant either way"},
		{"  1.2.3  ", "1.2.3", true, "whitespace from a binary's -v output"},
		{"1.2.3", "1.2.4", false, "different patch"},
		{"devel", "devel", false, "two devel builds need a checksum comparison, not a version one"},
		{"DEVEL", "1.2.3", false, "devel detection is case insensitive"},
		{"1.2.3", "1.2.3-devel", false, "a devel target needs a checksum comparison"},
	}

	for _, tt := range tests {
		if got := isVersionMatch(tt.current, tt.target); got != tt.expected {
			t.Errorf("%s: isVersionMatch(%q, %q) = %v, want %v",
				tt.reason, tt.current, tt.target, got, tt.expected)
		}
	}
}

// writeTempFile creates a file with the given contents and returns its path
// plus the hex SHA256 of those contents.
func writeTempFile(t *testing.T, contents string) (path, hash string) {
	t.Helper()

	path = filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatalf("failed to write the temp file: %v", err)
	}

	sum := sha256.Sum256([]byte(contents))
	return path, hex.EncodeToString(sum[:])
}

func TestIsChecksumMatch(t *testing.T) {
	path, hash := writeTempFile(t, "monokit plugin binary")

	t.Run("matching hash", func(t *testing.T) {
		match, canVerify := isChecksumMatch(path, hash)
		if !match || !canVerify {
			t.Errorf("expected (true, true), got (%v, %v)", match, canVerify)
		}
	})

	t.Run("hash comparison is case insensitive", func(t *testing.T) {
		// GitHub's checksums.txt has been seen with both cases.
		match, canVerify := isChecksumMatch(path, strings.ToUpper(hash))
		if !match || !canVerify {
			t.Errorf("expected (true, true) for an uppercase hash, got (%v, %v)", match, canVerify)
		}
	})

	t.Run("mismatching hash", func(t *testing.T) {
		match, canVerify := isChecksumMatch(path, strings.Repeat("a", 64))
		if match {
			t.Error("expected no match for a wrong hash")
		}
		if !canVerify {
			t.Error("expected canVerify to be true when an expected hash was supplied")
		}
	})

	t.Run("no expected hash", func(t *testing.T) {
		// Without a remote hash the updater must fall back to a version compare
		// rather than assume the local file is current.
		match, canVerify := isChecksumMatch(path, "")
		if match || canVerify {
			t.Errorf("expected (false, false), got (%v, %v)", match, canVerify)
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		match, canVerify := isChecksumMatch(filepath.Join(t.TempDir(), "absent"), hash)
		if match {
			t.Error("expected no match for a file that cannot be read")
		}
		if !canVerify {
			t.Error("expected canVerify to stay true: the expected hash exists, the file does not")
		}
	})
}

func TestVerifyChecksum(t *testing.T) {
	path, hash := writeTempFile(t, "downloaded artifact")

	if err := verifyChecksum(path, hash); err != nil {
		t.Errorf("expected a matching checksum to verify, got %v", err)
	}

	if err := verifyChecksum(path, strings.ToUpper(hash)); err != nil {
		t.Errorf("expected an uppercase checksum to verify, got %v", err)
	}

	err := verifyChecksum(path, strings.Repeat("b", 64))
	if err == nil {
		t.Error("expected a mismatching checksum to fail")
	} else if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected a mismatch error, got %v", err)
	}

	err = verifyChecksum(path, "")
	if err == nil {
		t.Error("expected an empty checksum to fail rather than pass silently")
	}

	err = verifyChecksum(filepath.Join(t.TempDir(), "absent"), hash)
	if err == nil {
		t.Error("expected an unreadable file to fail verification")
	}
}

func TestEnsureTmpDir(t *testing.T) {
	base := t.TempDir()

	tmpDir, err := ensureTmpDir(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpDir != filepath.Join(base, "tmp") {
		t.Errorf("expected %s, got %s", filepath.Join(base, "tmp"), tmpDir)
	}

	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("the directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}

	// Called on every update, so a pre-existing directory must not be an error.
	if _, err := ensureTmpDir(base); err != nil {
		t.Errorf("expected a second call to succeed, got %v", err)
	}
}

func TestBackupRestoreCleanupRoundTrip(t *testing.T) {
	// This is the rollback path: if it does not work, a failed update leaves the
	// host with a broken binary and no way back.
	dir := t.TempDir()
	original := filepath.Join(dir, "osHealth")

	if err := os.WriteFile(original, []byte("version one"), 0755); err != nil {
		t.Fatalf("failed to seed the original file: %v", err)
	}

	tmpDir, err := ensureTmpDir(dir)
	if err != nil {
		t.Fatalf("ensureTmpDir failed: %v", err)
	}

	backupPath, err := backupFile(original, tmpDir)
	if err != nil {
		t.Fatalf("backupFile failed: %v", err)
	}
	if backupPath != filepath.Join(tmpDir, "osHealth.backup") {
		t.Errorf("unexpected backup path: %s", backupPath)
	}

	// The restored binary has to stay executable, or the plugin cannot run.
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("failed to stat the backup: %v", err)
	}
	if backupInfo.Mode().Perm()&0111 == 0 {
		t.Errorf("backup lost its executable bit: %v", backupInfo.Mode().Perm())
	}

	// Simulate a botched update overwriting the binary.
	if err := os.WriteFile(original, []byte("broken"), 0644); err != nil {
		t.Fatalf("failed to overwrite the original: %v", err)
	}

	if err := restoreFile(backupPath, original); err != nil {
		t.Fatalf("restoreFile failed: %v", err)
	}

	restored, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("failed to read the restored file: %v", err)
	}
	if string(restored) != "version one" {
		t.Errorf("expected the original contents back, got %q", restored)
	}

	restoredInfo, err := os.Stat(original)
	if err != nil {
		t.Fatalf("failed to stat the restored file: %v", err)
	}
	if restoredInfo.Mode().Perm()&0111 == 0 {
		t.Errorf("restored file is not executable: %v", restoredInfo.Mode().Perm())
	}

	if err := cleanupBackup(backupPath); err != nil {
		t.Fatalf("cleanupBackup failed: %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("expected the backup to be gone after cleanup")
	}
}

func TestBackupFileMissingSource(t *testing.T) {
	tmpDir := t.TempDir()

	if _, err := backupFile(filepath.Join(tmpDir, "absent"), tmpDir); err == nil {
		t.Error("expected backing up a missing file to fail")
	}
}

func TestRestoreFileMissingBackup(t *testing.T) {
	dir := t.TempDir()

	if err := restoreFile(filepath.Join(dir, "absent.backup"), filepath.Join(dir, "target")); err == nil {
		t.Error("expected restoring from a missing backup to fail")
	}
}
