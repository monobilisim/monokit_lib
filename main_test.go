package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain bootstraps the suite the same way a plugin's main() does:
// InitConfig, InitLogger, InitializeDatabase. Every other test file assumes
// GlobalConfig, Logger and DB are live, so this is the one place that sets
// them up.
//
// The tests only ever run inside the Podman image (see Containerfile), which
// provides /etc/mono/global.yml and a throwaway root filesystem. Running them
// on a host would scribble on the host's real /etc/mono, so the bootstrap
// refuses to continue unless the fixture is the one the container installs.
func TestMain(m *testing.M) {
	// Makes IsTestMode() true so the alarm senders take their no-network path.
	os.Setenv("TEST", "true")

	if _, err := os.Stat("/etc/mono/global.yml"); err != nil {
		fmt.Fprintln(os.Stderr, "/etc/mono/global.yml is missing — run the suite with `just test`, not `go test`")
		os.Exit(1)
	}

	if err := InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "InitConfig: %v\n", err)
		os.Exit(1)
	}

	if GlobalConfig.ProjectIdentifier != "test" {
		fmt.Fprintf(os.Stderr,
			"refusing to run: /etc/mono/global.yml has project-identifier %q, expected the test fixture's %q\n",
			GlobalConfig.ProjectIdentifier, "test")
		os.Exit(1)
	}

	if _, err := InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "InitLogger: %v\n", err)
		os.Exit(1)
	}

	if err := InitializeDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "InitializeDatabase: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// --- helpers shared by the other test files ---

// writeMonoConfig drops a file into /etc/mono for the duration of a test and
// removes it afterwards, restoring whatever was there before. InitConfig reads
// /etc/mono unconditionally, so a test that needs a plugin config has to put it
// on disk rather than inject it.
func writeMonoConfig(t *testing.T, name, contents string) {
	t.Helper()

	path := filepath.Join("/etc/mono", name)

	previous, err := os.ReadFile(path)
	if err != nil {
		previous = nil // nothing there before, so cleanup deletes rather than restores
	}

	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}

	t.Cleanup(func() {
		if previous != nil {
			if err := os.WriteFile(path, previous, 0644); err != nil {
				t.Errorf("failed to restore %s: %v", path, err)
			}
			return
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("failed to remove %s: %v", path, err)
		}
	})
}

// removeMonoConfig takes a file out of /etc/mono for the duration of a test,
// putting it back afterwards. Used to exercise the missing-config error paths.
func removeMonoConfig(t *testing.T, name string) {
	t.Helper()

	path := filepath.Join("/etc/mono", name)

	previous, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("failed to read %s: %v", path, err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove %s: %v", path, err)
	}

	t.Cleanup(func() {
		if err := os.WriteFile(path, previous, 0644); err != nil {
			t.Errorf("failed to restore %s: %v", path, err)
		}
	})
}

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns what fn
// printed. The CLI helpers write straight to stdout, so this is the only way
// to assert on them.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf []byte
		chunk := make([]byte, 4096)
		for {
			n, err := r.Read(chunk)
			if n > 0 {
				buf = append(buf, chunk[:n]...)
			}
			if err != nil {
				break
			}
		}
		done <- string(buf)
	}()

	fn()

	os.Stdout = original
	w.Close()
	out := <-done
	r.Close()

	return out
}

// TestBootstrapIsLive is a canary: if the shared fixtures ever stop being set
// up, every other test fails in a confusing way. This one fails clearly.
func TestBootstrapIsLive(t *testing.T) {
	if DB == nil {
		t.Fatal("DB is nil, TestMain did not initialise the database")
	}
	if GlobalConfig.SqliteLocation == "" {
		t.Error("GlobalConfig.SqliteLocation is empty, global.yml was not loaded")
	}
	if !IsTestMode() {
		t.Error("IsTestMode() is false, the suite would try to reach Zulip and Redmine")
	}
}
