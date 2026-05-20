# M1 /browse 2D player + catalog search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/browse` self-sufficient inside Quest 3's Meta Browser by adding inline 2D playback to scene detail and a catalog-wide title search to the index, while removing the now-unused DeoVR launch buttons.

**Architecture:** Server-rendered Go templates throughout (no SPA, no build pipeline). Search threads `FindFilterType.Q` through the existing `fetchSceneIDs` GraphQL helper. The 2D player is a vanilla HTML5 `<video controls playsinline autoplay muted preload="metadata">` pointing at Stash's existing direct stream URL (`*vd.SceneParts.Paths.Stream`).

**Tech Stack:** Go 1.24 (existing), chi/v5 (existing), `html/template` (existing pattern), embed.FS (existing pattern), Stash GraphQL via genqlient (existing).

**Spec:** [docs/superpowers/specs/2026-05-08-m1-browse-2d-player-search.md](../specs/2026-05-08-m1-browse-2d-player-search.md)

**Project conventions to honor:**
- No test suite per [CLAUDE.md](../../../CLAUDE.md). "TDD" here means manual verification after each task: `go vet ./...`, `go build ./...`, `curl`, then visual eyeball.
- Lowercase commit prefixes following recent style: `browse: <message>`.
- The user has approved this plan; commit steps within tasks are authorized by that approval. Do NOT amend or rebase prior commits without further explicit user request.

---

## File structure

**Modified:**
- `internal/api/browse/grid.go` — `fetchSceneIDs` signature + `buildCards` field removal.
- `internal/api/browse/index.go` — `indexHandler` and `entityHandler` read+forward `q`.
- `internal/api/browse/data.go` — add `PageData.SearchQuery`, add `SceneDetailData.DirectStreamURL`, remove `Card.DeoVRPlayURL`, remove `SceneDetailData.DeoVRPlayURL`.
- `internal/api/browse/scene.go` — drop `DeoVRPlayURL` building, build `DirectStreamURL`.
- `internal/static/browse.gohtml` — add search form, remove `.quickplay` markup + CSS.
- `internal/static/browse_scene.gohtml` — replace "Play in DeoVR" link with `<video>`.

**Not touched:** router, config, library service, GraphQL documents, `/deovr`, `/heresphere`, mutation handlers (`scene_post.go`), heatmap/cover proxy, sidebar JS.

**No new files. No new packages.**

---

## Task 1: Thread `q` through `fetchSceneIDs` (backend plumbing only)

**Files:**
- Modify: `internal/api/browse/grid.go`
- Modify: `internal/api/browse/index.go`

This task makes search work on the backend without touching templates. Verifiable by curl. Splitting the backend wire-up from the template change keeps each commit small and lets us isolate failures.

- [ ] **Step 1: Update `fetchSceneIDs` signature in `internal/api/browse/grid.go`**

The current signature is:

```go
func fetchSceneIDs(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, page int) ([]string, int, error) {
	resp, err := gql.FindSceneIdsByFilter(ctx, client, sceneFilter, &gql.FindFilterType{
		Page:      util.Ptr(page),
		Per_page:  util.Ptr(pageSize),
		Sort:      util.Ptr("created_at"),
		Direction: util.Ptr(gql.SortDirectionEnumDesc),
	})
```

Add a `q string` parameter and conditionally set `FindFilterType.Q`:

```go
func fetchSceneIDs(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, q string, page int) ([]string, int, error) {
	findFilter := &gql.FindFilterType{
		Page:      util.Ptr(page),
		Per_page:  util.Ptr(pageSize),
		Sort:      util.Ptr("created_at"),
		Direction: util.Ptr(gql.SortDirectionEnumDesc),
	}
	if q != "" {
		findFilter.Q = util.Ptr(q)
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, client, sceneFilter, findFilter)
```

The rest of the function (the loop building `out`, returning `(out, resp.FindScenes.Count, nil)`) is unchanged.

- [ ] **Step 2: Update both call sites in `internal/api/browse/index.go`**

Two callers exist: `indexHandler` (currently around line 35) and `entityHandler` (currently around line 124). Both call `fetchSceneIDs(r.Context(), h.libraryService.StashClient, ...filter..., page)`. Both need to read `q` and pass it.

