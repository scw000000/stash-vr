# M4c Browse panel — corrective redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broken tile rendering shipped in commit `0628f7f` (oversized titles, tiles behind panel background, tiles past panel edges) with a working linear-x layout, add WebM hover preview, and bump panel background opacity.

**Architecture:** Three tasks. Task 1 adds the server bits (preview proxy, configurable page size). Task 2 replaces the tile rendering math + bumps panel opacity. Task 3 wires the WebM hover preview on top of the new tile structure.

**Tech Stack:** Go 1.24, chi router, A-Frame 1.7, Three.js (via A-Frame), vanilla JS.

**Spec:** [docs/superpowers/specs/2026-05-09-m4c-browse-redesign.md](../specs/2026-05-09-m4c-browse-redesign.md)

**No tests in this project.** Verification is `go vet ./...`, `go build ./...`, and manual on-headset checks at the end of the chain.

**Prerequisite:** This branch (`worktree-m4c-task2-task10`) carries the M4b base + the original M4c Tasks 2/10/3/4 commits. The broken Task 4 commit is `0628f7f`. This plan modifies-in-place rather than reverts; the result is a single improving change history.

---

## Task 1: Server — preview proxy + configurable page size

**Files:**
- Create: `internal/api/browse/preview.go`
- Modify: `internal/api/browse/router.go`
- Modify: `internal/api/browse/grid.go`
- Modify: `internal/api/browse/grid_json.go`

