# monokit_lib Roadmap

Shared library every monokit2 plugin links against. Feature parity with
monokit1 `common/` (alarm, config, display, service, proc, file, http, apilog,
redmine) plus the helpers plugins currently reimplement.

Alarm / Redmine / config / logging *feature* parity is tracked in the core
[../ROADMAP.md](../ROADMAP.md) so it stays in one place. This file tracks
lib's own gaps: testing, CI, and the shared surface plugins keep duplicating.

## Testing & CI

lib is compiled into every plugin, so a regression here breaks the whole fleet
at once. It used to be the only module in the project with no gate at all.

Run the suite with `just test` — never `go test` on a host. The tests write to
`/etc/mono`, `/var/log` and `/var/lib`, so they run in a throwaway Podman
container; `TestMain` refuses to start unless it finds the container's fixture.

- [X] `.github/workflows/ci.yml` — lint / build / test on every push and PR
      (Go 1.25, `just`, Podman, coverage artifact)
- [X] `justfile` with `build`, `build-all`, `vet`, `fmt-check`, `test`, `check`, `clean`
- [X] `Containerfile` test harness — plain `golang:1.25-trixie`, one config
      fixture, `go test` as the entrypoint so its exit code is the container's.
      No systemd and no service fixtures: these are mock tests against a real
      sqlite file, not integration tests.
  - Unlike the plugin harness, the test transcript is kept in its own file rather
    than teed into `global.yml`'s `log-location`. That path is also open in
    append mode by the library's own logger, and two writers at two offsets in
    one file overwrite each other — so `monokit2.log` here is purely what the
    library logged.
- [X] Cross-compile gate: linux/amd64, linux/arm64, windows/amd64, freebsd/amd64.
      `cmd/monokit_lib` is a smoke binary that exists only to be linked, which is
      what actually exercises the `shutdown_notifier_linux.go` / `_stub.go`
      build-tag split — a test only ever builds for the platform it runs on.
- [X] `go vet` + `gofmt -l` as failing steps
  - [ ] `staticcheck` as well
- [X] Unit suite for the pure functions: `parseSemver`, `isMajorVersionChange`,
      `isVersionMatch`, `isChecksumMatch`, `verifyChecksum`, `calculateFileHash`,
      `CreateMarkdownTable`, `extractPluginDisplayName`,
      `isPluginForCurrentPlatform`, `RedmineDate` marshal/unmarshal/Value/Scan,
      `RedmineAPIObject` Value/Scan
- [X] Config suite: global load, derived `LogDir`/`DbDir`/`PluginsDir`, plugin
      config load, missing-file and malformed-YAML error paths,
      `CheckPluginDependencies` (missing / satisfied / non-executable /
      unparseable probe output / fresh install)
- [X] Logger suite: file output, append-not-truncate across runs, package-level
      `Logger` assignment, stdout and empty-location defaults, unwritable path
- [X] Schema suite: every model is migrated, idempotent re-init, migration from an
      older schema (dropped column restored), `Issue` API-object round trip
- [X] Alarm suite: limit throttling, recovery allowed after the down budget is
      spent, newest-first ordering, `Limit`-deep window, service/module scoping
- [X] Updater suite: checksum verify, backup/restore/cleanup round trip including
      the executable bit, `ensureTmpDir`, missing source/backup error paths
- [X] Artifacts land in `logs/` and are published by CI, same as the plugin suites:
      `monokit2.db`, `monokit2.log`, `test-output.log`, `coverage.out`, `coverage.txt`
- [ ] Redmine suite against a stubbed Redmine API: create / update / reopen / close,
      the 6h `findRecentSimilarIssue` dedupe window, `CreateRedmineNews`.
      Needs the base URL to be injectable — `CreateRedmineIssue` builds it from
      `GlobalConfig.Redmine.Url` and there is no seam for a test server yet.
- [ ] Interval-throttle coverage for alarms and issues. `IsTestMode()` skips the
      interval branch entirely, so the only way to test it is to make the clock
      or the interval injectable.