In `indexHandler`, the existing `q := r.URL.Query()` shadows the name `q` for the search string. Reuse the existing variable: pull the search string via `q.Get("q")` into a `searchQ` local. Find this block at the top of `indexHandler`:

```go
func (h *httpHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tab := q.Get("tab")

	page, _ := strconv.Atoi(q.Get("page"))
```

Replace with:

```go
func (h *httpHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tab := q.Get("tab")
	searchQ := q.Get("q")

	page, _ := strconv.Atoi(q.Get("page"))
```

(One added line — the `searchQ` declaration. The existing `q` URL-Values variable stays untouched.)

Then change the `fetchSceneIDs` call inside `indexHandler`:

```go
ids, total, err := fetchSceneIDs(r.Context(), h.libraryService.StashClient, nil, searchQ, page)
```

Inside `indexHandler`, the existing line `extra := r.URL.Query()` and `extra.Del("page")` already preserves all params except `page`. Since `q` was in `r.URL.Query()`, it's preserved automatically — no `extra.Set` needed for the pager.

In `entityHandler`, do the same shape of change. Find the existing top:

```go
return func(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
```

Replace with:

```go
return func(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	searchQ := q.Get("q")
	page, _ := strconv.Atoi(q.Get("page"))
```

Then change the `fetchSceneIDs` call inside `entityHandler`:

```go
ids, total, err := fetchSceneIDs(r.Context(), h.libraryService.StashClient, sceneFilter, searchQ, page)
```

The `extra := r.URL.Query()` / `extra.Del("page")` pattern stays unchanged here too — `q` is preserved in `extra` automatically.

- [ ] **Step 3: Verify build**

Run:
```
go vet ./...
go build ./...
```

Expected: no errors. Both `go vet` and `go build` exit code 0.

- [ ] **Step 4: Verify search works via curl**

Start stash-vr against the user's running Stash:
```
go run ./cmd/stash-vr --STASH_GRAPHQL_URL=http://192.168.1.183:9999/graphql
```

Pick a substring known to appear in some scene title (the user can suggest one; otherwise pick the first word of the first card visible at `http://localhost:9666/browse`).

In another terminal:
```
curl -s 'http://localhost:9666/browse' | findstr /C:"<div class=\"card\""
curl -s 'http://localhost:9666/browse?q=<known-substring>' | findstr /C:"<div class=\"card\""
```

Expected: both return one or more lines of `<div class="card">`. The second should return a different (typically smaller) count of matching cards. Compare line counts to confirm filtering happened.

If `?q=` returns the same exact set as no-q, the search wiring is wrong — investigate `findFilter.Q` assignment or genqlient binding.

- [ ] **Step 5: Commit**

```
git add internal/api/browse/grid.go internal/api/browse/index.go
git commit -m "browse: thread q query param through fetchSceneIDs for catalog search"
```

---

## Task 2: Surface the search input in the UI

