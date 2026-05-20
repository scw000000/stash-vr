# M4d: Remove legacy endpoints — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete `/deovr`, `/heresphere`, the auto-section materializer, and their config flags. Simplify the root index page. Repair any consumer that broke as a side-effect.

**Architecture:** Five sequential tasks. Each task corresponds to one phase of the §4 removal strategy. Each ends with a clean `go build ./...`. Compile errors guide what else needs deleting.

**Tech Stack:** Go 1.24, chi router, html/template.

**Spec:** [docs/superpowers/specs/2026-05-09-m4d-remove-legacy-endpoints.md](../specs/2026-05-09-m4d-remove-legacy-endpoints.md)

**No tests in this project.** Verification is `go vet ./...`, `go build ./...`, and the manual checks at task ends.

**Prerequisite:** M4a is the practical minimum. M4b/M4c can be in any state — none of them depend on `/deovr` or `/heresphere`.

---

## Task 1: Drop the routes

**Files:**
- Modify: `internal/api/router.go`

**Goal:** Make `/deovr` and `/heresphere` return 404 by un-mounting them. Packages stay on disk for now; Task 2 deletes them.

- [ ] **Step 1: Open router.go and find the mounts**

In [internal/api/router.go](../../../internal/api/router.go), locate the chi router setup. There will be lines like:

```go
import (
    ...
    "stash-vr/internal/api/deovr"
    "stash-vr/internal/api/heresphere"
    ...
)
...
r.Mount("/deovr", deovr.Router(libraryService))
r.Mount("/heresphere", heresphere.Router(libraryService))
```

(Exact lines may differ — read the file to find them.)

- [ ] **Step 2: Remove the mounts and the imports**

Delete the two `r.Mount` lines. Delete the two import lines.

- [ ] **Step 3: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean. The `deovr` and `heresphere` packages are still on disk but unreachable.

- [ ] **Step 4: Manual verify**

Build, run. Hit:

```
curl -i http://localhost:9666/deovr
```

Expected: `HTTP/1.1 404 Not Found`. Same for `/heresphere`.

Hit `/browse` — works normally (regression check).

- [ ] **Step 5: Commit**

```
git add internal/api/router.go
git commit -m "m4d: drop /deovr and /heresphere route mounts"
```

---

## Task 2: Delete the packages

**Files:**
- Delete: `internal/api/deovr/` (whole directory)
- Delete: `internal/api/heresphere/` (whole directory)
- Modify: any file that imports the deleted packages (compile errors guide this)

**Goal:** Remove the actual package source. Compile errors point to remaining consumers; address each.

- [ ] **Step 1: Delete `internal/api/deovr/`**

```
git rm -r internal/api/deovr/
```

- [ ] **Step 2: Delete `internal/api/heresphere/`**

```
git rm -r internal/api/heresphere/
```

- [ ] **Step 3: Build to find broken imports**

Run: `go build ./...`

Expected: zero, one, or many compile errors. Record what fails.

- [ ] **Step 4: Repair each broken import**

For each compile error that says "imported and not used" or "package X not found," walk to the offending file and:

