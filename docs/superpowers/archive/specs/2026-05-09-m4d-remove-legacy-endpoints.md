# M4d design: remove legacy `/deovr` and `/heresphere` endpoints

**Date:** 2026-05-09
**Status:** Drafting (decision approved 2026-05-09).
**Predecessors:** M4a / M4b / M4c shipped (or in progress). The `/browse` UX is the load-bearing front-door; the `/deovr` and `/heresphere` endpoints have been dormant since the M2 pivot to Quest's Meta Browser + WebXR.
**Successors:** None planned.

---

## 1. Context (why this milestone)

`/deovr` and `/heresphere` were the original UX — JSON DTOs consumed by the DeoVR and HereSphere VR apps respectively. The 2026-05-08 brainstorm decisively pivoted away from those apps:

- DeoVR's in-VR Chromium browser does not support WebXR.
- DeoVR's library renderer cannot replicate SLR's right-panel facet UX through any URL we control.
- The user uses Quest's Meta Browser at `https://stash-vr.duckdns.org/browse` directly.

`/deovr` and `/heresphere` still mount and ship JSON DTOs, but the user has no ongoing workflow that depends on them. The auto-sections code path (driven by `AUTO_SECTIONS_PERFORMERS` / `_TAGS` / `_AGGREGATES`) was added to scale these endpoints to a usable UI inside DeoVR's library renderer; that effort was abandoned.

Carrying these packages forward has a cost:

- Two route trees (~16 Go files) with their own DTOs, mutation handlers (HereSphere two-way sync), GraphQL-shape constraints, and config flags.
- Six file paths in `internal/api/heresphere/` plus four in `internal/api/deovr/` — all dead code from the user's perspective.
- The auto-section materializer (`internal/library/autosection*.go`, ~250 LoC) exists solely to feed these endpoints.
- Tag-write-back legend strings (`internal/api/internal/legend.go`) parse HereSphere's `Video Tags` field; only HereSphere uses this contract.
- The root status page (`/` via `internal/api/web`) and `/filters` UI advertise these endpoints in the docs and tab order.

M4d removes all of it.

## 2. Goal & non-goals

**Goal:** Delete `/deovr`, `/heresphere`, and their entire surface area — packages, routes, config flags, dependencies in `library` and `api/internal`, and references from the root status page. After M4d, the only HTTP routes are: `/`, `/browse/*`, `/cover/{id}`, `/static/*` (the existing static asset routes), and any plumbing under those.

**Success criteria:**

1. `/deovr` and `/deovr/*` return 404. `/heresphere` and `/heresphere/*` return 404.
2. `internal/api/deovr/` directory is deleted. `internal/api/heresphere/` directory is deleted.
3. The auto-section flags (`AUTO_SECTIONS_PERFORMERS`, `AUTO_SECTIONS_TAGS`, `AUTO_SECTIONS_AGGREGATES`) are removed from `internal/config/application.go`. The auto-section materializer code in `internal/library` is deleted.
4. The root `/` index page no longer mentions `/deovr` or `/heresphere`. It shows only the `/browse` entry point and stash-vr's basic status (version, GraphQL connectivity).
5. `internal/api/internal/legend.go` (the HereSphere tag-write contract) is deleted unless any of its constants are used outside HereSphere (audit in implementation).
6. The `/filters` POST route — used by the root page's filter-ordering form — is reviewed for relevance: M4d retains it only if it still has any consumers; otherwise it's removed.
7. `go vet ./...` and `go build ./...` pass.
8. M4a / M4b / M4c surfaces unaffected: `/browse`, `/browse/scene/{id}`, all chip / mutation / VR-side functionality work identically.

**Non-goals:**

- Adding any deprecation message or grace period. The user has been off these endpoints for a while; clean removal is fine.
- Renaming or relocating functionality that *also* exists under `/browse`. M4d only deletes; it doesn't reshape `/browse`.
- Documentation rewrites of README.md beyond removing the deleted-feature sections. The README's "Manage metadata" section documents the HereSphere tag write-back contract — that section is removed.

## 3. What gets deleted

### 3.1 Packages

```
internal/api/deovr/
    http.go
    index.go
    router.go
    videodata.go

internal/api/heresphere/
    event.go
    http.go
    index.go
    playback.go
    resolution.go
    router.go
    scan.go
    summary.go
    summaryid.go
    tag.go
    videodata.go
```

### 3.2 Library code

```
internal/library/autosection.go
internal/library/autosection_materialize.go
```

### 3.3 Config flags

In `internal/config/application.go`, remove:

- `envKeyAutoSectionsPerformers` and its viper binding
- `envKeyAutoSectionsTags` and its viper binding
- `envKeyAutoSectionsAggregates` and its viper binding
- The corresponding fields on `applicationConfig`
- Any default values, help text, or pflag declarations associated with the above

