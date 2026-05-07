# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
## IMPORTANT: At the start of every conversation
- Read the relevant planning/work files for the task area.
- Follow the workflow below.

# AI Agent & Developer Workflow Guidelines

## Workflow Orchestration

### 1. Plan Mode Default
* *Enter plan mode* for ANY non-trivial task (3+ steps or architectural decisions).
* If something goes sideways, *STOP* and re-plan immediately – don't keep pushing.
* Use plan mode for *verification steps*, not just building.
* Write detailed specs upfront to reduce ambiguity.

### 2. Subagent Strategy
* Use subagents liberally to keep the main context window clean.
* Offload research, exploration, and parallel analysis to subagents.
* For complex problems, throw more compute at it via subagents.
* One task per subagent for focused execution.

### 3. Self-Improvement Loop
* After *ANY* correction from the user: write the lesson into the correct file (see *Lesson Routing* below).
* Write rules for yourself that prevent the same mistake.
* Ruthlessly iterate on these lessons until the mistake rate drops.
* Review lessons at the start of a session for the relevant project.

#### Lesson Routing (where to write a new lesson)

Pick the most specific file that applies. Order checks top-to-bottom:

**`Tasks/lessons.md`** — only if the rule is genuinely cross-cutting (applies to any project or any component): generic dev principles (root cause, threading, code style, review process, Windows shell quirks, Qt/PySide patterns that aren't Koken-specific). If you can rephrase the lesson without naming any Koken class/concept and still keep its meaning, it belongs here.

When in doubt, prefer the more specific file. A lesson written in `Tasks/lessons.md` that names Koken classes is misfiled and will pollute the cross-cutting file.

### 4. Verification Before Done
* Never mark a task complete without *proving it works*.
* Diff behavior between the main branch and your changes when relevant.
* Ask yourself: "Would a staff engineer approve this?"
* Run tests, check logs, and demonstrate correctness.

### 5. Demand Elegance (Balanced)
* For non-trivial changes: pause and ask, "is there a more elegant way?"
* If a fix feels hacky: "Knowing everything I know now, implement the elegant solution."
* Skip this for simple, obvious fixes – *don't over-engineer*.
* Challenge your own work before presenting it.

## What this is

Stash-VR is a Go HTTP service that bridges a [Stash](https://github.com/stashapp/stash) instance and VR video players (HereSphere, DeoVR). It exposes the player-specific JSON/HTML APIs each player expects, translating them into Stash GraphQL calls. There is no database — Stash is the source of truth.

## Build / run / generate

Go 1.24 module. Entrypoint: [cmd/stash-vr/main.go](cmd/stash-vr/main.go).

```sh
# Build (matches Dockerfile flags)
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X stash-vr/internal/build.Version=dev -X stash-vr/internal/build.SHA=local" -o stash-vr ./cmd/stash-vr

# Run locally (minimum required env)
STASH_GRAPHQL_URL=http://localhost:9999/graphql ./stash-vr

# Regenerate the Stash GraphQL client (genqlient) after editing
# internal/stash/gql/documents/*.graphql or schema/local.graphql
go generate ./cmd/stash-vr        # uses //go:generate in main.go

# Cross-platform release builds are produced by goreleaser (.goreleaser.yaml)
# CI: .github/workflows/publish_bin.yml (release), publish_docker.yml (push to develop / release)
```

There is no test suite and no lint config — `go vet ./...` and `go build ./...` are the only standard checks.

## Code generation: genqlient

`internal/stash/gql/generated.go` is fully generated from:
- `internal/stash/gql/schema/local.graphql` (a copy of Stash's schema)
- `internal/stash/gql/documents/{query,mutation}.graphql`

Never hand-edit `generated.go`. To add a Stash call: add the operation to `documents/*.graphql`, run `go generate ./cmd/stash-vr`, then call `gql.YourOperation(ctx, client, ...)` from Go. The schema file must be kept in sync with the targeted Stash version (last bumped for Stash v0.31 — see commit `6057405`). Bindings (Time, Int64, Map, etc.) live in `internal/stash/gql/genqlient.yaml`.

## Architecture

```
cmd/stash-vr/internal/run.go     -> wires config -> stash client -> library.Service -> server.Listen
internal/server                  -> http.Server + graceful shutdown via errgroup
internal/api/router.go           -> chi router; mounts /heresphere, /deovr, /, /cover, /filters
internal/api/{heresphere,deovr}  -> player-specific HTTP handlers + JSON DTOs
internal/api/web                 -> the index.gohtml status/config page
internal/api/heatmap             -> composes scene cover + heatmap PNG on the fly
internal/library                 -> the only stateful component: in-memory scene/tag cache + sync ops
internal/stash                   -> thin wrappers around the generated GraphQL client
internal/stash/filter            -> converts Stash SavedFilter JSON -> SceneFilterType for FindScenes
internal/config                  -> Application (env/flags via viper+pflag) and User (config.json) config
internal/static                  -> embed.FS for index.gohtml, loading.html, icon.png
```

Key flows:

- **Indexing.** `library.Service.GetSections` (called per index request, deduped via `singleflight`) loads saved Stash filters, runs each through `stash/filter` to build a `SceneFilterType`, fetches scene IDs in parallel, then **rebuilds the scene cache** with `nil` placeholders keyed by id. Stats (`Links`, `Scenes`) are updated here.
- **Scene fetch.** `GetScenes` lazily fills `nil` placeholders by batch-calling `gql.FindScenes`, also deduped via singleflight on key `"scenes"`. `GetScene(id, forceFetch)` is the per-video path; HereSphere `videoData` requests use `forceFetch=true` semantics elsewhere when a fresh read is required after a write.
- **Tag decoration.** After a scene is fetched, `decorateTags` walks ancestor tags via `tagCache` and appends them with sort_name prefixed by `prefix.SvrAncestor`. Tags whose `Sort_name` matches `EXCLUDE_SORT_NAME` (default `hidden`) are skipped. Prefix constants in `internal/prefix/constant.go` are how the player-specific layers tell categorized vs. ancestor vs. parent tags apart.
- **Two-way sync (HereSphere only).** `internal/api/heresphere/tag.go` parses incoming `Video Tags` strings using the legends in [internal/api/internal/legend.go](internal/api/internal/legend.go) (`#:Name`, `@:`, `Studio:`, `O-Count:`, `/o`, `/org`, `Rating:`, etc.) and dispatches to `library.Service.Update*` methods, which call generated GraphQL mutations. The legend strings are the contract with HereSphere — changing them is a breaking UX change.
- **Filter ordering.** If `CONFIG_PATH` is set and `config.json` lists filters, that order wins (`buildFiltersByUserConfig`). Otherwise filters are ordered by their position on the Stash front page UI config (`buildFiltersByFrontpage`), with non-frontpage filters appended.
- **Streams.** `internal/stash/stream.go` produces the direct + transcoding URLs handed to players. `stash.ApiKeyed(url)` appends the API key to media URLs so the headset can fetch directly from Stash.
- **Base URL.** All player-facing URLs are built relative to `internal/api/internal.GetBaseUrl(req)`, which respects `FORCE_HTTPS`. Don't hardcode scheme/host.

## Configuration

Two layers, both in `internal/config`:
- **Application** (`application.go`) — env vars / pflags via viper. Loaded once in `Init()`, read via `config.Application()`. List of keys is the source of truth for what env vars are supported (the README's "More" section can drift).
- **User** (`user.go`) — `config.json` at `CONFIG_PATH/config.json`, currently only filter ordering/renames/disables. Edited via `POST /filters` from the index web UI. If `CONFIG_PATH` is empty, changes apply in memory but don't persist (warning logged).

`config.Redacted(s)` is used in logs to mask `STASH_API_KEY` and host info; `DISABLE_REDACT=true` turns it off.

## Conventions worth knowing

- `singleflight` is used to coalesce concurrent identical requests (sections build, scenes fetch). When adding a new expensive operation invoked by multiple HTTP handlers, follow the same pattern.
- Logging is structured zerolog; handlers attach `mod` (e.g. `heresphere`, `deovr`) and `videoId` via `internal.LogRoute` / `internal.LogVideoId` middleware. Use `log.Ctx(ctx)` rather than the global logger inside request paths.
- HereSphere endpoints set `HereSphere-JSON-Version: 1` via middleware — preserve it.
- Player DTOs (`videoDataDto`, etc.) use omitempty pointers heavily because HereSphere/DeoVR distinguish "absent" from "zero". When adding fields, mirror that pattern.
- The README documents the user-facing tag/marker contract in detail under "Manage metadata"; treat it as the spec when changing HereSphere tag parsing.