**Goal:** Add `GET /browse/scene/{id}/preview` (proxy of Stash's `Paths.Preview` WebM, API-keyed) and accept `?per_page=N` on `/browse/grid` (default 20, max 60).

- [ ] **Step 1: Create the preview proxy handler**

Create [internal/api/browse/preview.go](../../../internal/api/browse/preview.go):

```go
package browse

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
)

// scenePreviewHandler proxies Stash's preview clip (typically WebM) for a
// given scene. Same pattern as the caption proxy: same-origin so the browser
// can fetch without CORS, and the Stash API key is appended server-side.
// Used by the in-VR browse panel's tile-hover preview.
func (h *httpHandler) scenePreviewHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil || vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Preview == nil {
		http.NotFound(w, r)
		return
	}

	upstream := stash.ApiKeyed(*vd.SceneParts.Paths.Preview)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch preview upstream")
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	upstreamCT := resp.Header.Get("Content-Type")
	if upstreamCT == "" {
		upstreamCT = "video/webm"
	}
	w.Header().Set("Content-Type", upstreamCT)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: copy preview body")
	}
}
```

- [ ] **Step 2: Mount the preview route**

In [internal/api/browse/router.go](../../../internal/api/browse/router.go), add to the route block alongside `sceneCaptionHandler` (around line 27):

```go
	r.Get("/scene/{id}/caption", h.sceneCaptionHandler)
	r.Get("/scene/{id}/preview", h.scenePreviewHandler)
```

- [ ] **Step 3: Split fetchSceneIDs into a sized variant**

In [internal/api/browse/grid.go](../../../internal/api/browse/grid.go), the existing `fetchSceneIDs` hardcodes `pageSize` (=30) into `Per_page`. Add a sized variant and make the existing function a thin wrapper. Replace the existing `fetchSceneIDs` (lines 22–44) with:

```go
// fetchSceneIDs runs FindSceneIdsByFilter at the package's default pageSize.
// Kept for callers that don't need a custom batch size (the existing
// /browse 2D index handler).
func fetchSceneIDs(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, q string, page int) ([]string, int, error) {
	return fetchSceneIDsWithSize(ctx, client, sceneFilter, q, page, pageSize)
}

// fetchSceneIDsWithSize runs FindSceneIdsByFilter at an arbitrary per-page
// batch size. Used by /browse/grid where the in-VR browse client can
// configure how many tiles arrive per request.
// sceneFilter may be nil (all scenes); page is 1-indexed.
// q is an optional full-text search string (passed to FindFilterType.Q).
func fetchSceneIDsWithSize(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, q string, page, perPage int) ([]string, int, error) {
	findFilter := &gql.FindFilterType{
		Page:      util.Ptr(page),
		Per_page:  util.Ptr(perPage),
		Sort:      util.Ptr("created_at"),
		Direction: util.Ptr(gql.SortDirectionEnumDesc),
	}
	if q != "" {
		findFilter.Q = util.Ptr(q)
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, client, sceneFilter, findFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("FindSceneIdsByFilter: %w", err)
	}
	out := make([]string, 0, len(resp.FindScenes.Scenes))
	for _, s := range resp.FindScenes.Scenes {
		if s == nil {
			continue
		}
		out = append(out, s.Id)
	}
	return out, resp.FindScenes.Count, nil
}
```

`pageSize` (=30) stays as the default for the original `/browse` 2D paginator. The grid endpoint will use a different default in step 4.

- [ ] **Step 4: Wire per_page param in grid_json.go**

In [internal/api/browse/grid_json.go](../../../internal/api/browse/grid_json.go), find the parameter-parsing block at the top of `gridJSONHandler`. After parsing `cursor` (the existing `page, _ := strconv.Atoi(q.Get("cursor"))` line), add:

```go
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage <= 0 {
		perPage = 20 // default in-VR batch size; smaller than 2D pageSize=30
	}
	if perPage > 60 {
		perPage = 60
	}
```

Then replace the `fetchSceneIDs(...)` call with:

```go
	ids, total, err := fetchSceneIDsWithSize(r.Context(), h.libraryService.StashClient, sceneFilter, searchQ, page, perPage)
```

And update the `HasMore` calculation that currently uses `pageSize` to use `perPage` instead. Find:

```go
	resp := GridResponse{
		Tiles:   tiles,
		HasMore: page*pageSize < total,
	}
```

Replace with:

```go
	resp := GridResponse{
		Tiles:   tiles,
		HasMore: page*perPage < total,
	}
```

- [ ] **Step 5: Vet + build**

Run from `c:\dev\stash-vr\.claude\worktrees\m4c-task2-task10`:

```
go vet ./...
go build ./...
```

Expected: clean, no output.

- [ ] **Step 6: Commit**

```
git add internal/api/browse/preview.go internal/api/browse/router.go internal/api/browse/grid.go internal/api/browse/grid_json.go
git commit -m "browse: scene preview proxy + configurable per_page on /browse/grid"
```

---

## Task 2: Replace tile rendering math + bump panel opacity

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** Replace the broken `tileCellPositions()` and `relayoutTiles()` (committed in `0628f7f`) with a linear-x layout that fixes oversized titles, z-bleed behind panel bg, and edge overflow. Bump panel background opacity from 0.85 to 0.95.

- [ ] **Step 1: Bump panel background opacity**

In [internal/static/browse_scene.gohtml](../../../internal/static/browse_scene.gohtml), find the `vrBrowsePanel` element (added in Task 3 commit `1212bea`). Inside it, the first child plane is the background:

```html
<a-plane class="vr-grid-bg vr-btn" data-action="grid-bg" width="3.6" height="2.4"
         color="#000" material="opacity:0.85"></a-plane>
```

Change `material="opacity:0.85"` to `material="opacity:0.95"`.

- [ ] **Step 2: Locate the existing tile-rendering block**

In the IIFE script in `browse_scene.gohtml`, find the block introduced by Task 4 (commit `0628f7f`). It contains the constants `TILE_W`, `TILE_H`, `TILE_GAP`, `ARC_RADIUS`, `ARC_HALF`, `scrollY`, the `tileCellPositions()` function, the `tileTextures` cache, `getTileTexture()`, and `relayoutTiles()`.

Use Grep to locate it — search for `ARC_RADIUS` to find the start. The block should run roughly from the `const TILE_W = 0.6;` line through the end of `relayoutTiles()`.

- [ ] **Step 3: Replace the constants block**

Replace the constant declarations (the lines starting with `const TILE_W = 0.6;` through `let scrollY = 0;`) with the new constants:

```javascript
// Tile rendering geometry. Computed at relayout time from m4cCols
// and the panel's usable area. Constants here are the layout *parameters*;
// derived per-tile dimensions live inside relayoutTiles().
const PANEL_USABLE_W = 3.4;     // panel width (3.6) minus 0.1m padding each side
const TILE_GAP_X = 0.06;        // horizontal gap between tiles
const TILE_GAP_Y = 0.06;        // vertical gap between rows
const TITLE_GAP = 0.04;         // gap between cover bottom and title strip top
const TITLE_STRIP_H = 0.08;     // height of the title text strip
const TILE_TOP_Y = 0.85;        // y-coordinate of the first row's center (below top strip)
const TILE_Z = 0.02;            // z offset, in front of panel bg plane (z=0)
const MAX_ROT_DEG = 10;         // outermost columns rotate inward by this much
let scrollY = 0;                // updated by Task 5 (vertical scroll)
```

- [ ] **Step 4: Replace tileCellPositions with the new math**

Replace the existing `tileCellPositions()` function with one that takes `tileW` and `tileH` (computed once per layout from current cols), and returns linear-x positions with per-column rotation:

```javascript
// Returns [{x, y, z, rotY}] for each tile slot, given the current m4cCols,
// computed tile dimensions, and current scroll offset. The "gentle curve"
// look comes from rotY only — all tiles are coplanar at TILE_Z, so the
// panel background plane never occludes them.
function tileCellPositions(tileW, tileH, count) {
  const cols = m4cCols;
  const halfCols = (cols - 1) / 2;
  const positions = [];
  const rows = Math.ceil(count / cols);
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const i = row * cols + col;
      if (i >= count) break;
      const x = (col - halfCols) * (tileW + TILE_GAP_X);
      const y = TILE_TOP_Y - row * (tileH + TILE_GAP_Y) - scrollY;
      const z = TILE_Z;
      const colNorm = halfCols === 0 ? 0 : (col - halfCols) / halfCols;
      const rotY = -colNorm * MAX_ROT_DEG;
      positions.push({ x, y, z, rotY });
    }
  }
  return positions;
}
```

- [ ] **Step 5: Replace relayoutTiles with the new tile construction**

Replace the existing `relayoutTiles()` function with the new version that:
- Computes `tileW` from cols
- Builds tiles with cover plane sized to tileW × tileCoverH, ⓘ badge at proper top-right corner, title strip with `width: tileW`
- Uses the new `tileCellPositions()` signature

```javascript
function relayoutTiles() {
  const root = document.getElementById('vrBrowseTiles');
  if (!root) return;

  const cols = m4cCols;
  const tileW = (PANEL_USABLE_W - (cols - 1) * TILE_GAP_X) / cols;
  const tileCoverH = tileW * 9 / 16;
  const tileH = tileCoverH + TITLE_GAP + TITLE_STRIP_H;
  const badgeR = Math.min(0.04, tileW * 0.07);

  // Remove tile entities for tiles no longer in state.tiles.
  const expected = new Set(m4cState.tiles.map(t => t.id));
  Array.from(root.children).forEach(child => {
    if (!expected.has(child.dataset.sceneId)) {
      root.removeChild(child);
    }
  });

  const positions = tileCellPositions(tileW, tileH, m4cState.tiles.length);

  m4cState.tiles.forEach((tile, i) => {
    const pos = positions[i];
    let el = root.querySelector('a-entity[data-scene-id="' + CSS.escape(tile.id) + '"]');
    if (!el) {
      // Build a new tile entity from scratch.
      el = document.createElement('a-entity');
      el.classList.add('vr-tile');
      el.dataset.sceneId = tile.id;
      el.dataset.projection = JSON.stringify(tile.projection);
      el.dataset.streamUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/stream';
      el.dataset.previewUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/preview';

      // Cover plane — tap → seamless scene swap (Task 8).
      const plane = document.createElement('a-plane');
      plane.classList.add('vr-btn', 'vr-tile-cover');
      plane.setAttribute('width', tileW);
      plane.setAttribute('height', tileCoverH);
      plane.setAttribute('material', 'color:#222;opacity:1;shader:flat');
      plane.setAttribute('position', '0 0 ' + TILE_Z);
      el.appendChild(plane);

      // ⓘ detail badge — top-right corner of the cover. Radius shrinks
      // for narrow tiles at high col counts but caps at 0.04m.
      const detailBadge = document.createElement('a-entity');
      detailBadge.classList.add('vr-btn', 'vr-tile-detail');
      detailBadge.setAttribute('geometry', 'primitive:circle;radius:' + badgeR.toFixed(3));
      detailBadge.setAttribute('material', 'color:#000;opacity:0.85;shader:flat');
      const badgeX = (tileW / 2) - badgeR - 0.005;
      const badgeY = (tileCoverH / 2) - badgeR - 0.005;
      detailBadge.setAttribute('position', badgeX.toFixed(3) + ' ' + badgeY.toFixed(3) + ' ' + (TILE_Z + 0.005).toFixed(3));
      const badgeText = document.createElement('a-text');
      badgeText.setAttribute('value', 'ⓘ');
      badgeText.setAttribute('align', 'center');
      badgeText.setAttribute('color', '#fff');
      badgeText.setAttribute('width', (badgeR * 8).toFixed(3));
      badgeText.setAttribute('position', '0 0 0.005');
      detailBadge.appendChild(badgeText);
      el.appendChild(detailBadge);

      // Title strip — text below the cover. width = tileW so the text
      // geometry matches the tile width (this was the 2.5 bug from M4c
      // Task 4: width was the rendered text-geometry width in meters,
      // not the wrap width).
      const titleEl = document.createElement('a-text');
      titleEl.setAttribute('value', tile.title);
      titleEl.setAttribute('align', 'center');
      titleEl.setAttribute('color', '#fff');
      titleEl.setAttribute('width', tileW);
      titleEl.setAttribute('wrap-count', '20');
      titleEl.setAttribute('height', TITLE_STRIP_H);
      const titleY = -(tileCoverH / 2 + TITLE_GAP + TITLE_STRIP_H / 2);
      titleEl.setAttribute('position', '0 ' + titleY.toFixed(3) + ' ' + (TILE_Z + 0.005).toFixed(3));
      el.appendChild(titleEl);

      // Bind cover texture once available.
      const tex = getTileTexture(tile.thumbnailURL);
      if (tex) {
        plane.addEventListener('loaded', function() {
          const mesh = plane.getObject3D('mesh');
          if (mesh) {
            mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: tex });
          }
        });
      }

      root.appendChild(el);
    } else {
      // Existing tile: cols may have changed → resize cover, badge, title.
      const plane = el.querySelector('a-plane.vr-tile-cover');
      if (plane) {
        plane.setAttribute('width', tileW);
        plane.setAttribute('height', tileCoverH);
      }
      const badge = el.querySelector('.vr-tile-detail');
      if (badge) {
        badge.setAttribute('geometry', 'primitive:circle;radius:' + badgeR.toFixed(3));
        const bx = (tileW / 2) - badgeR - 0.005;
        const by = (tileCoverH / 2) - badgeR - 0.005;
        badge.setAttribute('position', bx.toFixed(3) + ' ' + by.toFixed(3) + ' ' + (TILE_Z + 0.005).toFixed(3));
      }
      const title = el.querySelector('a-text:not(.vr-tile-detail a-text)');
      if (title) {
        title.setAttribute('width', tileW);
        title.setAttribute('height', TITLE_STRIP_H);
        const ty = -(tileCoverH / 2 + TITLE_GAP + TITLE_STRIP_H / 2);
        title.setAttribute('position', '0 ' + ty.toFixed(3) + ' ' + (TILE_Z + 0.005).toFixed(3));
      }
    }
    el.setAttribute('position', { x: pos.x, y: pos.y, z: pos.z });
    el.setAttribute('rotation', { x: 0, y: pos.rotY, z: 0 });
  });
}
```

- [ ] **Step 6: Vet + build**

```
go vet ./...
go build ./...
```

Expected: clean.

- [ ] **Step 7: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: redesign tile rendering — linear x, coplanar z, sized title strip; bg opacity 0.95"
```

---

## Task 3: WebM hover preview

**Files:**
- Modify: `internal/static/browse_scene.gohtml`

**Goal:** When the laser pointer enters a tile's cover plane, swap the cover texture for the WebM preview clip (looped, muted, from start). Restore the cover image on `mouseleave`.

- [ ] **Step 1: Persist the cover URL on the tile entity**

In `relayoutTiles()` from Task 2, the new-tile branch already sets `el.dataset.previewUrl`. The mouseleave handler will need to restore the cover texture, which means the helper needs to find the cover URL via `tileEl.dataset.thumbnailUrl`. Add a sibling line to record it.

Find this block in the new-tile branch:

```javascript
      el.dataset.previewUrl = '/browse/scene/' + encodeURIComponent(tile.id) + '/preview';
```

Add immediately after:

```javascript
      el.dataset.thumbnailUrl = tile.thumbnailURL;
```

(Note on naming: HTML5 `dataset` lowercases attribute names. Setting `el.dataset.thumbnailUrl` produces an attribute `data-thumbnail-url`; reading via `el.dataset.thumbnailUrl` round-trips. Use this exact spelling — `thumbnailUrl`, lowercase `u-r-l` — on both write and read sides.)

- [ ] **Step 2: Add the hover-preview helper**

In the IIFE in `browse_scene.gohtml`, just after `getTileTexture()` (which is unchanged from M4c Task 4), add the preview-attach helper. This caches one `<video>` element per ever-hovered tile on the tile entity itself, so re-hover doesn't re-fetch.

```javascript
// Hover preview: swap a tile's cover texture for its WebM preview clip
// while the laser pointer is on it. One decoded video at a time (whichever
// tile is hovered). The <video> element is allocated lazily on first hover
// and cached on the tile entity as el._previewVideo.
function attachPreviewHandlers(tileEl) {
  const plane = tileEl.querySelector('a-plane.vr-tile-cover');
  if (!plane) return;

  function ensureVideo() {
    if (tileEl._previewVideo) return tileEl._previewVideo;
    const v = document.createElement('video');
    v.src = tileEl.dataset.previewUrl;
    v.muted = true;
    v.loop = true;
    v.playsInline = true;
    v.crossOrigin = 'anonymous';
    v.preload = 'metadata';
    tileEl._previewVideo = v;
    return v;
  }

  plane.addEventListener('mouseenter', function() {
    const v = ensureVideo();
    v.currentTime = 0;
    const playPromise = v.play();
    if (playPromise && playPromise.catch) {
      playPromise.catch(function(err) {
        // Common case: 404 because Stash hasn't generated a preview for
        // this scene. Silently keep the cover.
        console.debug('stash-vr: preview play() failed', err);
      });
    }
    const mesh = plane.getObject3D('mesh');
    if (!mesh) return;
    const tex = new AFRAME.THREE.VideoTexture(v);
    tex.minFilter = AFRAME.THREE.LinearFilter;
    tex.magFilter = AFRAME.THREE.LinearFilter;
    mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: tex });
    mesh.material.needsUpdate = true;
  });

  plane.addEventListener('mouseleave', function() {
    const v = tileEl._previewVideo;
    if (v) {
      v.pause();
      v.currentTime = 0;
    }
    const coverURL = tileEl.dataset.thumbnailUrl;
    const coverTex = coverURL ? tileTextures[coverURL] : null;
    const mesh = plane.getObject3D('mesh');
    if (mesh && coverTex) {
      mesh.material = new AFRAME.THREE.MeshBasicMaterial({ map: coverTex });
      mesh.material.needsUpdate = true;
    }
  });
}
```

- [ ] **Step 3: Call attachPreviewHandlers from the new-tile branch**

In `relayoutTiles()`, the new-tile branch ends with `root.appendChild(el);`. Add right before that line:

```javascript
      attachPreviewHandlers(el);