`FAVORITE_TAG` stays (M4a's favorite mechanism uses it).
`EXCLUDE_SORT_NAME` stays (used by browse sidebar via `internal/api/browse/entities.go`).

### 3.4 Library service surface

`library.Service.GetSections` was the entry point used by `/deovr` and `/heresphere` to assemble their JSON section lists. Audit:

- If `GetSections` is called from anywhere in `internal/api/web/*` (the root index page) or `internal/api/browse/*`, the call site is reviewed and either removed or replaced with a simpler "scene count" lookup.
- The `Sections`-related types in `library` (e.g., `Section`, `Stats.Links`, `Stats.Scenes`) are deleted unless retained-for-stats-only.
- The `singleflight` deduping for "build sections" goes away with `GetSections`.

### 3.5 The root index page

`internal/api/web/` (and `internal/static/index.gohtml`) currently shows: version, links to `/browse`, `/deovr`, `/heresphere`, plus the "configure filter ordering" form that POSTs to `/filters`.

After M4d, the page is simplified to:

- Version / commit info (kept).
- Direct link to `/browse`.
- Stats: scenes counted in Stash (single number).
- (Removed) DeoVR / HereSphere library URLs.
- (Removed) Filter-ordering form, unless any other consumer of `config.User.Filters` exists. (Audit: `internal/config/user.go`'s filter ordering was M3-era for `/deovr` section ordering. With `/deovr` gone, this config is dead. Drop both the form and `config/user.go`.)

### 3.6 Static / asset references

`internal/static/loading.html` is used as DeoVR's "loading scene" icon — delete.
`internal/static/icon.png` is used by DeoVR / HereSphere index DTOs as a logo — keep if used by `/browse` or `/`, otherwise delete.

### 3.7 GraphQL queries used only by deleted code

Audit `internal/stash/gql/documents/{query,mutation}.graphql` for queries used only by `deovr` / `heresphere` packages. Candidates:

- HereSphere two-way sync mutations: `SceneAddTag`, `SceneRemoveTag`, `SceneSetRating`, `SceneIncrementO`, etc. — but **most of these are reused by M4a's mutation handlers** ([scene_post.go](../../../internal/api/browse/scene_post.go)). Verify each before deletion.
- Auto-section queries: `FindPerformerByName`, `FindPerformersByIDs`, `FindPerformersWithSceneCount`, `FindTagByName`, `FindTagsByIDs`, `FindTags`, `FindAllTags` — used by autosection materializer. Some may be reused by M4c's `/browse/filter-options/{kind}` endpoint (Task 2 of M4c plan calls `LoadSidebar` which uses these). Verify each.

Conservatively, only delete a generated GraphQL operation when nothing in the post-M4d codebase calls it. Since `genqlient` regen happens on `go generate`, an unused operation in the document is just dead bytes — preferred outcome is to also remove it from `documents/query.graphql` and regen.

### 3.8 README

Remove sections of `README.md` that describe `/deovr`, `/heresphere`, the HereSphere tag-write-back vocabulary (`#:`, `@:`, `Studio:`, `O-Count:`, `/o`, `/org`, `Rating:`), the auto-sections environment variables, and DeoVR-specific setup. Keep `/browse` setup and the WebXR Meta Browser instructions.

## 4. Strategy

Five-phase removal:

1. **Drop the routes.** Remove the deovr/heresphere mounts in `internal/api/router.go`. Build still passes because the packages exist; they're just not reachable.
2. **Delete the packages.** Remove `internal/api/deovr/` and `internal/api/heresphere/` directories. Build breaks at any remaining imports.
3. **Repair imports.** Walk each compile error: each one is either (a) library code only used by deovr/heresphere → delete, or (b) library code shared with `/browse` → keep, just remove the deleted import.
4. **Drop config + autosection.** Remove `AUTO_SECTIONS_*`, `internal/library/autosection*.go`, and the `GetSections` plumbing if nothing else calls it.
5. **Simplify the root page + README.** Update `internal/static/index.gohtml`, `internal/api/web/`, and `README.md`.

Each phase is its own commit. Phase 3 may need several sub-commits depending on what surfaces.

## 5. Risks

- **Shared code with M4a/M4b/M4c.** Most likely conflict: GraphQL operations or library helpers used by both. Mitigation: don't aggressively delete shared types; let `go vet` and `go build` guide what's truly orphan.
- **HereSphere tag legend re-used in M4a's parsing.** [internal/api/internal/legend.go](../../../internal/api/internal/legend.go) defines tag prefix strings (`#:`, `@:`, etc.). M4a's tag-add/tag-remove handlers don't parse these — they accept raw tag names. Confirm legend.go is HereSphere-only before deletion.
- **`config/user.go` filter ordering.** This was the per-user `/filters` UI for ordering /deovr's section list. With `/deovr` gone, the config is dead. Drop the file. If the index page still needs to know "what scenes exist," it can call `library.Service.GetScenes` directly (M4c already exposes the JSON grid).
- **The `/filters` POST handler.** Removing it requires updating any client that posts to it — the root page's form is the only known consumer.
- **CI / Docker builds.** `Dockerfile` references the same entrypoint; nothing to change unless `docker-compose.yml` mentions the deleted env vars.
- **Goreleaser config.** `.goreleaser.yaml` doesn't depend on routes; nothing to change.

## 6. Validation

After M4d ships:

1. `go vet ./...` clean.
2. `go build ./...` clean.
3. Run the binary; visit `/browse` — index, sidebar, search, scene detail, mutations, VR all work (M4a/b/c regression).
4. Visit `/deovr` → 404. Visit `/deovr/<id>` → 404. Visit `/heresphere` → 404.
5. Visit `/` → simplified index, version + scene count + link to /browse. No mention of DeoVR or HereSphere.
6. Set `AUTO_SECTIONS_PERFORMERS=true` in the env → process starts (flag is unrecognized but viper's default behavior is to ignore extras — or, if pflag panics on unknown flag, the start should fail; verify and document).
7. Repository directory listing under `internal/api/` shows only `browse/`, `heatmap/`, `internal/`, `web/`, plus `router.go`. Directory listing under `internal/library/` no longer has `autosection*.go`.
8. README no longer mentions `/deovr`, `/heresphere`, or `AUTO_SECTIONS_*`.

## 7. Open follow-ups

- If anyone actually used `/deovr` or `/heresphere` in their setup and complains after the ship, restore behavior is a `git revert`. Spec doesn't include any deprecation path because the user has already pivoted off these surfaces.
- M4d-followup: prune any unused GraphQL operations from `documents/query.graphql` once the audit in §3.7 is complete and the build is stable. Could be its own small cleanup pass.
