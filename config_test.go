package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitConfigLoadsGlobal(t *testing.T) {
	// TestMain already called InitConfig; assert it actually populated the struct
	// rather than leaving zero values, which is how a yaml tag typo shows up.
	if GlobalConfig.ProjectIdentifier != "test" {
		t.Errorf("ProjectIdentifier = %q, want %q", GlobalConfig.ProjectIdentifier, "test")
	}
	if GlobalConfig.Hostname != "test" {
		t.Errorf("Hostname = %q, want %q", GlobalConfig.Hostname, "test")
	}
	if GlobalConfig.LogLocation == "" {
		t.Error("LogLocation is empty")
	}
	if GlobalConfig.SqliteLocation == "" {
		t.Error("SqliteLocation is empty")
	}
	if GlobalConfig.PluginsLocation == "" {
		t.Error("PluginsLocation is empty")
	}

	// Nested keys use their own yaml tags, and a mismatch there disables
	// throttling silently rather than erroring.
	if !GlobalConfig.ZulipAlarm.Enabled {
		t.Error("ZulipAlarm.Enabled is false, expected the fixture's true")
	}
	if GlobalConfig.ZulipAlarm.Interval == 0 {
		t.Error("ZulipAlarm.Interval is 0, so the interval throttle would never engage")
	}
	if GlobalConfig.ZulipAlarm.Limit == 0 {
		t.Error("ZulipAlarm.Limit is 0, so the limit throttle would block every alarm")
	}
	if len(GlobalConfig.ZulipAlarm.WebhookUrls) == 0 {
		t.Error("ZulipAlarm.WebhookUrls is empty")
	}
	if GlobalConfig.Redmine.Interval == 0 {
		t.Error("Redmine.Interval is 0")
	}
}

func TestInitConfigDerivesDirsFromLocations(t *testing.T) {
	// LogDir and DbDir are string-split off the file locations, so a trailing
	// slash or a bare filename would silently produce "" and drop state into the
	// process's working directory.
	if want := filepath.Dir(GlobalConfig.LogLocation); LogDir != want {
		t.Errorf("LogDir = %q, want %q", LogDir, want)
	}
	if want := filepath.Dir(GlobalConfig.SqliteLocation); DbDir != want {
		t.Errorf("DbDir = %q, want %q", DbDir, want)
	}
	if PluginsDir != GlobalConfig.PluginsLocation {
		t.Errorf("PluginsDir = %q, want %q", PluginsDir, GlobalConfig.PluginsLocation)
	}
}

