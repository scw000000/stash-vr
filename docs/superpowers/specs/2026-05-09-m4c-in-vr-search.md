# M4c design: in-VR search/browse

**Date:** 2026-05-09
**Status:** Drafting (brainstorming approved 2026-05-09).
**Predecessors:** [M4b VR control panel polish](2026-05-09-m4b-vr-control-panel.md) — adds the polished panel that hosts M4c's "Browse" button. M4c assumes M4b is shipped.
**Successors:** **M4c-followup-α** — auto-next on video end, pulling next scene from the current filtered list. **M4c-followup-β** — scene previews on tile hover (3-sec clips). **M4c-followup-γ** — multi-select / queue building. **M4c-followup-δ** — saved-filter integration (`/filters`). **M4c-followup-ε** — sort options (newest / highest-rated / random).
**Reference player:** No direct SKYBOX equivalent for in-VR search; SKYBOX's library browser is a separate app screen, not an in-immersion grid. M4c is original UX guided by stash-vr's /browse 2D semantics.

---

## 1. Context (why this milestone)

M4a + M4b polish the per-scene experience. M4c is the headliner for the entire M4 bucket: the user can switch scenes without exiting WebXR. Without this, every scene change costs a re-entry to immersive mode, which the user has explicitly named the deal-breaker — "in 3d space, that's what makes the whole thing worth building."

