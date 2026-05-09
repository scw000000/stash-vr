# M4a design: web view polish (scene detail)

**Date:** 2026-05-09
**Status:** Drafting (brainstorming approved 2026-05-09).
**Predecessors:** [M3c SKYBOX-style controller mappings](2026-05-08-m3c-skybox-controller-mappings.md). M3c shipped the in-VR controls; M4a is the first slice of M4 (UI polish bucket).
**Successors:** **M4b** — VR control panel polish (real scrub bar with drag, current-time display, subtitle on/off). **M4c** — in-VR search/browse: button on the panel summons the main scene grid in 3D space so the user can pick a new scene without exiting VR. M4a/M4b/M4c each get their own spec/plan/implementation cycle.

---

## 1. Context (why this milestone)

`/browse/scene/{id}` is the 2D detail page rendered by [internal/api/browse/scene.go](../../../internal/api/browse/scene.go) and [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml). It currently has three friction points:

1. Performer / studio names render as plain text. Tag chips only carry a remove button. No way to click "AliceBob" and see all of Alice's scenes — even though the routes `/browse/perf/{id}`, `/browse/studio/{id}`, `/browse/tag/{id}` already exist (`entityHandler` in [internal/api/browse/index.go:76-168](../../../internal/api/browse/index.go#L76-L168)). The detail page just doesn't link to them.
2. Hovering the 4th rating star highlights only that star, not stars 1–4. Standard rating-UX expectation is a fill from left to N.
3. Every mutation (favorite, rating, ±O-counter, organized, tag add/remove) is a `<form method="post">` that 303-redirects back to the scene page. The browser scrolls to top, the `<video>` reloads, the user loses position. Unusable as a fast tagging surface.

M4a fixes all three. M4b and M4c — the VR-side polish — come after.

## 2. Goal & non-goals

**Goal:** From `/browse/scene/{id}`, the user can navigate to filtered grids by clicking entity names, rate intuitively with hover-fill stars, and toggle every other piece of metadata without ever reloading the page.

**Success criteria, manually verified on Quest 3 Meta Browser + a desktop browser:**

1. Click a performer name → lands on `/browse/perf/{id}` with the filtered grid.
2. Click the studio name → lands on `/browse/studio/{id}`.
3. Click a tag chip's text → lands on `/browse/tag/{id}`. The chip's ✕ button still removes the tag.
4. Hover star N (N=1..5) → stars 1..N all gold, stars N+1..5 grey.
5. Click rating star, toggle favorite, add tag, remove tag, ±O-counter, mark organized → DOM updates in place. No page reload. No scroll jump. `<video>` keeps playing without re-buffering.
6. On any mutation error → an inline error message shows; the rest of the page is unaffected.
7. M3a/M3b/M3c regressions absent: VR mode still enters, projection picker still works, controller mappings still fire.

**Non-goals (deferred):**

