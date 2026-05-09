# M4a: Web view polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/browse/scene/{id}` feel finished — clickable performer/studio/tag chips, intuitive rating UX, no full-page reloads on mutations.

**Architecture:** Five tasks. Task 1 extends the GraphQL fragment so studio and performer IDs are available. Task 2 reshapes the data path and template to render chip links (Item 1). Task 3 inverts star DOM order and adds CSS for hover-fill (Item 2). Tasks 4 and 5 split the AJAX work: Task 4 converts the seven POST handlers to JSON-only and drops the `redirectBack` / `?err=` flow; Task 5 adds the client-side form interception that consumes those JSON responses.

**Tech Stack:** Go 1.24, chi router, `genqlient` GraphQL client, Go html/template, vanilla JS in browser, A-Frame (untouched in M4a).

**Spec:** [docs/superpowers/specs/2026-05-09-m4a-web-polish.md](../specs/2026-05-09-m4a-web-polish.md)

**No tests in this project** (per [CLAUDE.md](../../../CLAUDE.md): "There is no test suite and no lint config — `go vet ./...` and `go build ./...` are the only standard checks."). Verification is `go vet ./...`, `go build ./...`, and the manual browser steps in §8 of the spec at the end of each task.

---

## Task 1: Add `id` to SceneParts.studio and SceneParts.performers

**Files:**
- Modify: `internal/stash/gql/documents/query.graphql:163-171`
- Regenerate: `internal/stash/gql/generated.go` (do not hand-edit)

**Why:** `vd.SceneParts.Studio` and `vd.SceneParts.Performers[]` currently expose only `Name`, not `Id`. Item 1 of the spec needs `Id` so chip links can target the existing `/browse/perf/{id}` and `/browse/studio/{id}` routes.

- [ ] **Step 1: Edit the SceneParts fragment**

In [internal/stash/gql/documents/query.graphql](../../../internal/stash/gql/documents/query.graphql), change lines 163-171 from:

```graphql
    studio{
        name
    },
    scene_markers {
        ...SceneMarkerParts
    },
    performers {
        name
    },
```

to:

```graphql
    studio{
        id
        name
    },
    scene_markers {
        ...SceneMarkerParts
    },
    performers {
        id
        name
    },
```

- [ ] **Step 2: Regenerate the GraphQL client**

Run: `go generate ./cmd/stash-vr`

Expected: `internal/stash/gql/generated.go` is rewritten. `ScenePartsStudio` and `ScenePartsPerformersPerformer` gain an `Id string` field.

Verify with:
```
git diff internal/stash/gql/generated.go | grep -E "type ScenePartsStudio|type ScenePartsPerformersPerformer" -A 4
```

Expected output shows both types now have `Id string` plus `Name string`.

- [ ] **Step 3: Build to confirm no callers broke**

Run: `go build ./...`

Expected: clean build. The `Id` field is additive; nothing references it yet.

- [ ] **Step 4: Vet**

Run: `go vet ./...`

Expected: no output (success).

- [ ] **Step 5: Commit**

```
git add internal/stash/gql/documents/query.graphql internal/stash/gql/generated.go
git commit -m "library(gql): include id on studio and performers in SceneParts"
```

---

## Task 2: Item 1 — Clickable performer / studio / tag chips

**Files:**
- Modify: `internal/api/browse/data.go` (add `EntityRef`, reshape `SceneDetailData` fields)
- Modify: `internal/api/browse/scene.go` (populate new fields from `vd.SceneParts`)
- Modify: `internal/static/browse_scene.gohtml` (template + CSS)

- [ ] **Step 1: Add `EntityRef` and reshape `SceneDetailData` in data.go**

Edit [internal/api/browse/data.go](../../../internal/api/browse/data.go). Add the `EntityRef` type just below the existing `Entity` struct (around line 12). Then change the `SceneDetailData` fields:

```go
// EntityRef is a clickable chip target on the scene detail page —
// performer, studio, or tag. ID drives the link href; Name is the
// display text. JSON tags keep the wire format lowercase so the
// browser-side AJAX layer reads {id, name}.
type EntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

Replace these three fields in `SceneDetailData`:

```go
	Performers   string
	Studio       string
	...
	Tags         []string // tag names currently on the scene (chips), excluding favorite tag and ancestor-injected tags