M4c surfaces a 3D grid of scene tiles (cover textures + titles), with text search, six filter pickers (Performer, Studio, Tag, Favorites, Stars, O-Counter), and a Clear-all button — mirroring the affordances of /browse 2D but adapted for VR. Selecting a tile triggers a seamless scene swap: the `<video>` source changes, the projection rebinds (per M3a/M3b's existing path), and playback resumes — all without leaving the WebXR session.

## 2. Goal & non-goals

**Goal:** From inside WebXR, the user clicks "Browse" on the M4b panel, sees a configurable curved grid of scene tiles, can search and filter as they would on /browse 2D, and selecting a tile swaps to that scene without re-entering VR.

**Success criteria, manually verified on Quest 3 / Meta Browser:**

1. Click "Browse" on the M4b panel → grid panel appears in front of the user. Default 4 cols × ~3 visible rows on a slight cylinder curve.
2. Tap the cols button on the browse panel → cycles `3 → 4 → 5 → 6 → 3`. Grid relayouts.
3. Push thumbstick Y while browse panel open → grid scrolls vertically, lazy-loading further results. M3c's geometry-scale binding is suppressed during this state.
4. Tap the search field → Meta Browser surfaces Quest's system VR keyboard via DOM overlay. Type → grid filters live (debounced 250 ms).
5. Tap "Filters ▾" → sub-panel opens listing the six filter pickers (Performer, Studio, Tag, Favorites, Stars, O-Counter), each showing its current value. Tap one → its own picker opens with the option list.
6. Pick a performer → grid re-fetches filtered by that performer. Filters sub-panel reflects the active value.
7. Tap "Clear all" → all six filters reset, search clears, grid shows the default `/browse` ordering.
8. Click a tile from a different projection (e.g., DOME → fisheye) → fade-out, `<video>` src + projection rebind, fade-in, M3c geometry pose resets. Scene plays.
9. Click a tile from the same projection → fade-out, src swap only (no projection rebind), fade-in.
10. After a swap, M4b state preserved (mute, speed, loop survive); subtitle picker resets if no captions on new scene.
11. M3c regressions absent: panel hide/show, geometry-drag (when browse panel closed), thumbstick L/R = ±10s, B/Y reset, recenter.
12. M3a/M3b regressions absent: projection auto-detect on first load, Format picker, audio sync, no first-frame flash.

**Non-goals (explicit deferrals):**

- **Auto-next on video end.** When the current scene finishes, automatically pull the next from the current filtered list. M4c-followup-α.
- **Scene previews on hover.** Hover a tile → it plays a 3-sec preview clip. M4c-followup-β.
- **Multi-select / queue building.** Pick several tiles and queue them up. M4c-followup-γ.
- **Saved-filter integration.** Surface user-defined Stash filters (`/filters` UI) inside VR. M4c-followup-δ.
- **Sort options.** newest / highest-rated / random selectors. v1 uses `/browse` default order. M4c-followup-ε.
- **Voice search.** Quest 3 supports voice; not in v1.
- **Persistent search state across VR exits.** Re-entering VR resets search/filter/scroll position.
- **Tile drag to reposition.** Tiles are static.
- **In-VR scene editing (rating, tags) directly on the grid.** Tagging is on the scene detail (M4a). v1 of M4c is read-only browsing.

## 3. Layout

The browse panel sits in front of the user, replacing the dead space typically occupied by the active geometry's ahead-of-camera region. It's positioned at world `(0, 1.4, -2.5)` for cinema mode and `(0, 1.4, -2.5)` in immersive (the sphere is ~100 m radius so the panel sits inside it).

```
Panel layout (~3.6m wide, ~2.4m tall):

┌──────────────────────────────────────────────────────────────────────┐
│  [Search...]                Filters ▾   Clear all   Cols: 4   Close ✕ │  Top strip: 0.20m
├──────────────────────────────────────────────────────────────────────┤
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐                                          │
│  │tile│ │tile│ │tile│ │tile│                                          │  Grid rows
│  └────┘ └────┘ └────┘ └────┘                                          │  (visible: ~3-4)
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐                                          │
│  │    │ │    │ │    │ │    │                                          │
│  └────┘ └────┘ └────┘ └────┘                                          │
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐                                          │
│  │    │ │    │ │    │ │    │                                          │
│  └────┘ └────┘ └────┘ └────┘                                          │
└──────────────────────────────────────────────────────────────────────┘
                Loading more…  (sentinel; appears when scrolling near bottom)
```

Tiles arrange on a slight cylinder of radius 3.0 m; arc width ±60° depending on cols. Each tile is `0.6 × 0.34 m` (16:9-ish, accommodating cover image plus title text below).

Top strip elements (left to right):
- Search field (DOM `<input>` revealed via WebXR DOM overlay; ~1.2 m wide visual surface)
- Filters ▾ button (opens the filters sub-panel)
- Clear all button (resets all filters + search)
- Cols cycle button — text shows current value: `Cols: 4` → `Cols: 5` → ... → `Cols: 3` → `Cols: 4`
- Close ✕ button (dismisses the browse panel)

## 4. Behavior details

### 4.1 Browse panel toggle

The M4b control panel grows by one button: **Browse** (`data-action="browse"`), placed in row 3 between Loop and Format. Click → toggles `vrBrowsePanel` visibility. While open:

- M3c trigger-toggle on empty space STILL closes the M4b panel — closing M4b ALSO closes the browse panel (they share `vrControlsRoot` parenting).
- M3c geometry-drag and thumbstick-scale are suppressed (see §4.4).

### 4.2 Grid: tile rendering

Each tile is `<a-entity class="vr-btn vr-tile" data-scene-id="...">` with two children:

- `<a-plane>` carrying the cover texture (loaded from `/cover/{id}` via `THREE.TextureLoader`).
- `<a-text>` below the plane, value=scene title, `wrap-count=22`.

Cylinder placement, given `cols` and `(row, col)` where col is 0-indexed from left:

```
arcStep = 0.18 rad   (matches ~60° / 5 cols)
arcOffset = (col - (cols - 1) / 2) * arcStep
x = radius * sin(arcOffset)
z = -radius * cos(arcOffset) + 1.0   // panel center at z=-2 ish
y = topY - row * (tileH + gap) - scrollY
rotationY = -arcOffset (radians) so each tile faces inward to user
```

Where `radius = 3.0`, `topY = 1.7`, `tileH = 0.34`, `gap = 0.06`, `scrollY` is the current scroll offset.

A small render-set cache keyed by scene ID stores texture references so re-scrolling old rows doesn't re-fetch.

### 4.3 Search input via DOM overlay

WebXR's `requestSession({ optionalFeatures: ['dom-overlay'], domOverlay: { root: document.getElementById('vrDomOverlay') } })` allows a regular DOM tree to render on top of the immersive view. Meta Browser supports this (per WebXR DOM Overlay Module W3C draft, implemented in Chromium 88+ which Meta Browser inherits).

Implementation:

1. `enterVR()` is updated to pass `domOverlay: { root: document.getElementById('vrDomOverlay') }` in the session config.
2. `<div id="vrDomOverlay">` lives in the page DOM, hidden when not in VR. Inside: an `<input class="vr-search">` and a small label.
3. While the browse panel is hidden, the overlay's contents are display:none.
4. While the browse panel is shown, contents are positioned to align with the search field's apparent location on the curve.
5. Tap (raycast click) on the search-field plane → the JS calls `domOverlayInput.focus()`, which causes Meta Browser to surface Quest's system VR keyboard.

If DOM overlay turns out to be missing in Meta Browser (verify in Task 1 of the plan), fallback is a custom in-VR keyboard component — out of scope for this spec, would be re-spec'd.

### 4.4 Vertical scroll

Browse panel state has a `scrollY` value. Two scroll inputs:

- **Thumbstick Y** while browse panel is open: continuous scroll, rate `0.6 m/sec` per stick magnitude (similar to M3c's scale rate but applied to scrollY). Reuses the M3c thumbstick polling — emit `m3c:browse-scroll` event when `vrBrowsePanel.visible`, suppress `m3c:scale` while emitting browse-scroll.
- **Laser-grab + vertical drag** on the grid background: triggerdown on `.vr-grid-bg` captures cursor.y; per-tick delta is added to scrollY. M3c geometry-drag is opt-out via the same `.vr-scrub`/`.vr-grid-bg` mechanism from M4b (extending the opt-out class list).

`scrollY` clamps to `[0, totalContentHeight - visibleHeight]`.

When `scrollY` reaches within 1 row of `totalContentHeight`, fire `loadMore()` which fetches the next batch of tiles from the server.

### 4.5 Configurable cols

`cols` state defaults to 4, persisted in `localStorage` (key `m4c.cols`) so it survives page reloads (but not per-scene since it's a global UX preference).

The cols cycle button reads current value, increments mod 4 in `[3, 4, 5, 6]`, writes back. Triggers a layout-only reflow (no re-fetch, just re-position existing tile entities).

### 4.6 Filters panel (standalone, 3-column layout + bottom value row)

The filters panel is a **standalone sibling** of the browse panel. Both live under `vrControlsRoot` and are visible together: the browse grid stays visible while the user adjusts filters.

`<a-entity id="vrFiltersPanel" position="3.6 1.4 -2.5" rotation="0 -25 0" visible="false">` sits to the **right** of the browse panel (which is centered at `0 1.4 -2.5` and is 3.6 m wide). The `-25°` Y-rotation angles it toward the user. Width 3.0 m, height ~1.8 m.

**No tabs.** All filter sections are visible simultaneously, leveraging the available 3D space. Three side-by-side columns hold the entity lists (Performer / Studio / Tag); a bottom row holds the value pickers (Favorites / Stars / O-Counter).

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Filters                                                ✕    │
├──────────────────────────────────────────────────────────────┤
│ [Performer: Alice ✕]  [Tag: POV ✕]                           │  Active chips (only when ≥1)
├──────────────────┬───────────────────┬───────────────────────┤
│  Performer       │   Studio          │   Tag                 │
│ [Search Perf…]   │  [Search Stu…]    │  [Search Tag…]        │
│   Alice          │    StudioX        │    POV                │
│   Bob            │    StudioY        │    Outdoor            │
│   Charlie        │    ...            │    ...                │  Three scrollable
│   ...            │                   │                       │  lists, side-by-side
├──────────────────┴───────────────────┴───────────────────────┤
│ Favorites: [Only][Not]   Stars: [1+][2+][3+][4+][5 only]     │
│                          O-Counter: [1+][5+][10+]            │
└──────────────────────────────────────────────────────────────┘
```

Three columns each ~0.95 m wide, separated by 0.05 m gaps. Each column has a header label, its own search field, and its own scrollable list of options.

The bottom row holds value-pickers in a single flow: Favorites buttons left, Stars buttons middle/wrapping, O-Counter buttons right. No "Any" buttons — toggling the active button off (or clearing the chip) is the clear-filter affordance.

#### Behaviour

- **Per-column search.** Each column has its own DOM-overlay-backed search field. Tapping any one focuses the overlay's `<input>`; the `overlayTarget` flag tracks which column is being typed into. Live filter (debounced 100 ms, local client-side `includes` on `name`).
- **List rendering.** Each kind fetches its option list once on first panel-open (cached for the session). Visible window: ~5 rows per column with scroll for the rest.
- **List item selection.** Tap a name → applies as the active filter for that kind (single-select). The active chip appears at top of panel. Tap a different name → swaps. Tap the same name again → clears.
- **Bottom-row value pickers.** Tap a button (e.g., "3+" under Stars) → applies; button highlights. Tap the same button again → clears (button de-highlights). No "Any" button needed — its absence is the cleared state.
- **Active filter chips.** One chip per active filter, up to 6. Each chip reads "Performer: Alice" / "Stars: 3+" etc. Tap ✕ → clears that filter, refreshes the grid.
- **Single-select v1.** Each kind allows one active value. Multi-select for Performer/Tag is M4c-followup.
- **Scroll target.** Five possible focuses: the grid, or each of the three lists, or none (if user last clicked the bottom row). Tracked via `lastScrollFocus = 'grid' | 'list-performer' | 'list-studio' | 'list-tag' | 'none'`. Thumbstick Y scrolls only the focused element. Tapping a row sets focus to that column.
- **Close ✕** dismisses the filters panel without clearing filters. Active filters persist; the panel can be reopened to keep editing.
- **Closing the browse panel** also closes the filters panel.

```
┌──────────────────────────────────┐
│  Filters                Close ✕   │
├──────────────────────────────────┤
│  Performer:  [None]      ▸  pick │
│  Studio:     [None]      ▸  pick │
│  Tag:        [None]      ▸  pick │
│  Favorites:  [Any]       ▸  pick │
│  Stars:      [Any]       ▸  pick │
│  O-Counter:  [Any]       ▸  pick │
└──────────────────────────────────┘
```

Click a picker row → the standalone options panel (further right) opens with the option list. Pick an option → applies, options panel closes; filters panel stays open so the user can continue adjusting other filters.

Picker semantics:

- **Performer**: list of all performers (alphabetic). Select one (single-select). Filter applies as `?performer=<id>` server-side.
- **Studio**: same, single-select.
- **Tag**: same, single-select. (Multi-select is M4c-followup.)
- **Favorites**: 3 options — `Any`, `Favorites only`, `Not favorites`. Server-side: applies the FAVORITE_TAG filter via Stash GraphQL.
- **Stars**: options `Any`, `≥1 ★`, `≥2 ★`, `≥3 ★`, `≥4 ★`, `5 ★ only`. Server-side: maps to Stash's rating100 filter.
- **O-Counter**: options `Any`, `≥1`, `≥5`, `≥10`. Server-side: maps to Stash's o_counter int filter.
- **Clear all**: resets all six filters + search query.

Each filter's current value displays in its row; "None"/"Any" indicates inactive.

### 4.7 Server endpoints

#### 4.7.1 `GET /browse/grid?...`

JSON list endpoint, parallel to the existing HTML `/browse`. Query params:

- `q` (text search)
- `performer` (id)
- `studio` (id)
- `tag` (id)
- `favorite` (string: `only` / `not`; absent = any)
- `stars` (int: minimum 1-5; or `5` with strict bit; v1 uses `>=` semantics, with `stars=5strict` for exact-5)
- `ocount` (int: minimum)
- `cursor` (string for pagination — see below)
- `limit` (default 24)

Returns:

```json
{
  "tiles": [
    {
      "id": "1234",
      "title": "...",
      "thumbnailURL": "/cover/1234",
      "projection": { "geometry": "equirectangular", "fov": 180, "stereo": "sbs" }
    },
    ...
  ],
  "nextCursor": "opaque-string",
  "hasMore": true
}
```

`cursor` is opaque server state — for v1, just a base64 of `{page: int, params...}` consumed by the same `fetchSceneIDs` path.

The `projection` field per tile lets the seamless swap know what to bind without a second request. Server fills it via the same `apiinternal.Detect` path that `/browse/scene/{id}` uses.

#### 4.7.2 `GET /browse/scene/{id}/projection-meta`

Lightweight JSON used as a freshness check before swapping. Returns `{streamURL, projection}`. The grid endpoint already provides this per tile, so this endpoint is mainly a safety net for swap-time freshness; it can be skipped if the grid data is fresh enough. **Decision:** skip this endpoint for v1; trust the grid's projection field. Simpler.

#### 4.7.3 `GET /browse/filter-options/{kind}`

Returns the option list for a filter picker:

- `kind = performer` → `[{id, name}, ...]` of all performers.
- `kind = studio` → list of studios.
- `kind = tag` → list of tags.

Cached server-side; mirrors what `/browse` sidebar already loads.

### 4.8 Seamless scene swap

1. User clicks tile.
2. JS captures the tile's `data-scene-id` and projection.
3. Fade audio: `video.volume` to 0 over 200 ms via `requestAnimationFrame`.
4. Fade visual: overlay a black `<a-plane>` parented to the camera at `(0, 0, -0.5)`, fade its opacity to 1 over 200 ms.
5. Update `<video>` element:
   - `video.pause()`
   - `video.src = '/browse/scene/' + id + '/stream'`
   - `video.load()`
6. Update active geometry per the new projection:
   - Read tile's projection metadata.
   - If different from current: apply M3b's `applyPickerState()` with the new values; re-bind material via `applyAll()`.
   - If same: no rebind needed.
7. Update `<a-scene>` data attrs (`data-stereo`, `data-geometry`, `data-fov`) so subsequent renders are consistent.
8. Reset M3c geometry pose: `resetGeometry()` from M3c.
9. Wait for `loadedmetadata` + `canplay`.
10. `video.play()` (autoplay; user gesture from the tile click is sufficient).
11. Fade audio back: `video.volume` to 1 over 200 ms.
12. Fade visual back: black overlay opacity to 0 over 200 ms; remove the overlay node.
13. Hide the browse panel (`vrBrowsePanel.visible = false`).

Total swap time: ~600 ms perceived to user, ~200 ms of which is unavoidable buffer fetch.

The M4b panel state (mute, playbackRate, loop) survives the swap because they're properties of `<video>` which we don't recreate. Subtitles reset because the new scene may not have captions; M4b's CC button updates accordingly (hidden if no captions on new scene).

### 4.9 Keyboard shortcuts (for completeness)

When the browse panel is open:

- Thumbstick Y → scroll (suppresses M3c scale)
- Thumbstick X → still ±10s seek of CURRENT scene (M3c untouched)
- Trigger single-click on tile → swap
- Trigger single-click on empty space (still routes through M3c) → toggles M4b panel + browse panel together
- B/Y short-press → in immersive: yaw recenter; in cinema: reset cinema plane (M3c untouched)
- B/Y long-press → full recenter (M3c untouched)

## 5. Data model

`SceneDetailData` is unchanged from M4a/M4b; this milestone doesn't add fields to that struct. New types:

```go
type GridTile struct {
    ID           string                  `json:"id"`
    Title        string                  `json:"title"`
    ThumbnailURL string                  `json:"thumbnailURL"`
    Projection   apiinternal.Projection  `json:"projection"`
}

type GridResponse struct {
    Tiles      []GridTile `json:"tiles"`
    NextCursor string     `json:"nextCursor,omitempty"`
    HasMore    bool       `json:"hasMore"`
}

type FilterOption struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

Server-side helper: `buildGridResponse(ctx, libraryService, params GridParams) (GridResponse, error)` lives in a new `internal/api/browse/grid.go` (or extends the existing one).

## 6. Files touched

```
internal/api/browse/router.go               <- mount /browse/grid, /browse/filter-options/{kind}
internal/api/browse/grid_json.go (new)      <- JSON grid + filter-options handlers
internal/api/browse/data.go                 <- GridTile, GridResponse, FilterOption
internal/static/browse_scene.gohtml         <- vrBrowsePanel, vrFiltersPanel, vrFilterOptions sub-panel,
                                              vrDomOverlay div, browse-related JS (texture loader, scroll handler,
                                              filter state machine, swap routine)
internal/static/m3c-controls.js             <- suppress geometry-drag/scale when vrBrowsePanel.visible;
                                              emit m3c:browse-scroll on thumbstick Y in that state
```

No new external dependencies. Three.js TextureLoader is part of A-Frame's bundled THREE.

## 7. Risks

- **DOM overlay availability in Meta Browser.** Highest risk — the entire search UX hinges on this. **Plan task 1 verifies it.** If it fails, the spec needs revision (custom in-VR keyboard adds ~2-3 days).
- **Seamless scene swap during WebXR session.** Three.js scene-graph changes mid-session can race with rendering. Mitigation: gate the swap on a `tick` event so we control timing relative to the render loop.
- **Texture upload latency for the grid.** 24 tiles × 1 KB+ texture each. JPEGs decode async. First page may have visible pop-in as textures load — acceptable; show a placeholder gray plane then swap.
- **`SceneCount`-related performance for filter pickers.** Loading all performers (could be hundreds) into a picker list may stutter. Mitigation: lazy-load on first open, use the existing sidebar's data, page within the picker if the list exceeds 50.
- **Cylinder geometry math precision at 5 or 6 cols.** Tiles near the edges may face away from the user too steeply. Mitigation: cap arc at ±60°; adjust `arcStep` so total arc stays within bound regardless of cols.
- **Thumbstick Y conflict with M3c scale.** Need a clean handoff: when browse panel opens, suspend M3c scale; when closes, resume. Implementable as a state boolean read by M3c's tick handler.
- **Filter combinatorics on server.** 6 filters + search + cursor means GraphQL query construction. Already factored through `internal/stash/filter`; should compose cleanly. Confirm that `Stars` and `O-Counter` filter types are exposed by the existing `SceneFilterType` (they are — `rating100` and `o_counter` exist).
- **Live-search debounce vs server load.** 250 ms debounce is comfortable. Server-side `singleflight` key includes the search query, so rapid keystrokes don't all trigger fetches.
- **Browser memory under continuous scroll.** If the user scrolls through 500 tiles, that's 500 textures in memory. Mitigation: virtualize — keep only ±50 tiles' textures around the visible window; release others.
- **Scene swap fails (network drop, bad ID).** Detect via `error` event on `<video>` after `load()`; cancel the fade-in, show an error toast, restore previous scene if possible (cache previous scene state before swap).
- **Re-rendering grid on cols change.** With 50 tiles loaded, recomputing all positions is fine (~50 transform updates). But texture nodes shouldn't be recreated; only their parents' position should change. The implementation must reuse entity nodes across cols changes.

## 8. Validation

On Quest 3 / Meta Browser, assuming M4a and M4b shipped:

### A. Initial state
- [ ] Click "Browse" on M4b panel → grid appears with default 4 cols, ~3 visible rows.
- [ ] Top strip shows Search, Filters ▾, Clear all, Cols: 4, Close ✕.
- [ ] Each tile shows cover image and title.

### B. Cols cycle
- [ ] Tap Cols → "Cols: 5", grid relayouts smoothly.
- [ ] Tap again → "Cols: 6", "Cols: 3", "Cols: 4".
- [ ] Reload page, re-enter VR, open browse → cols persists from localStorage.

### C. Vertical scroll
- [ ] Push thumbstick Y down with browse open → grid scrolls down.
- [ ] Hold for 2 s → scrolls past visible window. Lazy-load fires; new tiles appear.
- [ ] Release → scroll stops smoothly.
- [ ] Push UP → scrolls back up to top. Clamps.
- [ ] M3c scale doesn't fire while browse open (verify by checking sphere/plane size doesn't change).

### D. Search
- [ ] Tap search field → Quest VR keyboard pops.
- [ ] Type "POV" → grid filters to matching scenes within 250 ms.
- [ ] Clear search → grid restores.

### E. Filter pickers
- [ ] Tap Filters ▾ → standalone Filters panel appears to the right of the grid (angled toward user). Grid stays visible.
- [ ] Three columns visible side-by-side: Performer, Studio, Tag. Each column has a header label, a search field, and a scrollable list of names.
- [ ] Bottom row visible with Favorites buttons [Only][Not], Stars buttons [1+][2+][3+][4+][5 only], O-Counter buttons [1+][5+][10+]. No "Any" buttons.
- [ ] Tap Performer column's search field → DOM-overlay input focuses; Quest VR keyboard pops.
- [ ] Type "Ali" → Performer list narrows; Studio and Tag lists unchanged.
- [ ] Tap "Alice" → active chip "Performer: Alice ✕" appears at top of panel; "Alice" row highlights blue; grid filters to Alice's scenes.
- [ ] Tap "Alice" again → row de-highlights; chip disappears; grid restores. (Toggle-off clears the filter.)
- [ ] Tap chip ✕ instead → same effect: filter clears.
- [ ] Repeat searches on Studio and Tag columns independently — three separate searches, three separate lists.
- [ ] Tap "Only" under Favorites → button highlights; chip "Favorites: Only ✕" appears; grid filters.
- [ ] Tap "Only" again → button de-highlights; chip disappears; filter clears.
- [ ] Tap "3+" under Stars → button highlights; chip "Stars: 3+ ✕" appears.
- [ ] Tap "1+" under O-Counter → button highlights; chip appears.
- [ ] Push thumbstick Y after tapping inside Performer column → Performer list scrolls (Studio, Tag, grid don't move).
- [ ] Tap inside Tag column then push thumbstick Y → Tag list scrolls.
- [ ] Tap a grid tile area then push thumbstick Y → grid scrolls.
- [ ] Tap "Clear all" on browse top strip → all chips clear; all values reset; all column highlights de-activate; grid restores.
- [ ] Close browse panel ✕ → both panels (browse + filters) close together.

### F. Scene swap (different projection)
- [ ] Currently watching DOME 180° SBS. Open browse, click a fisheye tile → fade out, video swaps, projection swaps, fade in. Fisheye plays.
- [ ] Browse panel closed automatically.
- [ ] M3c geometry pose at default for new projection.
- [ ] M4b mute/speed/loop preserved across swap.
- [ ] M4b time text restarts from 0:00 with new total.

### G. Scene swap (same projection)
- [ ] DOME → another DOME → no projection rebind, just src swap. Fade in/out.

### H. M3c regressions
- [ ] Browse panel closed: trigger toggle still works, geometry-drag fires on empty space, thumbstick scale works, B/Y reset works.

### I. M4b regressions
- [ ] After swap: scrub bar reflects new scene's duration; mute persists; loop persists; speed persists.
- [ ] Captions: if old scene had captions and new doesn't, CC button hides; vice versa, CC button appears.

### J. Edge cases
- [ ] Search returns 0 results → grid shows "No scenes found" placeholder text.
- [ ] Scroll to end of full result set → "No more scenes" sentinel; loadMore stops.
- [ ] Click tile while previous swap is in flight → ignored (debounced) until first swap completes.
- [ ] Network failure mid-swap → error overlay; restore previous scene.

## 9. Open follow-ups for next milestones

**M4c-followup-α — auto-next on video end.** When playback ends, automatically load the next tile from the current filtered list. Requires preserving the result list across the active scene.

**M4c-followup-β — scene previews on tile hover.** Quest 3 has decent thumbnail-strip support; could play a 3-sec preview MP4 on hover via the existing `Paths.Preview` Stash field (already in the GraphQL fragment).

**M4c-followup-γ — multi-select / queue building.** Long-press a tile to add to a queue; queue plays sequentially.

**M4c-followup-δ — saved-filter integration.** Surface user-defined Stash saved filters (the same ones `/filters` UI manages) as a 7th picker or a top-level entry alongside Filters.

**M4c-followup-ε — sort options.** Newest / highest-rated / random / longest selectors. v1 inherits `/browse`'s default order.

**M4c-followup-ζ — tag picker multi-select.** Single-select for v1 keeps the picker simple; multi-tag is the natural follow-up.