```

So the new-tile branch tail looks like:

```javascript
      attachPreviewHandlers(el);
      root.appendChild(el);
```

- [ ] **Step 4: Vet + build**

```
go vet ./...
go build ./...
```

Expected: clean.

- [ ] **Step 5: Commit**

```
git add internal/static/browse_scene.gohtml
git commit -m "m4c: WebM hover preview on browse tiles via VideoTexture"
```

---

## On-headset validation (run after Task 3 lands)

Build via `scripts\build-windows.bat` from the worktree, deploy. Open a scene → Enter VR → summon panel → Browse.

- A. Panel appears with 4 cols × ~3.5 visible rows. Title text fits within tile width. Tiles do not spill past panel edges.
- B. Tile cover textures load and render. ⓘ badge visible in top-right corner of each cover.
- C. Point laser at a tile → WebM preview plays from start, looping. Move laser off → cover restored. Hover another tile → its preview starts; previous tile is back to cover.
- D. Cycle Cols 4→5→6→3→4 → tiles relayout with new widths. No overflow. Existing covers persist (no re-fetch unless `total > tiles.length`).
- E. M4b regressions intact: video continues playing behind, M4b control panel visible below browse panel, mute / scrub / etc. still work.
- F. Click a tile cover → seamless scene swap (Task 8 verifies this independently; this redesign only ensures the click target works — Task 8 is unchanged in this plan).
- G. Click ⓘ badge → opens detail panel (Task 9 verifies independently — also unchanged).

If sign convention or visual feel needs adjusting (rotation feels too aggressive / too flat, opacity wrong, etc.), one-line tweaks suffice:
- Less curve: lower `MAX_ROT_DEG` from 10 to 5.
- More opacity: bump panel bg from 0.95 to 1.0 (loses translucency feel but kills any video bleed-through).
- Different default cols: Task 3 of original M4c plan defaulted to 4 — change `m4cCols` initial value.

---

## Self-review checklist

- **Spec coverage:**
  - §3 Tile rendering math → Task 2 steps 3–5
  - §4 Tile content (cover, ⓘ, title strip) → Task 2 step 5
  - §5 Hover preview (WebM) → Task 3 + Task 1 step 1 (preview proxy)
  - §6 Panel bg opacity → Task 2 step 1
  - §7.1 Preview proxy → Task 1 steps 1–2
  - §7.2 Configurable page size → Task 1 steps 3–4
- **No placeholders:** every code block is concrete. The dead-line reminder in Task 3 step 1 is explicitly removed in Task 3 step 2.
- **Type consistency:** `m4cState`, `m4cCols`, `tileTextures`, `getTileTexture` carry through unchanged from existing code. New names: `attachPreviewHandlers`, `el._previewVideo`, `el.dataset.thumbnailUrl`, `el.dataset.previewUrl`. The function `tileCellPositions` changes signature: now takes `(tileW, tileH, count)` instead of zero args. Both call sites (the only caller is `relayoutTiles`) updated.
- **Frequent commits:** three commits, one per task.
- **Risks acknowledged:** `MAX_ROT_DEG = 10` may need tuning on headset (one-line change). WebM hover may be too aggressive on some scenes (cure: increase preload to "auto" or downgrade to sprite-sheet — out of scope here).
- **YAGNI:** no eviction policy for video elements, no sprite-sheet fallback, no Per-page UX knob, no aspect-ratio handling for non-16:9 covers.