- VR control panel polish (M4b). M4a touches no VR-side code.
- In-VR search summon (M4c).
- AJAX on the `/browse` index/grid page. The grid has no mutation buttons; nothing to AJAX-ify there.
- No-JS graceful degradation. JS is required after M4a (Meta Browser and desktop both have JS; we're not targeting curl).
- Rating with half-stars or 10-point scales. Stars stay 1–5.
- Optimistic updates. Every DOM update waits on server response — keeps client and server in lockstep.
- Undo / toast confirmations. Plain success = silent DOM update; failure = inline error.

## 3. Item 1 — Clickable performer / studio / tag chips

### 3.1 Data shape changes

Today [internal/api/browse/data.go](../../../internal/api/browse/data.go) carries:

```go
type SceneDetailData struct {
    Performers string         // "Alice, Bob"
    Studio     string         // "StudioX"
    Tags       []string       // ["Outdoor", "POV"]
    // ...
}
```

After M4a:

```go
type EntityRef struct {
    ID   string
    Name string
}

type SceneDetailData struct {
    Performers []EntityRef
    Studio     *EntityRef    // nil if no studio
    Tags       []EntityRef
    // ...
}
```

`scene.go` populates these from `vd.SceneParts.Performers[].ID/.Name`, `vd.SceneParts.Studio.ID/.Name`, and `vd.SceneParts.Tags[].TagParts.Id/.Name`. All three IDs are already present on `SceneParts` — this is a pure reshape, no genqlient regen.

### 3.2 Template changes

In [browse_scene.gohtml](../../../internal/static/browse_scene.gohtml):

- The current meta line `{{.Performers}} · {{.Studio}} · {{.Date}}` becomes a flex row of small chips for performers and studio, followed by date / duration as plain dim text.
- Each performer chip: `<a class="chip" href="/browse/perf/{{.ID}}">{{.Name}}</a>`.
- Studio chip: `<a class="chip studio" href="/browse/studio/{{.ID}}">{{.Name}}</a>` — same `.chip` style, optional accent color via the `.studio` modifier.
- Tag chip text becomes a link: `<a href="/browse/tag/{{.ID}}">{{.Name}}</a>` inside the existing `<span class="chip">`. The ✕ remove form sits beside it, unchanged in behavior.

### 3.3 Visual contract

Anything that *looks* like a chip is clickable. The performer/studio chips inherit the existing `.chip` styling (rounded, dark-grey background); the tag chip's clickable region is the name text only — the ✕ button stays visually distinct. Hover state on chip names: lighter background (`#3a3a3a`) so they read as interactive.

## 4. Item 2 — Rating star fill on hover

Pure CSS, no JS.

### 4.1 DOM reorder

Today the rating form renders 5 buttons in numeric order, button 1 first:

```html
<form class="stars">
  <button value="1">★</button>
  <button value="2">★</button>
  ...
  <button value="5">★</button>
  <button value="0" class="clear">✕</button>
</form>
```

After M4a: render the 5 star buttons in **reverse** numeric order (5 first, 1 last) inside a wrapper with `direction: rtl`. Visually the user still sees stars 1..5 left to right, but the DOM order is 5..1. The `✕` clear button stays appended after the wrapper.

### 4.2 CSS rule

```css
.stars-fill { display: inline-flex; direction: rtl; gap: 4px; }
.stars-fill button:hover,
.stars-fill button:hover ~ button { color: #f7b500; }
.stars-fill button.on,
.stars-fill button.on ~ button { color: #f7b500; }
```

The `~` general-sibling selector matches all subsequent siblings in DOM order — which, after the reverse, are the lower-numbered stars. So hovering button 4 fills stars 4, 3, 2, 1.

The `.on` rule mirrors the same trick for the persistent state: server still emits `class="on"` for the highest currently-set star (button N), and `~ button` fills 1..N-1.

### 4.3 Server change

`scene.go` currently emits `class="on"` for every star ≤ rating. After M4a, only star N (the highest set) gets `class="on"` — CSS handles the fill of 1..N-1 from there. Reduces server template logic by one comparison.

## 5. Item 3 — AJAX mutations (JSON-only)

### 5.1 Server: convert seven handlers to JSON

Seven existing POST handlers in [scene_post.go](../../../internal/api/browse/scene_post.go) are converted to return JSON. The dual-path 303-redirect fallback is removed.

Affected handlers:

| Handler | Path |
|---|---|
| `sceneRatingHandler` | POST /browse/scene/{id}/rating |
| `sceneFavoriteHandler` | POST /browse/scene/{id}/favorite |
| `sceneTagAddHandler` | POST /browse/scene/{id}/tags/add |
| `sceneTagRemoveHandler` | POST /browse/scene/{id}/tags/remove |
| `sceneOIncrementHandler` | POST /browse/scene/{id}/o/increment |
| `sceneODecrementHandler` | POST /browse/scene/{id}/o/decrement |
| `sceneOrganizedHandler` | POST /browse/scene/{id}/organized |

Each handler:

1. Performs its mutation as today.
2. Calls `refreshSceneCache` as today.
3. Reads the post-mutation scene state.
4. Writes a `SceneState` JSON response with `Content-Type: application/json`.

```go
type SceneState struct {
    Rating1to5 int         `json:"rating1to5"`
    IsFavorite bool        `json:"isFavorite"`
    OCounter   int         `json:"oCounter"`
    Organized  bool        `json:"organized"`
    Tags       []EntityRef `json:"tags"`        // chip-renderable, FAVORITE_TAG filtered out
    Err        string      `json:"err,omitempty"`
}
```

A shared helper `buildSceneState(ctx, libraryService, id) (SceneState, error)` lives in `scene_post.go` and is reused by all seven handlers. It reads the scene from the cache and applies the same FAVORITE_TAG filtering and ancestor-tag skipping that [scene.go:80-100](../../../internal/api/browse/scene.go#L80-L100) does for the page render.

Status codes:

- Success → 200 OK, full `SceneState`, `err: ""`.
- Validation failure (bad form, empty tag, FAVORITE_TAG unset) → 400 Bad Request, body `{"err":"<msg>"}`.
- Server failure (Stash unreachable, mutation failed) → 500, body `{"err":"<msg>"}`.

### 5.2 Removed code

- `redirectBack` function in scene_post.go.
- `?err=` query param handling in `sceneDetailHandler` ([scene.go:42](../../../internal/api/browse/scene.go#L42) — `data.ErrMessage = r.URL.Query().Get("err")`).
- `.errbanner` server-side conditional render in [browse_scene.gohtml:53](../../../internal/static/browse_scene.gohtml#L53) (`{{if .ErrMessage}}<div class="errbanner">{{.ErrMessage}}</div>{{end}}`). Replaced by a JS-driven banner that the AJAX layer populates on error.

### 5.3 Client: form interception + DOM update

A new `<script>` block in `browse_scene.gohtml` (or a separate `/browse-scene.js` served from `static.Fs` for cleanliness — TBD in plan) does the following on page load:

1. Selects every form with class `js-mut` (added to all seven mutation forms in the template).
2. Attaches a `submit` handler that:
   - `event.preventDefault()`.
   - Builds `FormData` from the submitted form.
   - `fetch(form.action, { method: 'POST', headers: { 'Accept': 'application/json' }, body: formData })`.
   - On 200 → call `applySceneState(json)` to mutate the DOM.
   - On non-2xx → render `json.err` in the `.errbanner` element; clear after 5 s or on next successful mutation.
   - On network error → render `"network error"` in `.errbanner`.
3. `applySceneState(state)` re-renders four sections from current DOM nodes:
   - **Rating:** clear `.on` from all star buttons; if `state.rating1to5 > 0`, add `.on` to the matching button. Show/hide the `✕` clear button.
   - **Favorite:** toggle `.on` class on the favorite button; update its label text.
   - **Tags:** rebuild the chip list inside `.tags` from `state.tags` (chip nodes cloned from a hidden template `<template id="tag-chip-tpl">`). The "Add tag" form is preserved — only the chip span list is replaced.
   - **O-counter:** update the `<span>` count.
   - **Organized:** toggle `.on` class and label.

No framework, no SPA. Vanilla DOM. The only client-side state is what's in the DOM; every mutation goes server→DOM via the `SceneState` response.

### 5.4 Concurrency

If the user mashes the +O button five times rapidly, each click fires a separate POST. Server-side, the underlying `IncrementO` GraphQL mutation is naturally serialized through Stash, so the server-truth `OCounter` always reflects all completed increments. Client renders whichever response arrives last. No coalescing logic needed — unlike the projection picker (which has a single state to converge on), the O-counter is monotonic.

For rating/favorite/organized: the user can't realistically mash these. If they do, last response wins. Acceptable.

## 6. Files touched

```
internal/api/browse/data.go         # EntityRef, SceneState; reshape SceneDetailData
internal/api/browse/scene.go        # populate new fields; drop ErrMessage from data
internal/api/browse/scene_post.go   # JSON responses, buildSceneState helper, drop redirectBack
internal/static/browse_scene.gohtml # chip markup, star reorder, AJAX <script> block,
                                    # tag-chip <template>, drop server .errbanner conditional
internal/static/browse_scene.css    # (optional) extract growing styles to a separate file —
                                    # decide in plan
```

No new routes. No genqlient regen. No changes to `entityHandler`, `indexHandler`, sidebar, grid, or any VR code path.

## 7. Risks

- **Tag list rebuild loses input focus.** If the user is mid-typing in "Add tag…" when a different mutation completes (e.g., they checked "Favorite" via keyboard), the tag-chip rebuild shouldn't blow away the input. **Mitigation:** rebuild only the chip span list, not the form. The form (with its input + datalist) sits as a separate sibling — already the case in the current template.
- **Optimistic UX feel under high latency.** Every mutation waits on round-trip. On slow networks (Quest 3 over LAN should be fine), the user sees a moment of unresponsiveness. We accept this for v1; an optimistic-first variant is a possible follow-up if it bothers anyone.
- **CSS reverse-DOM star approach + screen readers.** The visual order mismatches DOM order. Screen readers will announce the buttons in DOM order (5, 4, 3, 2, 1). Acceptable — this is a single-user VR-adjacent UI, accessibility is not a current target.
- **`favTag` filtering in `SceneState.Tags`.** Server must apply the same FAVORITE_TAG filter and ancestor-tag skip in `buildSceneState` as the page-render path. Otherwise the favorite-tag would appear as a regular chip after a mutation. Centralizing this in the helper avoids drift.
- **VR overlay (`<a-scene>`) coexisting with chip links.** The VR scene is hidden by default and only swaps in when "Enter VR" is clicked ([browse_scene.gohtml:867](../../../internal/static/browse_scene.gohtml#L867)). Chip clicks happen on the 2D layer, which is hidden during VR. No interaction.
- **Tag chip ID collisions or escaping.** Tag IDs from Stash are numeric strings; safe in URLs and HTML attrs. Names need standard HTML escape (already done by Go templates).

## 8. Validation

Manual on Quest 3 Meta Browser + a desktop browser:

1. Open a scene with multiple performers, a studio, and several tags. Click a performer chip → filtered grid loads with that performer's scenes. Back button returns to scene detail.
2. Same for studio, then for a tag chip.
3. Hover star 4 → stars 1, 2, 3, 4 are gold; star 5 is grey. Move to star 2 → stars 1, 2 gold; rest grey. Move out → all stars revert to currently-set state.
4. Click star 4 → server confirms. Star highlight stays; URL doesn't change; page doesn't scroll; `<video>` keeps its currentTime.
5. Click ♥ → favorite badge flips. No reload.
6. Type a new tag name, click Add → new chip appears. Input clears.
7. Click ✕ on a chip → chip vanishes. No reload.
8. Click +O five times rapidly → counter increments by 5 (eventually). No flicker, no reset.
9. Click Mark organized → button flips state.
10. Force a server error: temporarily turn off Stash; click ♥ → `.errbanner` shows the error; rest of page is intact.
11. Enter VR mode → all M2/M3a/M3b/M3c behavior unchanged.
12. Exit VR → 2D layer reappears with current state (rating, favorite, tags reflecting any changes made before entering VR).

## 9. Open follow-ups for next milestones

(Things M4a may surface that should inform M4b/M4c or beyond.)

- **Server-pushed updates if Stash is mutated externally.** If the user edits a scene in Stash's own UI while stash-vr's page is open, the page won't reflect it without a reload. Out of scope for M4a; consider an SSE / polling layer if it ever matters.
- **Toast / confirmation UX.** v1 is silent on success. If the user reports "did it work?" doubt, add a brief toast or a transient highlight.
- **Move CSS to a separate file.** `browse_scene.gohtml` is approaching the size where inlined CSS is hostile to skim. M4a may bundle this as a small refactor; decide in the plan.
