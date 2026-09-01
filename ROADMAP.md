# monokit_lib Roadmap

Shared library every monokit2 plugin links against. Feature parity with
monokit1 `common/` (alarm, config, display, service, proc, file, http, apilog,
redmine) plus the helpers plugins currently reimplement.

Alarm / Redmine / config / logging *feature* parity is tracked in the core
[../ROADMAP.md](../ROADMAP.md) so it stays in one place. This file tracks
lib's own gaps: testing, CI, and the shared surface plugins keep duplicating.

## Testing & CI

lib has no tests and no workflow today. It is compiled into every plugin, so a
regression here breaks the whole fleet at once, and it is the only module in
the project with no gate at all.

- [ ] `.github/workflows/release.yml` — build + test + release, matching the shape
      pluginTemplate already uses (Go 1.25, `just`, Podman, artifact upload)
- [ ] `justfile` with `build`, `build-all`, `test`, `test-<suite>`, `clean` recipes
- [ ] `Containerfile` test harness (systemd as PID 1, artifacts to `logs/`), same
      pattern as the plugin repos
- [ ] Cross-compile gate: linux/amd64, linux/arm64, windows/amd64, freebsd/amd64 —
      lib carries the `shutdown_notifier_linux.go` / `_stub.go` build-tag split and
      nothing currently proves the non-Linux side still compiles
- [ ] `go vet` + `gofmt -l` + `staticcheck` as a failing step
- [ ] Unit suite for the pure functions: `parseSemver`, `isMajorVersionChange`,
      `isVersionMatch`, `isChecksumMatch`, `CreateMarkdownTable`,
      `RedmineDate` marshal/unmarshal/Value/Scan, `RedmineAPIObject` Value/Scan
- [ ] Config suite: `InitConfig` per config file, missing-file handling, malformed
      YAML, `CheckPluginDependencies`
- [ ] Schema suite: `InitializeDatabase` automigration from empty and from an
      older schema (every plugin's tables migrate through lib)
- [ ] Alarm suite against a stubbed Zulip endpoint: interval throttling, limit
      throttling, up/down state transitions, `GetLastZulipAlarm(s)`
- [ ] Redmine suite against a stubbed Redmine API: create / update / reopen / close,
      the 6h `findRecentSimilarIssue` dedupe window, `CreateRedmineNews`
- [ ] Updater suite: checksum verify, backup/restore/cleanup, major-version guard,
      devel-prerelease tag resolution (download paths stubbed)
- [ ] Coverage report published as a CI artifact

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