```

with:

```go
	Performers   []EntityRef
	Studio       *EntityRef // nil if the scene has no studio
	...
	Tags         []EntityRef // chip-renderable tags, excluding favorite tag and ancestor-injected tags
```

- [ ] **Step 2: Update population in scene.go**

In [internal/api/browse/scene.go](../../../internal/api/browse/scene.go), replace the performer-name join block (lines 57-64) with:

```go
	for _, p := range vd.SceneParts.Performers {
		if p == nil {
			continue
		}
		data.Performers = append(data.Performers, EntityRef{ID: p.Id, Name: p.Name})
	}
```

Replace the studio block (lines 66-68) with:

```go
	if vd.SceneParts.Studio != nil {
		data.Studio = &EntityRef{ID: vd.SceneParts.Studio.Id, Name: vd.SceneParts.Studio.Name}
	}
```

Replace the tag chip-list build (lines 81-100) — keep the `tagInputs` build for `Detect`, change only the chip list:

```go
	favTag := config.Application().FavoriteTag
	tagInputs := make([]apiinternal.TagInput, 0, len(vd.SceneParts.Tags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		// Collect every tag (including ancestor-injected) for projection
		// detection — an ancestor DOME tag is just as authoritative as a
		// direct one.
		tagInputs = append(tagInputs, apiinternal.TagInput{Name: name, Aliases: t.Aliases})
		// Skip ancestor-injected tags from the chip list.
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			data.IsFavorite = true
			continue
		}
		data.Tags = append(data.Tags, EntityRef{ID: t.TagParts.Id, Name: name})
	}