- [ ] TUI suite (`fetchAvailablePlugins`, `fetchChecksums`, `getPluginVersion`) —
      all three hit the network or shell out, same injectability problem

## Known bugs the suite pinned down

- [ ] `HandleCommonPluginArgs(args, ...)` ignores its `args` parameter and switches
      on `os.Args[1]` instead. Harmless today because every caller passes `os.Args`,
      wrong the moment one passes a filtered slice.
      `TestHandleCommonPluginArgsIgnoresItsArgsParameter` fails when this is fixed —
      replace it with the inverse assertion then.
- [ ] `InitConfig` has no `default` case, so a config file it does not know about is
      skipped in silence. A plugin declaring `wppconnect.yml` gets an empty struct
      and no error. Pinned by `TestInitConfigUnknownConfigFileIsSilentlyIgnored`.
- [X] `fmt.Printf(version)` in `cli.go` treated the version as a format string; a
      `%` in a tag would corrupt the output the updater parses. Now `fmt.Print`.
- [X] Four `fmt.Errorf(message)` calls in `alarms.go` had the same problem; now
      `errors.New`.

## Shared check helpers

monokit1 kept these in `common/`; in monokit2 each plugin writes its own copy,
so the same bug has to be fixed N times.

- [ ] Service status: systemd unit active/installed, with SysV init and FreeBSD
      rc.d fallbacks (monokit1 `common.GetServiceStatus`)
- [ ] Process helpers: `ProcGrep`, `ProcRunning`, `ConnsByProc`, `ConnsByProcMulti`
- [ ] TCP port-listening probe (rmqHealth, traefikHealth, redisHealth, pgsqlHealth
      all need it)
- [ ] Top-N processes by CPU / memory + markdown formatting (osHealth, winHealth)
- [ ] TLS endpoint inspection: dial, read the leaf certificate, days-until-expiry
      (opnsenseHealth, postalHealth, zimbraHealth all repeat this)
- [ ] endoflife.date client with throttling and notified-version dedupe (osHealth
      and k8sHealth both need it)
- [ ] HTTP client with the monokit user agent, timeout and retry
      (monokit1 `common.AddUserAgent` + lbPolicy's `doWithRetry`)
- [ ] Exported semver comparison (`parseSemver` is private today, vaultHealth and
      the updater both want it)
- [ ] File helpers: `CreateDirIfNotExists`, `IsOlderThan`, `MoveFile`,
      `ChangeOwnership`
- [ ] Package presence: `DpkgPackageExists` and an rpm equivalent
- [ ] `IsEmptyOrWhitespaceStr` and the other small string helpers

## Health summary output

monokit1 rendered a lipgloss box per component (`common/display.go`:
`DisplayBox`, `SectionTitle`, `ListItem`, `StatusListItem`,
`SimpleStatusListItem`) and every component had a `ui.go` with
`RenderCompact` / `RenderAll`. monokit2 has none of it — `lib/tui.go` is the
plugin manager, not a health renderer, so plugin output is log lines only.

- [ ] Box renderer + section/list/status-line helpers in lib
- [ ] `RenderCompact` / `RenderAll` convention documented in PLUGIN-DEVELOPMENT.md
- [X] `CreateMarkdownTable` (used for alarm bodies)

## State DB

- [X] GORM + automigration (`schema.go`)
- [X] `CronInterval` helpers (`GetLastCronInterval`, `CreateOrUpdateCronInterval`)
- [ ] Typed tables for the plugins that are not wired yet (mongodb, rabbitmq, es,
      vault, k8s, mail/zimbra/pmg/postal, traefik, pritunl, wppconnect, upcheck,
      glb, ssh-notifier, win, opnsense)
- [ ] Note: monokit1 stored plugin state in a `healthdb` key/value table. monokit2
      uses explicit typed tables per plugin instead — the KV shape is deliberately
      not being ported.

## Packaging

- [ ] Tagged releases so plugins can pin a lib version instead of tracking `main`
- [ ] `go.work` / submodule wiring documented for a fresh checkout
- [ ] CHANGELOG generated from tags
