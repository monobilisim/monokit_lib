package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withLogLocation points the logger at a different destination for one test and
// restores both the config and the package-level Logger afterwards. Every other
// test writes through Logger, so leaving it pointed at a temp file that has
// been deleted would break them.
func withLogLocation(t *testing.T, location string) {
	t.Helper()

	original := GlobalConfig.LogLocation
	GlobalConfig.LogLocation = location

	t.Cleanup(func() {
		GlobalConfig.LogLocation = original
		if _, err := InitLogger(); err != nil {
			t.Errorf("failed to restore the logger: %v", err)
		}
	})
}

func TestInitLoggerWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monokit2.log")
	withLogLocation(t, path)

	logger, err := InitLogger()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logger.Info().Str("check", "disk").Msg("hello from the suite")

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected the log file to exist: %v", err)
	}

	line := string(contents)
	if !strings.Contains(line, "hello from the suite") {
		t.Errorf("message missing from the log: %q", line)
	}
	if !strings.Contains(line, `"check":"disk"`) {
		t.Errorf("structured field missing from the log: %q", line)
	}
	// The log viewer and the future API shipper both filter on the timestamp, so
	// it has to be there.
	if !strings.Contains(line, `"time":`) {
		t.Errorf("timestamp missing from the log: %q", line)
	}
}

func TestInitLoggerAppendsRatherThanTruncates(t *testing.T) {
	// Plugins run once per interval and re-open the log every time. Truncating
	// would throw away the history the log viewer is supposed to read.
	path := filepath.Join(t.TempDir(), "monokit2.log")
	withLogLocation(t, path)

	first, err := InitLogger()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first.Info().Msg("run one")

	second, err := InitLogger()
	if err != nil {
		t.Fatalf("unexpected error on the second init: %v", err)
	}
	second.Info().Msg("run two")

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read the log: %v", err)
	}

	if !strings.Contains(string(contents), "run one") {
		t.Error("the first run's line was truncated by the second InitLogger")
	}
	if !strings.Contains(string(contents), "run two") {
		t.Error("the second run's line is missing")
	}
}

func TestInitLoggerSetsPackageLogger(t *testing.T) {
	// The library logs through the package-level Logger, not the returned value,
	// so InitLogger has to assign it.
	path := filepath.Join(t.TempDir(), "monokit2.log")
	withLogLocation(t, path)

	if _, err := InitLogger(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	Logger.Warn().Msg("written through the package logger")

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read the log: %v", err)
	}
	if !strings.Contains(string(contents), "written through the package logger") {
		t.Errorf("the package-level Logger was not repointed: %q", contents)
	}
}

func TestInitLoggerStdout(t *testing.T) {
	withLogLocation(t, "stdout")

	out := captureStdout(t, func() {
		logger, err := InitLogger()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logger.Info().Msg("to stdout")
	})

	if !strings.Contains(out, "to stdout") {
		t.Errorf("expected the message on stdout, got %q", out)
	}
}

func TestInitLoggerEmptyLocationDefaultsToStdout(t *testing.T) {
	// A global.yml with no log-location must not silently create a file called
	// "" in the working directory.
	withLogLocation(t, "")

	out := captureStdout(t, func() {
		logger, err := InitLogger()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		logger.Info().Msg("defaulted to stdout")
	})

	if !strings.Contains(out, "defaulted to stdout") {
		t.Errorf("expected the message on stdout, got %q", out)
	}
}

func TestInitLoggerUnwritableLocationErrors(t *testing.T) {
	// A typo'd or unwritable path has to surface as an error at startup rather
	// than leave the plugin running with a silently dead logger.
	withLogLocation(t, "/proc/definitely/not/writable/monokit2.log")

	if _, err := InitLogger(); err == nil {
		t.Error("expected an error for an unwritable log location")
	}
}
