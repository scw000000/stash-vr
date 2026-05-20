# M4c Browse panel — corrective redesign

**Status:** design proposed 2026-05-09. Supersedes the tile-rendering portion of [2026-05-09-m4c-in-vr-search.md §4.2](2026-05-09-m4c-in-vr-search.md). Other sections of the original M4c spec stand unchanged.

## 1. Why

Task 4 of the original M4c plan (`0628f7f`) was implemented faithfully but produced an unusable in-VR view. On Quest 3, the user reported "the whole thing looks terrible." Three independent failure modes, all rooted in the spec's `§4.2` rendering math:

1. **Title text 4× oversized.** `<a-text width="2.5">` on tiles 0.6m wide. `width` is the rendered text-geometry width in meters, so each tile's title overflowed into neighbors.
2. **Tiles render behind the panel background.** Panel bg plane sits at panel-local z=0; tile z formula `-3.0 * cos(θ) + 1.0` puts tiles at z=−2.0 to −0.5, behind the bg. With opacity 0.85 they're ghosted/visible-through, layered with the playing video for additional noise.
3. **Tiles spill past panel edges.** Arc half-angle ±60° makes outer-column tiles land at x=±2.23m on a 3.6m-wide panel (edges at ±1.8m). Cols 0 and 4 hang off the panel into empty space.

The user has accepted these as real rendering bugs and asked for a corrective redesign of just the tile-rendering layer, keeping all other M4c subsystems (Tasks 3, 5–9) intact.

## 2. Goals & non-goals

**Goals:**
- Tiles render in front of the panel background, within panel bounds, with title text sized to fit.
- "Gentle curve" feel preserved via per-column rotation, not z-displacement.
- Hovering a tile with the laser plays its 30-sec WebM preview clip from the start, looping while hovered.
- Default 4 cols. Default fetch batch 20 tiles. Vertical scroll loads more (per Task 5).

