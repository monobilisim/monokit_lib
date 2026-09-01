package lib

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// withOsArgs sets os.Args for the duration of a test.
//
// HandleCommonPluginArgs takes an args slice but reads os.Args instead, so the
// only way to drive it is through the global. See
// TestHandleCommonPluginArgsIgnoresItsArgsParameter.
func withOsArgs(t *testing.T, args ...string) {
	t.Helper()
	original := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = original })
}

func TestHandleCommonPluginArgsVersion(t *testing.T) {
	// Every spelling the switch accepts must print the version verbatim, because
	// the updater parses this output to decide whether a plugin needs replacing.
	for _, flag := range []string{"--version", "-v", "version", "v"} {
		t.Run(flag, func(t *testing.T) {
			withOsArgs(t, "osHealth", flag)

			out := captureStdout(t, func() {
				HandleCommonPluginArgs(os.Args, "1.2.3", []string{"os.yml"})
			})

			if out != "1.2.3" {
				t.Errorf("expected %q, got %q", "1.2.3", out)
			}
		})
	}
}

func TestHandleCommonPluginArgsDependencies(t *testing.T) {
	// The runner shells out to `<plugin> -d` and unmarshals the result, so the
	// output has to be bare JSON with no banner or trailing newline noise.
	for _, flag := range []string{"--dependencies", "-d", "dependencies", "d"} {
		t.Run(flag, func(t *testing.T) {
			withOsArgs(t, "osHealth", flag)

			out := captureStdout(t, func() {
				HandleCommonPluginArgs(os.Args, "1.2.3", []string{"os.yml", "global.yml"})
			})

			var deps Dependencies
			if err := json.Unmarshal([]byte(out), &deps); err != nil {
				t.Fatalf("output is not valid JSON (%v): %q", err, out)
			}

			if len(deps.ConfigFiles) != 2 {
				t.Fatalf("expected 2 config files, got %d: %v", len(deps.ConfigFiles), deps.ConfigFiles)
			}
			if deps.ConfigFiles[0] != "os.yml" || deps.ConfigFiles[1] != "global.yml" {
				t.Errorf("unexpected config files: %v", deps.ConfigFiles)
			}
		})
	}
}

func TestHandleCommonPluginArgsEmptyDependencies(t *testing.T) {
	// A plugin with no config files must still emit a decodable object; the
	// runner would otherwise treat the empty output as a probe failure.
	withOsArgs(t, "monokit2", "-d")

	out := captureStdout(t, func() {
		HandleCommonPluginArgs(os.Args, "devel", []string{})
	})

	var deps Dependencies
	if err := json.Unmarshal([]byte(out), &deps); err != nil {
		t.Fatalf("output is not valid JSON (%v): %q", err, out)
	}
	if len(deps.ConfigFiles) != 0 {
		t.Errorf("expected no config files, got %v", deps.ConfigFiles)
	}
}

func TestHandleCommonPluginArgsUnknownArgPrintsNothing(t *testing.T) {
	// An unrecognised first argument has to fall through silently: the plugin's
	// own main() handles it afterwards, and stray output here would corrupt the
	// version/dependency probes.
	withOsArgs(t, "osHealth", "--login")

	out := captureStdout(t, func() {
		HandleCommonPluginArgs(os.Args, "1.2.3", []string{"os.yml"})
	})

	if out != "" {
		t.Errorf("expected no output for an unknown argument, got %q", out)
	}
}

func TestHandleCommonPluginArgsNoArgsPrintsNothing(t *testing.T) {
	withOsArgs(t, "osHealth")

	out := captureStdout(t, func() {
		HandleCommonPluginArgs(os.Args, "1.2.3", []string{"os.yml"})
	})

	if out != "" {
		t.Errorf("expected no output when there are no arguments, got %q", out)
	}
}

// TestHandleCommonPluginArgsIgnoresItsArgsParameter pins down a real bug rather
// than asserting the behaviour is correct.
//
// The function signature promises it acts on the args slice it is handed, but
// the body switches on os.Args[1]. Today every caller passes os.Args, so the
// two agree and nothing breaks. The moment a caller passes a filtered slice —
// which is exactly what monokit2's arg forwarding does elsewhere — it will read
// the wrong argument.
//
// When the function is fixed to read its parameter, this test will fail and
// should be replaced with one asserting the parameter wins.
func TestHandleCommonPluginArgsIgnoresItsArgsParameter(t *testing.T) {
	withOsArgs(t, "osHealth", "--version")

	// The parameter asks for the dependency probe; os.Args asks for the version.
	out := captureStdout(t, func() {
		HandleCommonPluginArgs([]string{"osHealth", "-d"}, "1.2.3", []string{"os.yml"})
	})

	if strings.HasPrefix(out, "{") {
		t.Fatalf("the args parameter is now honoured — good, but update this test: got %q", out)
	}
	if out != "1.2.3" {
		t.Fatalf("expected os.Args to win with %q, got %q", "1.2.3", out)
	}
}