**Files:**
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/index.go`
- Modify: `internal/static/browse.gohtml`

- [ ] **Step 1: Add `SearchQuery` to `PageData` in `internal/api/browse/data.go`**

The current `PageData` struct ends with `ErrMessage string`. Add one field. The result should look like:

```go
// PageData is what browse.gohtml expects.
type PageData struct {
	Sidebar SidebarData
	Header  string
	SubHead string
	BackURL string
	Cards   []Card
	PrevURL string
	NextURL string
	PageNum int
	PageMax int
	ErrMessage string
	SearchQuery string
}
```

- [ ] **Step 2: Populate `SearchQuery` in both handlers in `internal/api/browse/index.go`**

In `indexHandler`'s `data := PageData{...}` literal (currently around line 58), add `SearchQuery: searchQ,` as a new field. The block becomes:

```go
data := PageData{
	Sidebar: sidebar,
	Header:  "All scenes — newest first",
	SubHead: fmt.Sprintf("%d scenes", total),
	Cards:   cards,
	PrevURL: prev,
	NextURL: next,
	PageNum: page,
	PageMax: pageMax,
	SearchQuery: searchQ,
}
```

Do the same in `entityHandler`'s `data := PageData{...}` literal (currently around line 150):

```go
data := PageData{
	Sidebar: sidebar,
	BackURL: "/browse",
	Header:  header,
	SubHead: fmt.Sprintf("%d scenes", total),
	Cards:   cards,
	PrevURL: prev,
	NextURL: next,
	PageNum: page,
	PageMax: pageMax,
	SearchQuery: searchQ,
}
```

- [ ] **Step 3: Add the search form to `internal/static/browse.gohtml`**

Insert a search form between the `.header` div and the `.grid` div. Find this block (currently around line 53–57):

```html
<div class="header">
{{if .BackURL}}<a href="{{.BackURL}}">← All scenes</a>{{end}}
<h1>{{.Header}}</h1>
{{if .SubHead}}<span class="subhead">{{.SubHead}}</span>{{end}}
</div>
{{if .Cards}}
<div class="grid">
```

Replace with (search form added between `</div>` and `{{if .Cards}}`):

```html
<div class="header">
{{if .BackURL}}<a href="{{.BackURL}}">← All scenes</a>{{end}}
<h1>{{.Header}}</h1>
{{if .SubHead}}<span class="subhead">{{.SubHead}}</span>{{end}}
</div>
<form method="GET" class="searchbar">
<input type="search" name="q" value="{{.SearchQuery}}" placeholder="Search titles…" autocomplete="off">
{{if .SearchQuery}}<a class="searchclear" href="?">Clear</a>{{end}}
</form>
{{if .Cards}}
<div class="grid">
```

(The `?` in the Clear link strips all query params including `q` and `page`, returning to the unfiltered current path.)

- [ ] **Step 4: Add CSS for the search bar in `internal/static/browse.gohtml`**

Inside the existing `<style>` block, anywhere between `{` rules, add:

```css
.searchbar { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; }
.searchbar input { flex: 1; padding: 8px 12px; background: #1a1a1a; border: 1px solid #333; color: #eee; border-radius: 4px; font-size: 0.95rem; }
.searchbar input:focus { outline: none; border-color: #2c5282; }
.searchclear { color: #9cf; text-decoration: none; font-size: 0.9rem; }
.searchclear:hover { color: #fff; }
```

A reasonable place is right after the existing `.errbanner { ... }` rule for grouping with other top-bar styles, but anywhere inside `<style>` works.

- [ ] **Step 5: Verify build**

```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Verify the search box renders and filters**

Restart stash-vr (Ctrl+C the previous run, restart with the same args). Then:

```
curl -s 'http://localhost:9666/browse' | findstr /C:"name=\"q\""
```

Expected: one matching line containing the input element.

Open `http://localhost:9666/browse` in a desktop browser. Expected:
- Search input visible above the grid.
- Type a known title substring, press Enter. URL becomes `/browse?q=<substring>`.
- Grid shows only matching scenes.
- A "Clear" link appears next to the input. Click it. URL becomes `/browse?` (or just `/browse`). Full grid back.
- Pagination preserves `q` (if there are enough results to span pages): click "Next ›" and the URL has both `q=<substring>` and `page=2`.
- Navigate to `/browse/perf/<an id>`. Type a query; press Enter. URL becomes `/browse/perf/<id>?q=<sub>`. Grid shows entity-scoped + search-filtered scenes.

- [ ] **Step 7: Commit**

```
git add internal/api/browse/data.go internal/api/browse/index.go internal/static/browse.gohtml
git commit -m "browse: add catalog search input with form-based submit and clear link"
```

---

## Task 3: Replace "Play in DeoVR" with inline `<video>` on scene detail

**Files:**
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/scene.go`
- Modify: `internal/static/browse_scene.gohtml`

- [ ] **Step 1: Swap `DeoVRPlayURL` for `DirectStreamURL` in `SceneDetailData`**

In `internal/api/browse/data.go`, the current `SceneDetailData` has:

```go
DeoVRPlayURL string
```

Replace that field with:

```go
DirectStreamURL string
```

The `StarSlice [5]struct{}` and other fields stay unchanged.

- [ ] **Step 2: Build the direct stream URL in `internal/api/browse/scene.go`**

The current handler builds the data struct around line 38:

```go
data := SceneDetailData{
	ID:           id,
	Title:        vd.Title(),
	BackURL:      backURL(r),
	DeoVRPlayURL: "/deovr/" + url.PathEscape(id),
	ErrMessage:   r.URL.Query().Get("err"),
}
```

Replace `DeoVRPlayURL` with `DirectStreamURL` built from `*vd.SceneParts.Paths.Stream` (this is what `stash.GetDirectStream` returns under the hood — see [internal/stash/stream.go:24-34](../../../internal/stash/stream.go#L24-L34)).

Defensive: `Paths.Stream` is a `*string` per the GraphQL schema; if nil, leave `DirectStreamURL` empty and the template will skip the `<video>` block. The struct literal becomes:

```go
data := SceneDetailData{
	ID:           id,
	Title:        vd.Title(),
	BackURL:      backURL(r),
	ErrMessage:   r.URL.Query().Get("err"),
}

if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Stream != nil {
	data.DirectStreamURL = *vd.SceneParts.Paths.Stream
}
```

(The existing `if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil` block stays; add the new `Stream` check separately rather than nesting them, for readability.)

The `"net/url"` import is no longer used in this file (it was only there for `url.PathEscape(id)` in the deleted `DeoVRPlayURL` line). Remove it from the import block:

```go
import (
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/static"
)
```

(Just delete the `"net/url"` line; `goimports` would do this automatically but we do it explicitly here.)

- [ ] **Step 3: Replace the Play button in `internal/static/browse_scene.gohtml`**

The current file ends (around line 105) with:

```html
<a class="play" href="{{.DeoVRPlayURL}}">▶  Play in DeoVR</a>
```

And has a corresponding CSS rule (around line 43–44):

```css
.play { display: block; text-align: center; background: #2c5282; color: #fff; padding: 14px; border-radius: 6px; text-decoration: none; font-weight: 600; margin-top: 24px; }
.play:hover { background: #3776c2; }
```

Remove BOTH the `.play` and `.play:hover` CSS rules from the `<style>` block.

Replace the `<a class="play" ...>` line near the end of `<body>` with a `<video>` element. But the better location is at the **top** of the `.wrap` div (above the title), per the spec's "video at top, metadata below" layout. So:

(a) Remove the `<a class="play" href="{{.DeoVRPlayURL}}">▶  Play in DeoVR</a>` line entirely.

(b) Find the existing thumb image line near the top:

```html
{{if .ThumbnailURL}}<img class="thumb" src="{{.ThumbnailURL}}" alt="">{{end}}
```

Replace it with a conditional that prefers the video, falls back to the thumbnail:

```html
{{if .DirectStreamURL}}
<video class="player" controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}"{{if .ThumbnailURL}} poster="{{.ThumbnailURL}}"{{end}}></video>
{{else if .ThumbnailURL}}
<img class="thumb" src="{{.ThumbnailURL}}" alt="">
{{end}}
```

(`poster` shows the thumbnail before the user hits play — small UX win since `autoplay muted` may still be blocked on some platforms.)

(c) Add a CSS rule for `.player` inside the `<style>` block (anywhere works; group near the `.thumb` rule):

```css
.player { width: 100%; max-height: 70vh; background: #000; border-radius: 6px; margin-bottom: 16px; }
```

The `.thumb` rule stays unchanged (still used as fallback).

- [ ] **Step 4: Verify build**

```
go vet ./...
go build ./...
```

Expected: no errors. (If `go vet` complains about unused `net/url` import, double-check Step 2 removed it.)

- [ ] **Step 5: Verify scene detail renders the video**

Restart stash-vr. Pick any real scene ID — open `http://localhost:9666/browse` and copy the ID from any scene's `data-` attribute or the URL when you click into it. Call this `<TEST_SCENE_ID>`.

```
curl -s 'http://localhost:9666/browse/scene/<TEST_SCENE_ID>' | findstr /C:"<video"
curl -s 'http://localhost:9666/browse/scene/<TEST_SCENE_ID>' | findstr /C:"Play in DeoVR"
```

Expected:
- First curl: one match showing the `<video class="player" ...>` line.
- Second curl: zero matches (the DeoVR button is gone).

Open `http://localhost:9666/browse/scene/<TEST_SCENE_ID>` in a desktop browser. Expected:
- Video element visible at the top of the page (above the title).
- Browser's native controls bar at the bottom of the video.
- Audio muted icon shown by default (because of `muted` attribute).
- Click play — audio plays once user unmutes (click the volume icon).
- Seek slider works (proves byte-range from Stash).
- All existing mutation buttons (rating stars, favorite, tags, O-counter, organized) still render and are clickable.

If the video shows a broken-image / "format not supported" indicator, check the `DirectStreamURL` value — it should look like `http://192.168.1.183:9999/scene/<id>/stream?apikey=...` or similar. If empty, `vd.SceneParts.Paths.Stream` was nil at handler time — investigate Stash's response.

- [ ] **Step 6: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go internal/static/browse_scene.gohtml
git commit -m "browse: replace 'Play in DeoVR' with inline <video> player on scene detail"
```

---

## Task 4: Remove the `▶` quickplay overlay from grid tiles

**Files:**
- Modify: `internal/api/browse/data.go`
- Modify: `internal/api/browse/grid.go`
- Modify: `internal/static/browse.gohtml`

- [ ] **Step 1: Remove `DeoVRPlayURL` field from `Card` in `internal/api/browse/data.go`**

The current `Card` struct ends with:

```go
type Card struct {
	ID           string
	Title        string
	ThumbnailURL string
	Duration     string
	Performers   string
	Studio       string
	DeoVRPlayURL string
	DetailURL    string
}
```

Remove `DeoVRPlayURL`:

```go
type Card struct {
	ID           string
	Title        string
	ThumbnailURL string
	Duration     string
	Performers   string
	Studio       string
	DetailURL    string
}
```

- [ ] **Step 2: Drop the `DeoVRPlayURL` assignment in `buildCards` in `internal/api/browse/grid.go`**

The current loop has (around line 58):

```go
c := Card{
	ID:           ids[i],
	Title:        vd.Title(),
	Duration:     formatDuration(vd.SceneParts.Files[0].Duration),
	DetailURL:    "/browse/scene/" + url.PathEscape(ids[i]),
	DeoVRPlayURL: "/deovr/" + url.PathEscape(ids[i]),
}
```

Remove the `DeoVRPlayURL` line:

```go
c := Card{
	ID:        ids[i],
	Title:     vd.Title(),
	Duration:  formatDuration(vd.SceneParts.Files[0].Duration),
	DetailURL: "/browse/scene/" + url.PathEscape(ids[i]),
}
```

(Also re-align the `:` columns for cleanliness, as shown.)

- [ ] **Step 3: Remove the `.quickplay` markup from `internal/static/browse.gohtml`**

Find the per-card block (around lines 60–73):

```html
{{range .Cards}}
<div class="card">
<div class="thumb-wrap">
<a class="thumb-link" href="{{.DetailURL}}">
<img class="thumb" src="{{.ThumbnailURL}}" alt="" loading="lazy">
<span class="duration">{{.Duration}}</span>
</a>
<a class="quickplay" href="{{.DeoVRPlayURL}}">▶</a>
</div>
<div class="body">
<p class="title">{{.Title}}</p>
<p class="meta">{{if .Performers}}{{.Performers}}{{end}}{{if and .Performers .Studio}} · {{end}}{{.Studio}}</p>
</div>
</div>
{{end}}
```

Delete the `<a class="quickplay" href="{{.DeoVRPlayURL}}">▶</a>` line. The block becomes:

```html
{{range .Cards}}
<div class="card">
<div class="thumb-wrap">
<a class="thumb-link" href="{{.DetailURL}}">
<img class="thumb" src="{{.ThumbnailURL}}" alt="" loading="lazy">
<span class="duration">{{.Duration}}</span>
</a>
</div>
<div class="body">
<p class="title">{{.Title}}</p>
<p class="meta">{{if .Performers}}{{.Performers}}{{end}}{{if and .Performers .Studio}} · {{end}}{{.Studio}}</p>
</div>
</div>
{{end}}
```

- [ ] **Step 4: Remove the `.quickplay` CSS rule from `internal/static/browse.gohtml`**

In the `<style>` block, find:

```css
.card .quickplay { position: absolute; bottom: 6px; left: 6px; background: rgba(0,0,0,0.7); padding: 2px 8px; border-radius: 3px; font-size: 0.9rem; text-decoration: none; color: #eee; }
```

Delete that single line.

- [ ] **Step 5: Verify build**

```
go vet ./...
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Verify the overlay is gone**

Restart stash-vr. Then:

```
curl -s 'http://localhost:9666/browse' | findstr /C:"quickplay"
curl -s 'http://localhost:9666/browse' | findstr /C:"DeoVRPlayURL"
```

Expected: both return zero matches.

Open `http://localhost:9666/browse` in a desktop browser. Expected:
- Tiles still show thumbnail + duration overlay (bottom-right).
- No `▶` button (bottom-left is empty).
- Click any tile → navigates to scene detail (the `.thumb-link` `<a>` is still wrapping the image).
- Pagination still works.
- Sidebar still works.

- [ ] **Step 7: Commit**

```
git add internal/api/browse/data.go internal/api/browse/grid.go internal/static/browse.gohtml
git commit -m "browse: remove DeoVR play overlay from grid tiles"
```

---

## Task 5: Final on-headset validation and writeup

**Files:**
- Create: `docs/superpowers/research/2026-05-08-m1-browse-result/checklist.md`
- Create: `docs/superpowers/research/2026-05-08-m1-browse-result/result.md`

This task validates the full M1 in the actual target environment (Quest 3 / Meta Browser) and produces an artifact that gates moving to M2.

- [ ] **Step 1: Create the checklist file at `docs/superpowers/research/2026-05-08-m1-browse-result/checklist.md`**

```markdown
# M1 /browse — Quest 3 Meta Browser checklist

**Run on:** Quest 3 hardware, Meta Browser (NOT DeoVR's in-VR browser; that's M2 territory and known not to support this stack).

**URL to open:** `https://stash-vr.duckdns.org/browse`

For each criterion: PASS / FAIL / PARTIAL + one-line note.

## Browse / search

- [ ] /browse loads. Cards visible. Thumbnails visible. Search input visible above the grid.
  - Result: ___ — note: ___

- [ ] Type a known title fragment, press Enter. URL shows `?q=<fragment>`. Grid filters.
  - Result: ___ — note: ___

- [ ] Click "Clear" → all scenes back.
  - Result: ___ — note: ___

- [ ] Click into a sidebar performer / studio / tag → entity-filtered grid loads.
  - Result: ___ — note: ___

- [ ] On entity-filtered route, type a query → grid scopes to entity + query.
  - Result: ___ — note: ___

- [ ] Pagination Next/Prev works AND preserves `?q=...` if present.
  - Result: ___ — note: ___

- [ ] No `▶` overlay on tiles (only thumbnail + duration).
  - Result: ___ — note: ___

## Scene detail / playback

- [ ] Click any scene tile → detail page loads.
  - Result: ___ — note: ___

- [ ] Video element visible at top of page. Player controls work.
  - Result: ___ — note: ___

- [ ] Click play (or unmute) — audio audible, frames visible.
  - Result: ___ — note: ___

- [ ] Drag the seek scrubber to a different position — playback resumes at the new time. (Validates byte-range.)
  - Result: ___ — note: ___

- [ ] No "Play in DeoVR" button anywhere on scene detail.
  - Result: ___ — note: ___

## Existing mutations regression

- [ ] Click a star → rating updates (visible after page reload).
  - Result: ___ — note: ___

- [ ] Toggle favorite → state persists.
  - Result: ___ — note: ___

- [ ] Add a tag via the input → tag appears as a chip.
  - Result: ___ — note: ___

- [ ] Remove a tag → chip disappears.
  - Result: ___ — note: ___

- [ ] O-counter +/- → number updates.
  - Result: ___ — note: ___

- [ ] Organized toggle → button state changes.
  - Result: ___ — note: ___

## Overall

- [ ] All checks PASS → proceed to M2 design.
- [ ] At least one FAIL → write up in result.md and surface to user.
```

- [ ] **Step 2: Create the result document stub at `docs/superpowers/research/2026-05-08-m1-browse-result/result.md`**

```markdown
# M1 /browse — result

**Date run:** ___
**Stash-vr commit:** ___ (run `git rev-parse --short HEAD`)
**Quest 3 firmware:** ___
**Meta Browser version:** ___
**Library size at time of run:** ___ scenes total, ___ tagged DOME or SBS

## Per-criterion results

Copy from `checklist.md` after running on the headset.

## Surprises / observations

(Free-form. Performance notes, autoplay-policy quirks, CORS issues, or anything else load-bearing for M2.)

___

## Recommendation

- [ ] All PASS → green-light M2 (WebXR VR player) design session.
- [ ] FAIL — re-spec needed because: ___

## Open M2 inputs from this milestone

(Things we learned during M1 that should inform M2 design — e.g., "video element supports HLS but not DASH on Meta Browser", "byte-range OK at 8K", "autoplay was blocked on first load until I clicked elsewhere on the page first".)

___
```

- [ ] **Step 3: Verify the files were created and have content**

```
findstr /C:"M1 /browse" docs/superpowers/research/2026-05-08-m1-browse-result/checklist.md
findstr /C:"Recommendation" docs/superpowers/research/2026-05-08-m1-browse-result/result.md
```

Expected: each command returns a matching line.

- [ ] **Step 4: Commit**

```
git add docs/superpowers/research/2026-05-08-m1-browse-result/checklist.md docs/superpowers/research/2026-05-08-m1-browse-result/result.md
git commit -m "browse: M1 validation checklist and result stub"
```

- [ ] **Step 5: HAND-OFF — user runs the checklist on Quest 3**

Stop here. Do NOT proceed to mark the entire plan complete.

Tell the user:

> "Implementation complete. Please run the checklist on your Quest 3 in Meta Browser at `https://stash-vr.duckdns.org/browse`, fill in `docs/superpowers/research/2026-05-08-m1-browse-result/result.md`, and tell me the overall outcome. If anything fails or surprises you, paste the result and we'll triage before moving to M2."

---

## Risk handling reminders (from spec § 9)

These come from the M1 spec. Surface to the user *before* changing scope:

- **`autoplay muted` blocked by Quest 3 / Meta Browser.** Acceptable degraded
  behavior — user clicks play once. No fallback needed unless user complains.
- **CORS on Stash direct stream from `stash-vr.duckdns.org`.** If browser
  console shows CORS errors, STOP. Do not add a `/stream/{id}` proxy in M1
  — escalate to user. Stash's CORS settings might solve it cleanly.
- **`FindFilterType.Q` field name.** Confirmed `Q *string` exists in
  [internal/stash/gql/generated.go:500](../../../internal/stash/gql/generated.go#L500).
  No adaptation needed.
- **Search performance slow.** Stash-side concern, not ours. Document in
  result.md and move on.
- **Sort vs. relevance ordering.** We pass an explicit `Sort = "created_at"`
  regardless of `Q`. If Stash overrides this with relevance ranking, that
  may surface as "search results in a weird order." Document if observed.

---

## Self-review (writer's check, run after writing each task above)

This is the writer's checklist — ignore if you're the executor.

1. **Spec coverage:**
   - § 2 success criterion 1 (tile click → detail) — Task 4 retains the `.thumb-link` so click navigation works.
   - § 2 success criterion 2 (video on detail) — Task 3.
   - § 2 success criterion 3 (catalog search) — Tasks 1 + 2.
   - § 2 success criterion 4 (empty q = no filter) — Task 1 Step 1's `if q != ""` gate.
   - § 2 success criterion 5 (entity-scoped search) — Task 1 Step 2 covers `entityHandler`.
   - § 2 success criterion 6 (DeoVR launch UI gone) — Tasks 3 + 4.
   - § 2 success criterion 7 (existing mutations work) — Task 5 checklist includes regression checks.
   - § 7 file list — exact match.
   - § 8 validation plan — Task 5 captures the Quest 3 / Meta Browser checklist.

2. **Placeholders:** None. Each step has actual code or a precise manual verification action. The few `___` blanks in the checklist/result files are intentional user-fill blanks, not plan placeholders.

3. **Type consistency:**
   - `SearchQuery` field used consistently in data.go + index.go + browse.gohtml.
   - `DirectStreamURL` field used consistently in data.go + scene.go + browse_scene.gohtml.
   - `q` query param name used consistently across handlers, template input, and pager-preserved URL.
   - The `qv` rename (URL `Values`) vs `searchQ` (search string) is consistent within both handlers.

4. **Ambiguity:** None significant. The CSS placement is "anywhere inside `<style>`" which is intentionally loose — the grouping suggestion is advisory, not load-bearing.