**Non-goals (deferred or unchanged):**
- Sprite-sheet hover preview (rejected in favor of WebM — the WebM is already Stash's curated highlight).
- Configurable "per-page" UX knob (rejected — fetch batch size is not a user-facing concept; scroll is the natural interaction).
- Filter sub-panel redesign (Task 7's original 3-column layout + bottom value row stays).
- Any change to Tasks 3, 5, 6, 7, 8, 9, 10 of the original M4c plan.

## 3. Tile rendering math

In `browse_scene.gohtml`'s IIFE, the existing `tileCellPositions()` and the body of `relayoutTiles()` are replaced. State (`m4cState`, `m4cCols`, `tileTextures`) and fetch (`fetchGrid`, `buildGridParams`) are kept.

Three principles:

1. **Linear x positioning.** No `R*sin(θ)` displacement.
2. **Coplanar z.** All tiles at panel-local z = 0.02, in front of the bg plane at z = 0.
3. **Per-column rotation only.** The "gentle curve" feel comes from rotating outer tiles inward up to ±10°, not from displacing them in z.

Given current `cols`, panel usable area = 3.4m × 2.0m (panel 3.6 × 2.4 minus padding):

```js
const TILE_GAP_X = 0.06;
const TILE_GAP_Y = 0.06;
const TITLE_STRIP_H = 0.08;
const TITLE_GAP = 0.04;
const MAX_ROT_DEG = 10;
const PANEL_USABLE_W = 3.4;

const tileW = (PANEL_USABLE_W - (cols - 1) * TILE_GAP_X) / cols;
const tileCoverH = tileW * 9 / 16;
const tileH = tileCoverH + TITLE_GAP + TITLE_STRIP_H;

// For tile at (row, col), col 0-indexed from left:
const halfCols = (cols - 1) / 2;
const x = (col - halfCols) * (tileW + TILE_GAP_X);
const y = topY - row * (tileH + TILE_GAP_Y) - scrollY;
const z = 0.02;
const colNorm = halfCols === 0 ? 0 : (col - halfCols) / halfCols;  // -1..+1
const rotY = -colNorm * MAX_ROT_DEG;
```

`topY = 0.85` (first row's center, in panel-local coordinates). Top strip occupies the band around y = 1.0; the 0.85 anchor places the first row just below it with a small gap. `scrollY = 0` initially, ranges to `[0, totalContentHeight - visibleHeight]` per Task 5.

Concrete tile dimensions at default `cols=4`:
- `tileW ≈ 0.805m`, `tileCoverH ≈ 0.453m`, `tileH ≈ 0.573m`
- 5 rows (= 20 tiles default fetch batch) span `5 × 0.573 + 4 × 0.06 = 3.10m` of vertical content
- Panel usable height is 2.0m, so ~3.5 rows are visible; the rest scrolls into view via Task 5

At `cols=6`: `tileW ≈ 0.517m`, `tileH ≈ 0.41m`. Visible rows ≈ 4.

## 4. Tile content

Each `<a-entity class="vr-tile" data-scene-id="...">` has three children:

- **Cover plane:** `<a-plane class="vr-btn vr-tile-cover">`, width=`tileW`, height=`tileCoverH`, z=0.02, cover texture from `/cover/{id}` via `THREE.TextureLoader`. Tap → seamless scene swap (Task 8).
- **ⓘ detail badge:** `<a-entity class="vr-btn vr-tile-detail">`, circle of `radius = Math.min(0.04, tileW * 0.07)` (so it shrinks for narrow tiles at high col counts but caps at 0.04m), positioned at the cover's top-right corner: `(tileW/2 - radius - 0.005, tileCoverH/2 - radius - 0.005, 0.025)`. Tap → opens detail panel (Task 9).
- **Title strip:** `<a-text>` below the cover. **`width = tileW`** (was `2.5` — the bug). `wrap-count` is fixed at `20`; titles longer than that are truncated by A-Frame's text component (single line at `height = TITLE_STRIP_H`). Position: `(0, -(tileCoverH/2 + TITLE_GAP + TITLE_STRIP_H/2), 0.025)`. Color #fff. No background plane — text floats over the panel bg.

## 5. Hover preview (WebM)

Stash exposes a curated 30-sec preview clip per scene at `Paths.Preview` (already in the GraphQL fragment). On hover the preview plays from start, loops while hovered, resets on unhover.

**Implementation per tile:**

- Lazily allocate one `<video>` element on first hover, attach `src = /browse/scene/{id}/preview`. Cache on the tile entity.
- Wrap the video in `THREE.VideoTexture`, swap onto the cover plane's material on hover.
- `video.muted = true; video.loop = true; video.play()` on `mouseenter`.
- On `mouseleave`: `video.pause(); video.currentTime = 0;` and restore the cover-image texture (kept in `tileTextures` map).

**Resource model:** at most one tile is hovered at a time, so at most one decoded WebM stream at a time. Pre-existing video elements (one per ever-hovered tile) sit dormant. If memory becomes a concern, evict on `tile.removeChild` during scroll-out — out of scope for v1 redesign.

**Fallback:** if `Paths.Preview` is empty or 404, `mouseenter` is a no-op (cover stays).

## 6. Panel bg opacity

The panel background plane at panel-local (0, 0, 0) currently has `material="opacity:0.85"`. Bumped to **0.95** so tiles render over an effectively-opaque surface against the playing video. Not 1.0 because a fully-solid panel feels like a hard wall in VR; 0.95 reads as a clearly-foregrounded HUD while preserving slight translucency at the panel edges.

## 7. Server changes

Two changes to `internal/api/browse/`:

### 7.1 Preview proxy

New file `internal/api/browse/preview.go`. Pattern matches the existing caption proxy ([caption.go](../../../internal/api/browse/caption.go)):

- Route: `GET /browse/scene/{id}/preview` mounted in [router.go](../../../internal/api/browse/router.go).
- Handler: fetch the scene from the library service; resolve `vd.SceneParts.Paths.Preview`; reverse-proxy that URL with the Stash API key appended via `stash.ApiKeyed`. Forward `Content-Type` from upstream.
- 404 if the scene has no preview.

### 7.2 Configurable page size

[grid_json.go](../../../internal/api/browse/grid_json.go) currently uses `fetchSceneIDs(...)` from `grid.go` which has a hardcoded `pageSize = 30`. Change:

- Accept `?per_page=N` query param on `/browse/grid`. Clamp to `[1, 60]`. Default `20`.
- Pass to `fetchSceneIDs` via a new variant `fetchSceneIDsWithSize(ctx, client, filter, q, page, perPage int)`. The existing `fetchSceneIDs` becomes a thin wrapper that calls the new variant with `pageSize`.

Client uses `?per_page=20` in `buildGridParams()`.

## 8. State + interactions

**State (in IIFE):**
- `m4cState.tiles` — accumulated `GridTile[]` (existing)
- `m4cState.hasMore` (existing), `m4cState.cursor` (existing)
- `m4cCols` (existing, persisted as `m4c.cols`)
- `tileTextures` cache (existing)
- New: per-tile `<video>` element references, lazily attached to tile entities as `el._previewVideo`

**Interactions:**
- Cols cycle: `relayoutTiles()`. No re-fetch.
- Search/filter change (Task 6/7): `fetchGrid(true)` (reset).
- Scroll near bottom (Task 5): `fetchGrid(false)` (append next batch).

## 9. Files touched by this redesign

- `internal/static/browse_scene.gohtml` — replace `tileCellPositions()` body + `relayoutTiles()` body; tile-construction inside `relayoutTiles()`. Bump panel bg opacity. Add hover handlers.
- `internal/api/browse/preview.go` (new) — preview proxy.
- `internal/api/browse/router.go` — mount preview route.
- `internal/api/browse/grid_json.go` — accept `per_page` param.
- `internal/api/browse/grid.go` — split `fetchSceneIDs` into a sized variant.

No changes to: `data.go`, `projection.go`, plan tasks 3 / 5 / 6 / 7 / 8 / 9 / 10.

## 10. Validation

Manual on Quest 3, after rebuild + redeploy:

- A. Open scene → Enter VR → summon panel → click Browse. Expect: panel appears with 4 cols × ~3.5 visible rows of tiles. Title text fits within tile width. Tiles do not spill past panel edges.
- B. Hover laser over a tile → preview WebM plays from start, looping. Move laser off → preview stops, cover restored.
- C. Click a tile cover → seamless scene swap (Task 8 validates this independently; this redesign just ensures the click target works).
- D. Click ⓘ badge → opens detail panel (Task 9 validates this independently).
- E. Cycle Cols 4→5→6→3→4 → tiles relayout with new widths, no re-fetch, no overflow.
- F. Scroll down via thumbstick (Task 5) → loads next batch of 20.
- G. M4b regressions intact: video continues playing, M4b control panel visible below browse panel, mute/scrub/etc still work.

## 11. Out of scope (do not implement here)

- Sprite-sheet preview (alternative not chosen)
- Per-page user-facing cycle button
- Memory-bounded video element eviction
- Hover preview audio (always muted)
- Aspect-ratio handling for non-16:9 covers (assume 16:9; minor letterboxing acceptable)