func TestInitConfigMissingGlobalErrors(t *testing.T) {
	// Restoring the file and re-running InitConfig has to happen inside this
	// function, not in a t.Cleanup, because later cleanups run first and every
	// following test needs the config back in place.
	path := "/etc/mono/global.yml"

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read the fixture: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove the fixture: %v", err)
	}
	defer func() {
		if err := os.WriteFile(path, saved, 0644); err != nil {
			t.Fatalf("failed to restore the fixture: %v", err)
		}
		if err := InitConfig(); err != nil {
			t.Fatalf("failed to re-initialise the config: %v", err)
		}
	}()

	err = InitConfig()
	if err == nil {
		t.Fatal("expected an error when global.yml is missing")
	}
	if !strings.Contains(err.Error(), "global configuration file does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitConfigLoadsPluginConfig(t *testing.T) {
	writeMonoConfig(t, "os.yml", `
disk-usage-alarm:
  enabled: true
  limit: 85
ram-usage-alarm:
  enabled: true
  limit: 90
  top-processes:
    enabled: true
    processes: 5
system-load-alarm:
  enabled: false
  limit-multiplier: 1.5
`)

	if err := InitConfig("os.yml"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !OsHealthConfig.DiskUsageAlarm.Enabled {
		t.Error("DiskUsageAlarm.Enabled did not load")
	}
	if OsHealthConfig.DiskUsageAlarm.Limit != 85 {
		t.Errorf("DiskUsageAlarm.Limit = %d, want 85", OsHealthConfig.DiskUsageAlarm.Limit)
	}
	if OsHealthConfig.RamUsageAlarm.TopProcesses.Processes != 5 {
		t.Errorf("RamUsageAlarm.TopProcesses.Processes = %d, want 5",
			OsHealthConfig.RamUsageAlarm.TopProcesses.Processes)
	}
	if OsHealthConfig.SystemLoadAlarm.LimitMultiplier != 1.5 {
		t.Errorf("SystemLoadAlarm.LimitMultiplier = %v, want 1.5",
			OsHealthConfig.SystemLoadAlarm.LimitMultiplier)
	}
}

func TestInitConfigMissingPluginConfigErrors(t *testing.T) {
	removeMonoConfig(t, "os.yml") // no-op when it is not there

	err := InitConfig("os.yml")
	if err == nil {
		t.Fatal("expected an error when os.yml is missing")
	}
	if !strings.Contains(err.Error(), "os configuration file does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitConfigMalformedPluginConfigErrors(t *testing.T) {
	// A truncated or hand-edited config must fail loudly. Falling through with
	// zero values would silently disable every check in the plugin.
	writeMonoConfig(t, "os.yml", "disk-usage-alarm:\n\tenabled: [unclosed\n")

	err := InitConfig("os.yml")
	if err == nil {
		t.Fatal("expected a parse error for malformed yaml")
	}
	if !strings.Contains(err.Error(), "failed to parse os configuration file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInitConfigUnknownConfigFileIsSilentlyIgnored records a sharp edge rather
// than approving of it: InitConfig's switch has no default case, so a config
// file it does not know about is skipped without a word. A plugin that declares
// `wppconnect.yml` today gets no config and no error — it just sees an empty
// struct. Worth fixing; until then, this is the documented behaviour.
func TestInitConfigUnknownConfigFileIsSilentlyIgnored(t *testing.T) {
	if err := InitConfig("definitely-not-wired.yml"); err != nil {
		t.Fatalf("expected an unwired config name to be ignored, got %v", err)
	}
}

// --- CheckPluginDependencies ---

// fakePlugin writes an executable stub into dir that answers the `-d` probe with
// the given JSON, imitating a real plugin's HandleCommonPluginArgs output.
func fakePlugin(t *testing.T, dir, name, dependencyJSON string) {
	t.Helper()

	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"-d\" ]; then printf '%%s' '%s'; fi\n", dependencyJSON)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write the fake plugin: %v", err)
	}
}

// withPluginsDir points PluginsDir at a throwaway directory for one test.
func withPluginsDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	original := PluginsDir
	PluginsDir = dir
	t.Cleanup(func() { PluginsDir = original })

	return dir
}

func TestCheckPluginDependenciesReportsMissingConfig(t *testing.T) {
	dir := withPluginsDir(t)
	fakePlugin(t, dir, "ghostHealth", `{"configFiles":["ghost.yml"]}`)

	missing := CheckPluginDependencies()

	if len(missing) != 1 {
		t.Fatalf("expected one missing dependency, got %d: %v", len(missing), missing)
	}
	if !strings.Contains(missing[0], "ghost.yml") {
		t.Errorf("expected the message to name ghost.yml, got %q", missing[0])
	}
	// The requester matters: with a dozen plugins installed, a bare "config
	// missing" line does not say who needs it.
	if !strings.Contains(missing[0], "ghostHealth") {
		t.Errorf("expected the message to name the requesting plugin, got %q", missing[0])
	}
}

func TestCheckPluginDependenciesSatisfied(t *testing.T) {
	dir := withPluginsDir(t)
	fakePlugin(t, dir, "presentHealth", `{"configFiles":["present.yml"]}`)
	writeMonoConfig(t, "present.yml", "enabled: true\n")

	if missing := CheckPluginDependencies(); len(missing) != 0 {
		t.Errorf("expected no missing dependencies, got %v", missing)
	}
}

func TestCheckPluginDependenciesSkipsNonExecutables(t *testing.T) {
	// The plugins directory also holds leftovers: backups from the updater and
	// the odd README. Probing those would produce noise or, worse, hang.
	dir := withPluginsDir(t)

	if err := os.WriteFile(filepath.Join(dir, "osHealth.backup"), []byte("not a binary"), 0644); err != nil {
		t.Fatalf("failed to write the non-executable file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "somedir"), 0755); err != nil {
		t.Fatalf("failed to create the subdirectory: %v", err)
	}

	if missing := CheckPluginDependencies(); len(missing) != 0 {
		t.Errorf("expected non-executables and directories to be skipped, got %v", missing)
	}
}

func TestCheckPluginDependenciesSkipsUnparseableProbeOutput(t *testing.T) {
	// A plugin from a mismatched release could print a banner or an error. The
	// probe must skip it rather than report a bogus missing dependency.
	dir := withPluginsDir(t)
	fakePlugin(t, dir, "chattyHealth", `starting up, please wait`)

	if missing := CheckPluginDependencies(); len(missing) != 0 {
		t.Errorf("expected unparseable probe output to be skipped, got %v", missing)
	}
}

func TestCheckPluginDependenciesCreatesMissingPluginsDir(t *testing.T) {
	original := PluginsDir
	PluginsDir = filepath.Join(t.TempDir(), "plugins")
	t.Cleanup(func() { PluginsDir = original })

	if missing := CheckPluginDependencies(); len(missing) != 0 {
		t.Errorf("expected an empty result for a fresh install, got %v", missing)
	}

	// A fresh install has no plugins directory yet; the probe is what creates it,
	// so the TUI has somewhere to install into.
	info, err := os.Stat(PluginsDir)
	if err != nil {
		t.Fatalf("expected the plugins directory to be created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}