(a) If the import is only used to call a deovr/heresphere helper that the file otherwise doesn't need: remove the import.
(b) If the file *did* use deovr/heresphere as part of its own functionality (e.g., the root page's index handler links to `/deovr`): keep the import removal AND remove the calling code (links, redirects, etc.). Such code is dead since Task 1 dropped the routes.

The most likely site is `internal/api/web/web.go` — the root page handler. Replace any `import "stash-vr/internal/api/deovr"` / `heresphere` with the simpler local handler. The HTML template (`internal/static/index.gohtml`) is updated in Task 4 — for this task, just satisfy the compiler.

Repeat `go build ./...` after each fix.

- [ ] **Step 5: Vet**

Run: `go vet ./...`

Expected: clean.

- [ ] **Step 6: Manual verify**

Run the binary; hit `/browse` (normal) and `/` (might look ugly because `index.gohtml` still mentions DeoVR — Task 4 fixes). No 500s.

- [ ] **Step 7: Commit**

```
git add -A
git commit -m "m4d: delete /deovr and /heresphere packages, repair callers"
```

---

## Task 3: Delete autosection materializer + AUTO_SECTIONS_* config flags

**Files:**
- Delete: `internal/library/autosection.go`
- Delete: `internal/library/autosection_materialize.go`
- Modify: `internal/library/sections.go` and any other library files that reference autosection helpers
- Modify: `internal/config/application.go`

**Goal:** Drop the auto-section code path. Config no longer recognizes `AUTO_SECTIONS_*`.

- [ ] **Step 1: Delete the autosection files**

```
git rm internal/library/autosection.go internal/library/autosection_materialize.go
```

- [ ] **Step 2: Build, walk compile errors**

Run: `go build ./...`

Expected: errors in `internal/library/sections.go` (or similar) where the materializer was called. Open each broken file, remove the calls, and clean up dead types (the `Section` type, `Stats`, etc. — these were only emitted to JSON for `/deovr` and `/heresphere`).

If `library.Service.GetSections` is no longer called anywhere (grep `GetSections` in `internal/`), delete the method body and its singleflight key.

- [ ] **Step 3: Drop the AUTO_SECTIONS_* config**

In [internal/config/application.go](../../../internal/config/application.go), find these constant blocks (around lines 24-29):

```go
envKeyAutoSectionsPerformers = "AUTO_SECTIONS_PERFORMERS"
envKeyAutoSectionsTags       = "AUTO_SECTIONS_TAGS"
envKeyAutoSectionsAggregates = "AUTO_SECTIONS_AGGREGATES"
```

Delete the const declarations. Then find and delete the corresponding fields on the `applicationConfig` struct, the pflag declarations in `Init()`, and the viper bindings/reads. (Search for `AutoSections` to find each occurrence.)

- [ ] **Step 4: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 5: Manual verify**

Run the binary with `AUTO_SECTIONS_PERFORMERS=true` in the env or as a flag. Stash-vr should start normally; viper silently ignores unrecognized env vars, and pflag emits an "unknown flag" warning if it was passed as a flag.

```
AUTO_SECTIONS_PERFORMERS=true ./stash-vr
```

(With `STASH_GRAPHQL_URL` set as required.) Verify `/browse` works.

- [ ] **Step 6: Commit**

```
git add -A
git commit -m "m4d: drop AUTO_SECTIONS_* config flags + autosection materializer"
```

---

## Task 4: Simplify the root index page

**Files:**
- Modify: `internal/static/index.gohtml`
- Modify: `internal/api/web/web.go`
- Possibly modify: `internal/api/router.go` (if the `/filters` POST is dropped)
- Possibly delete: `internal/config/user.go` (if filter-ordering config is dead)

**Goal:** The root `/` page is now a simple status banner with a link to `/browse`. No DeoVR/HereSphere mentions, no filter-ordering form.

- [ ] **Step 1: Audit `/filters` and `config/user.go`**

Grep for callers of `config.User()` and the `/filters` route:

```
git grep "config.User"
git grep "/filters"
```

If the only caller of `config.User().Filters` was the deovr/heresphere `getSectionsByUserConfig` flow (now deleted), the config is dead. If `/filters` POST has no remaining client (the index page form is the only known consumer), the route is dead.

If both are dead:
- Delete `internal/config/user.go`.
- Remove the `/filters` route from `internal/api/router.go`.
- Remove any `config.User()` reads from elsewhere.

If something else uses them (e.g., M4c's filter-options endpoint), keep them.

- [ ] **Step 2: Rewrite `internal/static/index.gohtml`**

Replace the contents with a minimal page. (Read the current file first to capture the styles you want to keep.) Suggested shape:

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>stash-vr — {{.Version}}</title>
<style>
  body { font-family: system-ui, sans-serif; background: #111; color: #eee; padding: 32px; }
  h1 { font-size: 1.4rem; }
  a.btn { display: inline-block; margin-top: 16px; padding: 12px 20px; background: #2c5282; color: #fff; text-decoration: none; border-radius: 6px; }
  a.btn:hover { background: #3776c2; }
  .stat { color: #aaa; }
  .err { color: #f55; margin-top: 16px; }
</style>
</head>
<body>
  <h1>stash-vr {{.Version}} ({{.SHA}})</h1>
  <p class="stat">Stash: {{if .StashOK}}connected ({{.SceneCount}} scenes){{else}}<span class="err">unreachable</span>{{end}}</p>
  <a class="btn" href="/browse">Open browser</a>
</body>
</html>
```

- [ ] **Step 3: Update `internal/api/web/web.go`**

The handler that serves `/` should populate `Version`, `SHA`, `StashOK`, `SceneCount`. Simplest implementation: call the existing GraphQL `FindScenes` with `per_page=1` and read `count` from the response.

(Read the existing web.go to keep the bits that worked.)

- [ ] **Step 4: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 5: Manual verify**

Run the binary. Visit `/`:
- Page renders with version + commit SHA.
- "Stash: connected (N scenes)" line shows correct N.
- "Open browser" button links to `/browse`.
- No mention of `/deovr` or `/heresphere`.

Visit `/filters` if it was kept → still loads. If it was removed → 404.

Visit `/browse` → unchanged.

- [ ] **Step 6: Commit**

```
git add -A
git commit -m "m4d: simplify root index page; drop filters UI if unused"
```

---

## Task 5: Update README

**Files:**
- Modify: `README.md`

**Goal:** README no longer documents `/deovr`, `/heresphere`, the HereSphere tag-write-back contract, or the `AUTO_SECTIONS_*` flags. It does document `/browse` and the WebXR setup.

- [ ] **Step 1: Read the current README**

Open `README.md`. Identify sections that reference:
- DeoVR (player URL, library URL, setup)
- HereSphere (player URL, tag-write-back vocabulary)
- AUTO_SECTIONS_* environment variables
- The legend strings (`#:Name`, `@:`, `Studio:`, `O-Count:`, `/o`, `/org`, `Rating:`)
- The "Manage metadata" section that documents the HereSphere contract

- [ ] **Step 2: Delete those sections**

Cut each block. Keep:
- Project overview
- Build instructions (`go build`, Docker)
- Required env vars (`STASH_GRAPHQL_URL`, `STASH_API_KEY`)
- The `FAVORITE_TAG` env var (still used by M4a)
- The `/browse` UX description
- Caddy / HTTPS / Quest 3 setup if present

- [ ] **Step 3: Add a brief migration note (optional)**

A few sentences at the top noting that as of v0.X, `/deovr` and `/heresphere` are removed; the app targets Quest's Meta Browser at `/browse`. Helps anyone who pulled an older docker image.

- [ ] **Step 4: Commit**

```
git add README.md
git commit -m "m4d: update README — remove DeoVR / HereSphere / AUTO_SECTIONS docs"
```

---

## Self-review checklist

- **Spec coverage:** All five phases of spec §4 are mapped to a task. Routes (Task 1), packages (Task 2), library + config (Task 3), root page (Task 4), docs (Task 5).
- **No placeholders:** Each task has concrete file paths and commands. Some compile errors are expected to vary based on actual codebase state — flagged in Task 2 and Task 3.
- **Frequent commits:** One per task. Five commits total.
- **YAGNI:** No deprecation path, no replacement endpoints, no graceful handoff. Just delete.
- **Risk:** the implementation is "follow the compiler." If a compile error reveals shared code (M4a/b/c uses something we thought was deovr-only), keep the shared piece. The plan doesn't enumerate every import dependency because they're easier discovered than predicted.