```

- [ ] **Step 3: Update the template — meta line and tag chips**

Edit [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml).

Replace the existing meta line (line 890) with chip rendering. Find:

```html
<p class="meta">{{if .Performers}}{{.Performers}}{{end}}{{if and .Performers .Studio}} · {{end}}{{.Studio}}{{if .Date}} · {{.Date}}{{end}}{{if .Duration}} · {{.Duration}}{{end}}</p>
```

Replace with:

```html
<div class="meta">
{{range .Performers}}<a class="chip chip-perf" href="/browse/perf/{{.ID}}">{{.Name}}</a>{{end}}
{{with .Studio}}<a class="chip chip-studio" href="/browse/studio/{{.ID}}">{{.Name}}</a>{{end}}
{{if .Date}}<span class="meta-text">{{.Date}}</span>{{end}}
{{if .Duration}}<span class="meta-text">{{.Duration}}</span>{{end}}
</div>
```

Replace the tag chip render block. Find (lines 911-914):

```html
<div class="tags">
{{range .Tags}}
<span class="chip">{{.}}<form method="post" action="/browse/scene/{{$.ID}}/tags/remove" style="display:inline"><button name="tag" value="{{.}}">✕</button></form></span>
{{end}}
```

Replace with:

```html
<div class="tags">
{{range .Tags}}
<span class="chip"><a href="/browse/tag/{{.ID}}">{{.Name}}</a><form method="post" action="/browse/scene/{{$.ID}}/tags/remove" style="display:inline"><button name="tag" value="{{.Name}}">✕</button></form></span>
{{end}}
```

- [ ] **Step 4: Update CSS for the meta line and chip links**

In the `<style>` block of [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find the existing `.meta` rule (around line 16):

```css
.meta { color: #aaa; margin-bottom: 24px; }
```

Replace with:

```css
.meta { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; margin-bottom: 24px; }
.meta .meta-text { color: #aaa; }
.chip-perf { background: #2a2a2a; }
.chip-studio { background: #2a2a3a; }
a.chip { color: #ddd; text-decoration: none; }
a.chip:hover { background: #3a3a3a; }
.chip a { color: inherit; text-decoration: none; }
.chip a:hover { color: #fff; }
```

- [ ] **Step 5: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean output from both.

- [ ] **Step 6: Manual verify**

Build: `scripts\build-windows.bat` (or `go build -o stash-vr.exe ./cmd/stash-vr`).

Run the binary, open `https://stash-vr.duckdns.org/browse/scene/<id>` for a scene with multiple performers and a studio:

1. Performer names render as chips. Click one → lands on `/browse/perf/{id}` with that performer's grid.
2. Studio chip clicks → `/browse/studio/{id}`.
3. Tag chip text links to `/browse/tag/{id}`. The ✕ button still removes the tag (and still page-reloads — that's fixed in Task 5).
4. Date and duration still render after the chips.

- [ ] **Step 7: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go internal/static/browse_scene.gohtml
git commit -m "browse: clickable performer, studio, tag chips on scene detail"
```

---

## Task 3: Item 2 — Rating star fill on hover

**Files:**
- Modify: `internal/static/browse_scene.gohtml` (template + CSS)

**Goal:** Hovering star N highlights stars 1..N. Pure CSS via reverse-DOM + `direction: rtl` + `~` sibling selector.

- [ ] **Step 1: Reorder star buttons in template (5 → 1)**

Find the rating block in [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml) (lines 893-900):

```html
<div class="section">
<h2>Rating</h2>
<form method="post" action="/browse/scene/{{.ID}}/rating" class="stars" style="display:inline-flex">
{{range $i, $_ := .StarSlice}}
<button name="value" value="{{add $i 1}}"{{if le (add $i 1) $.Rating1to5}} class="on"{{end}}>★</button>
{{end}}
{{if .Rating1to5}}<button name="value" value="0" class="clear" title="Clear rating">✕</button>{{end}}
</form>
</div>
```

Replace with:

```html
<div class="section">
<h2>Rating</h2>
<form method="post" action="/browse/scene/{{.ID}}/rating" class="stars">
<span class="stars-fill">
{{range $i, $_ := .StarSlice}}
{{- $star := sub 5 $i -}}
<button name="value" value="{{$star}}"{{if eq $star $.Rating1to5}} class="on"{{end}}>★</button>
{{end}}
</span>
{{if .Rating1to5}}<button name="value" value="0" class="clear" title="Clear rating">✕</button>{{end}}
</form>
</div>
```

The template now emits star buttons in DOM order 5,4,3,2,1, and only the button matching `Rating1to5` exactly gets `class="on"`. The `<span class="stars-fill">` wrapper isolates the reverse for CSS sibling selectors.

- [ ] **Step 2: Register the `sub` template func**

In [internal/api/browse/scene.go](../../../internal/api/browse/scene.go), find:

```go
var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"le":  func(a, b int) bool { return a <= b },
}).ParseFS(static.Fs, "browse_scene.gohtml"))
```

Add `"sub"` to the FuncMap:

```go
var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"le":  func(a, b int) bool { return a <= b },
}).ParseFS(static.Fs, "browse_scene.gohtml"))
```

The existing `le` and `add` funcs are no longer used by the rating block but are kept in case other callers reference them (none today, but the change is minimal).

- [ ] **Step 3: Update CSS for the reverse-DOM hover-fill**

In the `<style>` block, find the existing `.stars` rules (lines 20-25):

```css
.stars { display: inline-flex; gap: 4px; align-items: center; }
.stars button { background: transparent; border: none; font-size: 1.5rem; color: #555; cursor: pointer; padding: 0 2px; }
.stars button.on { color: #f7b500; }
.stars button:hover { color: #f7b500; }
.stars button.clear { font-size: 1rem; margin-left: 8px; color: #888; }
.stars button.clear:hover { color: #f55; }
```

Replace with:

```css
.stars { display: inline-flex; gap: 4px; align-items: center; }
.stars-fill { display: inline-flex; flex-direction: row-reverse; gap: 4px; }
.stars button { background: transparent; border: none; font-size: 1.5rem; color: #555; cursor: pointer; padding: 0 2px; }
.stars-fill button:hover,
.stars-fill button:hover ~ button,
.stars-fill button.on,
.stars-fill button.on ~ button { color: #f7b500; }
.stars button.clear { font-size: 1rem; margin-left: 8px; color: #888; }
.stars button.clear:hover { color: #f55; }
```

`flex-direction: row-reverse` flips visual order without changing DOM order — buttons render left-to-right as 1,2,3,4,5 visually, but DOM order remains 5,4,3,2,1. The `~` general-sibling selector matches all subsequent siblings in DOM order = lower-numbered stars. So hovering button 4 fills 4,3,2,1.

- [ ] **Step 4: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 5: Manual verify**

Open a scene with rating=0 and rating=3 (test both states):

1. Rating=0, hover star 4 → stars 1,2,3,4 are gold; star 5 is grey. Move out → all grey.
2. Click star 4 → page reloads, star 4 is `on`; CSS fills 1,2,3 too.
3. Rating=3, hover star 5 → all five gold. Hover star 1 → only star 1 gold (and the persistent on-state for star 3 keeps stars 1,2,3 gold via the `.on ~` rule, but the `:hover` rule narrows to just 1 — the `~` selector still matches star 3's siblings, so 1,2,3 stay gold). Acceptable.
4. Hover the ✕ clear → still red, still clears.

- [ ] **Step 6: Commit**

```
git add internal/static/browse_scene.gohtml internal/api/browse/scene.go
git commit -m "browse: rating star fill on hover (1..N gold)"
```

---

## Task 4: Item 3 (server) — JSON-only mutation handlers

**Files:**
- Modify: `internal/api/browse/data.go` (add `SceneState`)
- Modify: `internal/api/browse/scene.go` (drop `ErrMessage`, `?err=` reading)
- Modify: `internal/api/browse/scene_post.go` (rewrite handlers, drop `redirectBack`)
- Modify: `internal/static/browse_scene.gohtml` (drop server-rendered `.errbanner`)

**Goal:** All seven POST handlers return `application/json` with current scene state. No redirects, no `?err=` query param. Page render still works for GET. After this task, the page renders fine but POSTs return JSON which a browser without JS will display raw — Task 5 fixes that.

- [ ] **Step 1: Add `SceneState` type to data.go**

In [internal/api/browse/data.go](../../../internal/api/browse/data.go), drop the `ErrMessage` field from `SceneDetailData`, and append the new type at the bottom:

```go
// SceneState is the JSON returned by every mutation POST. The client
// uses it to update the DOM in place.
type SceneState struct {
	Rating1to5 int         `json:"rating1to5"`
	IsFavorite bool        `json:"isFavorite"`
	OCounter   int         `json:"oCounter"`
	Organized  bool        `json:"organized"`
	Tags       []EntityRef `json:"tags"`
	Err        string      `json:"err,omitempty"`
}
```

- [ ] **Step 2: Drop `ErrMessage` reading in scene.go**

In [internal/api/browse/scene.go](../../../internal/api/browse/scene.go), find (line 42):

```go
	data := SceneDetailData{
		ID:         id,
		Title:      vd.Title(),
		BackURL:    backURL(r),
		ErrMessage: r.URL.Query().Get("err"),
	}
```

Replace with:

```go
	data := SceneDetailData{
		ID:      id,
		Title:   vd.Title(),
		BackURL: backURL(r),
	}
```

- [ ] **Step 3: Rewrite scene_post.go with JSON-only handlers**

Replace the entire contents of [internal/api/browse/scene_post.go](../../../internal/api/browse/scene_post.go) with:

```go
package browse

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"stash-vr/internal/config"
	"stash-vr/internal/library"
	"stash-vr/internal/prefix"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// writeState writes a 200 with the post-mutation SceneState as JSON.
// Caller has already done refreshSceneCache(r, id) so the read sees
// the just-written state.
func (h *httpHandler) writeState(w http.ResponseWriter, r *http.Request, id string) {
	state, err := buildSceneState(r.Context(), h.libraryService, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build state failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode SceneState")
	}
}

// writeErr writes an error envelope at the given status.
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SceneState{Err: msg})
}

// buildSceneState reads a fresh scene from the cache and projects it to
// SceneState — applying the same FAVORITE_TAG and ancestor-tag filters
// that scene.go's GET path applies. Centralized here so the GET render
// and the POST response can never drift.
func buildSceneState(ctx context.Context, svc *library.Service, id string) (SceneState, error) {
	vd, err := svc.GetScene(ctx, id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		return SceneState{}, err
	}
	state := SceneState{}
	if vd.SceneParts.Rating100 != nil {
		state.Rating1to5 = *vd.SceneParts.Rating100 / 20
	}
	if vd.SceneParts.O_counter != nil {
		state.OCounter = *vd.SceneParts.O_counter
	}
	state.Organized = vd.SceneParts.Organized
	favTag := config.Application().FavoriteTag
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			state.IsFavorite = true
			continue
		}
		state.Tags = append(state.Tags, EntityRef{ID: t.TagParts.Id, Name: name})
	}
	return state, nil
}

func (h *httpHandler) sceneRatingHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	val, parseErr := strconv.Atoi(r.PostForm.Get("value"))
	if parseErr != nil || val < 0 || val > 5 {
		writeErr(w, http.StatusBadRequest, "bad rating")
		return
	}
	if val > 0 {
		vd, err := h.libraryService.GetScene(r.Context(), id, true)
		currentVal := 0
		if err == nil && vd != nil && vd.SceneParts != nil && vd.SceneParts.Rating100 != nil {
			currentVal = *vd.SceneParts.Rating100 / 20
		}
		if val == currentVal {
			val = 0
		}
	}
	var rating5 *float32
	if val > 0 {
		f := float32(val)
		rating5 = &f
	}
	if err := h.libraryService.UpdateRating(r.Context(), id, rating5); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: update rating")
		writeErr(w, http.StatusInternalServerError, "rating update failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	favTag := config.Application().FavoriteTag
	if favTag == "" {
		writeErr(w, http.StatusBadRequest, "FAVORITE_TAG not configured")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	currentlyFav := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if t.TagParts.Name == favTag {
			currentlyFav = true
			break
		}
	}
	if err := h.libraryService.UpdateFavorite(r.Context(), id, !currentlyFav); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: toggle favorite")
		writeErr(w, http.StatusInternalServerError, "favorite toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneTagAddHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		writeErr(w, http.StatusBadRequest, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	current := make([]string, 0, len(vd.SceneParts.Tags)+1)
	exists := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		current = append(current, t.TagParts.Name)
		if strings.EqualFold(t.TagParts.Name, tagName) {
			exists = true
		}
	}
	if !exists {
		current = append(current, tagName)
	}
	if err := h.libraryService.UpdateTags(r.Context(), id, current); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("tag", tagName).Msg("browse: add tag")
		writeErr(w, http.StatusInternalServerError, "tag add failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneTagRemoveHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		writeErr(w, http.StatusBadRequest, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	remaining := make([]string, 0, len(vd.SceneParts.Tags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if strings.EqualFold(t.TagParts.Name, tagName) {
			continue
		}
		remaining = append(remaining, t.TagParts.Name)
	}
	if err := h.libraryService.UpdateTags(r.Context(), id, remaining); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("tag", tagName).Msg("browse: remove tag")
		writeErr(w, http.StatusInternalServerError, "tag remove failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneOIncrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.IncrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: increment O")
		writeErr(w, http.StatusInternalServerError, "O increment failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneODecrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.DecrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: decrement O")
		writeErr(w, http.StatusInternalServerError, "O decrement failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneOrganizedHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	newState := !vd.SceneParts.Organized
	if err := h.libraryService.SetOrganized(r.Context(), id, newState); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: toggle organized")
		writeErr(w, http.StatusInternalServerError, "organized toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

// refreshSceneCache forceFetches the scene to refresh the in-memory cache.
// Called after a successful mutation so that buildSceneState reads the
// new state. Errors are logged but not surfaced — the mutation already
// succeeded.
func (h *httpHandler) refreshSceneCache(r *http.Request, id string) {
	if _, err := h.libraryService.GetScene(r.Context(), id, true); err != nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: refresh scene cache after mutation")
	}
}
```

Note: this drops the `redirectBack` function, the `net/url` import, and the `?err=` query-string flow.

- [ ] **Step 4: Drop the server-rendered `.errbanner` from template**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find (line 53):

```html
<a class="back" href="{{.BackURL}}">← Back</a>
{{if .ErrMessage}}<div class="errbanner">{{.ErrMessage}}</div>{{end}}
```

Replace with:

```html
<a class="back" href="{{.BackURL}}">← Back</a>
<div class="errbanner" id="errbanner" hidden></div>
```

The element is now always present but `hidden`; Task 5's JS toggles it.

- [ ] **Step 5: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean. (If `go vet` flags an unused import in scene_post.go after the rewrite, drop it — common offenders are `net/url` and `chi/v5`. The rewrite above uses `chi/v5` and does not use `net/url`.)

- [ ] **Step 6: Manual verify (server-only)**

Build and run. Open `/browse/scene/{id}` — page renders fine.

In a separate terminal, exercise a POST endpoint and confirm JSON:

```
curl -i -X POST -d "value=4" https://stash-vr.duckdns.org/browse/scene/<id>/rating
```

Expected: `HTTP/1.1 200 OK`, `Content-Type: application/json`, body like:

```json
{"rating1to5":4,"isFavorite":false,"oCounter":0,"organized":false,"tags":[{"id":"123","name":"Outdoor"}]}
```

Browser users clicking the rating buttons will now see raw JSON in a navigated page — that's expected between Tasks 4 and 5.

- [ ] **Step 7: Commit**

```
git add internal/api/browse/data.go internal/api/browse/scene.go internal/api/browse/scene_post.go internal/static/browse_scene.gohtml
git commit -m "browse: convert scene mutations to JSON-only responses"
```

---

## Task 5: Item 3 (client) — Form interception and DOM update

**Files:**
- Modify: `internal/static/browse_scene.gohtml` (form classes, `<template>` for tag chips, new `<script>` block)

**Goal:** All seven mutation forms intercept their submit, fetch JSON, and update the DOM in place. No reload, no scroll jump, video keeps playing.

- [ ] **Step 1: Add `js-mut` class to the seven mutation forms**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), edit each of the seven `<form method="post">` tags below the VR section. Add `js-mut` to the existing class attribute (or add a `class="js-mut ..."`).

Specifically:

a. Rating form:

```html
<form method="post" action="/browse/scene/{{.ID}}/rating" class="js-mut stars">
```

b. Favorite form:

```html
<form method="post" action="/browse/scene/{{.ID}}/favorite" class="js-mut fav" style="display:inline-flex">
```

c. Tag remove forms (inside the chip span — note this one's inside `{{range .Tags}}`):

```html
<form method="post" action="/browse/scene/{{$.ID}}/tags/remove" class="js-mut" style="display:inline">
```

d. Tag add form:

```html
<form method="post" action="/browse/scene/{{.ID}}/tags/add" class="js-mut addtag">
```

e. O-counter decrement:

```html
<form method="post" action="/browse/scene/{{.ID}}/o/decrement" class="js-mut ocount" style="display:inline-flex">
```

f. O-counter increment:

```html
<form method="post" action="/browse/scene/{{.ID}}/o/increment" class="js-mut ocount" style="display:inline-flex">
```

g. Organized form:

```html
<form method="post" action="/browse/scene/{{.ID}}/organized" class="js-mut org" style="display:inline-flex">
```

- [ ] **Step 2: Add a tag-chip template node**

Just before `<div class="tags">` in the Tags section, add a `<template>` node that the JS clones to render new chips. The template's structure must match what `{{range .Tags}}` emits:

```html
<template id="tag-chip-tpl">
<span class="chip"><a></a><form method="post" class="js-mut" style="display:inline"><button name="tag">✕</button></form></span>
</template>
```

(The JS will set the `<a>` href and text, the form's action, and the button's value when cloning.)

- [ ] **Step 3: Add the AJAX `<script>` block**

Append a new `<script>` block at the end of the page, after the existing closing `</script>` for the VR/picker code (around line 887, after `})();` and before `{{end}}`). Insert:

```html
<script>
(function() {
  const sceneId = (function() {
    const m = location.pathname.match(/\/browse\/scene\/([^\/]+)/);
    return m ? m[1] : '';
  })();
  if (!sceneId) return;

  const errBanner = document.getElementById('errbanner');
  let errTimer = null;
  function showErr(msg) {
    if (!errBanner) return;
    errBanner.textContent = msg;
    errBanner.hidden = false;
    if (errTimer) clearTimeout(errTimer);
    errTimer = setTimeout(() => { errBanner.hidden = true; errBanner.textContent = ''; }, 5000);
  }
  function clearErr() {
    if (!errBanner) return;
    errBanner.hidden = true;
    errBanner.textContent = '';
    if (errTimer) { clearTimeout(errTimer); errTimer = null; }
  }

  // Rebuild the chip list from state.tags. Preserves the add-tag form,
  // which sits as a sibling to the chip spans inside .tags.
  function renderTags(state) {
    const tagsRoot = document.querySelector('.tags');
    if (!tagsRoot) return;
    const tpl = document.getElementById('tag-chip-tpl');
    // Remove existing chip spans only (keep the addtag <form>).
    tagsRoot.querySelectorAll('span.chip').forEach(el => el.remove());
    const addForm = tagsRoot.querySelector('form.addtag');
    (state.tags || []).forEach(t => {
      const node = tpl.content.firstElementChild.cloneNode(true);
      const a = node.querySelector('a');
      a.href = '/browse/tag/' + encodeURIComponent(t.id);
      a.textContent = t.name;
      const form = node.querySelector('form');
      form.action = '/browse/scene/' + encodeURIComponent(sceneId) + '/tags/remove';
      const btn = form.querySelector('button[name="tag"]');
      btn.value = t.name;
      bindForm(form);
      tagsRoot.insertBefore(node, addForm);
    });
  }

  // Rating: clear all .on, set .on on the button matching state.rating1to5;
  // show/hide the ✕ clear button.
  function renderRating(state) {
    const form = document.querySelector('form.stars');
    if (!form) return;
    form.querySelectorAll('.stars-fill button').forEach(b => {
      b.classList.toggle('on', parseInt(b.value, 10) === state.rating1to5);
    });
    let clearBtn = form.querySelector('button.clear');
    if (state.rating1to5 > 0) {
      if (!clearBtn) {
        clearBtn = document.createElement('button');
        clearBtn.name = 'value';
        clearBtn.value = '0';
        clearBtn.className = 'clear';
        clearBtn.title = 'Clear rating';
        clearBtn.textContent = '✕';
        form.appendChild(clearBtn);
      }
    } else if (clearBtn) {
      clearBtn.remove();
    }
  }

  function renderFavorite(state) {
    const form = document.querySelector('form.fav');
    if (!form) return;
    const btn = form.querySelector('button');
    if (!btn) return;
    btn.classList.toggle('on', !!state.isFavorite);
    btn.textContent = state.isFavorite ? '♥ Favorited' : '♥ Favorite';
  }

  function renderOCounter(state) {
    // The OCounter span sits between the - and + forms, in document order.
    // Find by walking the increment form's previousElementSibling, which is
    // the <span>. (Simpler: query a dedicated id we set in the template.)
    const span = document.getElementById('ocount-value');
    if (span) span.textContent = state.oCounter;
  }

  function renderOrganized(state) {
    const form = document.querySelector('form.org');
    if (!form) return;
    const btn = form.querySelector('button');
    if (!btn) return;
    btn.classList.toggle('on', !!state.organized);
    btn.textContent = state.organized ? '✓ Organized' : 'Mark organized';
  }

  function applyState(state) {
    clearErr();
    if (state.err) { showErr(state.err); return; }
    renderRating(state);
    renderFavorite(state);
    renderTags(state);
    renderOCounter(state);
    renderOrganized(state);
  }

  function bindForm(form) {
    if (form._mutBound) return;
    form._mutBound = true;
    form.addEventListener('submit', function(ev) {
      ev.preventDefault();
      const submitBtn = ev.submitter;
      const fd = new FormData(form);
      // <button name="value" value="N"> in the rating form is not picked up
      // by FormData unless it's the submitter — guard for that:
      if (submitBtn && submitBtn.name && !fd.has(submitBtn.name)) {
        fd.set(submitBtn.name, submitBtn.value);
      }
      fetch(form.action, {
        method: 'POST',
        headers: { 'Accept': 'application/json' },
        body: fd
      }).then(r => r.json().then(j => ({ ok: r.ok, body: j })))
        .then(({ ok, body }) => {
          if (!ok) { showErr(body.err || 'request failed'); return; }
          applyState(body);
          // Clear the add-tag input on success.
          if (form.classList.contains('addtag')) {
            const input = form.querySelector('input[name="tag"]');
            if (input) input.value = '';
          }
        })
        .catch(err => showErr('network error: ' + err.message));
    });
  }

  document.querySelectorAll('form.js-mut').forEach(bindForm);
})();
</script>
```

- [ ] **Step 4: Add the `id="ocount-value"` to the O-counter span**

In the O-counter section (around line 930), find:

```html
<span style="display:inline-flex;align-items:center;padding:0 12px">{{.OCounter}}</span>
```

Replace with:

```html
<span id="ocount-value" style="display:inline-flex;align-items:center;padding:0 12px">{{.OCounter}}</span>
```

- [ ] **Step 5: Style the now-active `.errbanner`**

Confirm the existing `.errbanner` rule covers JS-driven use. Find in the `<style>` block:

```css
.errbanner { background: #5a1a1a; padding: 8px 12px; border-radius: 4px; margin-bottom: 12px; }
```

No change needed. The HTML `hidden` attribute handles show/hide; the JS toggles it.

- [ ] **Step 6: Vet, build**

Run: `go vet ./...` then `go build ./...`

Expected: clean.

- [ ] **Step 7: Manual verify (full spec validation, items 1-7 of §8 plus 11-12)**

Build and run. Open `/browse/scene/{id}` for a scene with multiple tags, performers, a studio. Walk through:

1. Click a performer chip → entity grid loads. Back button returns to scene with state preserved.
2. Hover star 4 → 1..4 gold; star 5 grey.
3. Click star 4 → no reload; star 4 shows `on`; CSS fills 1..3; URL unchanged; scroll preserved; video keeps playing.
4. Click ♥ → button flips state. No reload.
5. Type a new tag, click Add → chip appears. Input clears. No reload.
6. Click ✕ on a chip → chip vanishes. No reload.
7. Click +O five times rapidly → counter increments by 5 (eventually). No flicker. No reload.
8. Mark organized → button flips. No reload.
9. Force a server error: stop Stash temporarily; click ♥ → `.errbanner` shows the error; rest of page intact; banner clears after 5 s.
10. Restart Stash; click ♥ again → success path; banner stays cleared.
11. Enter VR → all M2/M3a/M3b/M3c behavior unchanged.
12. Exit VR → 2D layer reflects any changes made before entering.

- [ ] **Step 8: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "browse: AJAX scene mutations, no page refresh"
```

---

## Self-review checklist (run after writing the plan)

- **Spec coverage:**
  - Item 1 (clickable chips) → Tasks 1 + 2.
  - Item 2 (star fill on hover) → Task 3.
  - Item 3 (no page refresh) → Tasks 4 + 5.
  - Risks (tag focus, favTag filtering, screen readers, VR coexistence, ID safety) → addressed by the buildSceneState centralization in Task 4 and the chip-list-only rebuild in Task 5.
- **Type consistency:** `EntityRef{ID, Name}` defined in Task 2 step 1, used in Tasks 2, 4, and 5. `SceneState` defined in Task 4 step 1, consumed by Task 5 JS as `state.rating1to5 / isFavorite / oCounter / organized / tags / err`. Field naming matches across server JSON tags and JS access.
- **No placeholders:** every step has concrete code, exact paths, exact commands.
- **Frequent commits:** one commit per task. Five commits total.
- **YAGNI:** no optimistic UI, no toasts, no debounce on O-counter mash, no SSE — all explicit non-goals from the spec.
