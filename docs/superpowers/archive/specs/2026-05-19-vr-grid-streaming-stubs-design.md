# VR grid browser: streaming stubs, tile pool, filter-index split — design

**Status:** draft, awaiting user review
**Date:** 2026-05-19
**Scope:** in-VR browse panel only (`/browse/*`). HereSphere / DeoVR endpoints are untouched.

## Problem

Opening the in-VR grid browser feels laggy. Three concrete sources, in likely order of impact:

1. **`/browse/filter-index` ships a "whole library" payload on every open.** The endpoint runs `FindScenesForFacetIndex` with `per_page: -1` ([query.graphql:78-92](../../../internal/stash/gql/documents/query.graphql#L78-L92)) — every scene's id + rating + o_counter + studio + performers + tags — plus four other GraphQL queries in parallel. No caching, server- or client-side. The sidebar appears slow because the matrix payload is large.
2. **`/browse/grid` does N sequential GraphQL round-trips on cold cache.** [grid_json.go:54-80](../../../internal/api/browse/grid_json.go#L54-L80) loops `GetScene(id, false)` one at a time. A batch helper (`GetScenesByIds`) already exists but isn't used here. With `perPage=20` and a cold cache, that's up to 20 sequential calls before any JSON ships.
3. **No tile virtualization.** [relayoutTiles](../../../internal/static/browse_scene.gohtml#L2735) builds one entity (cylinder cover + ⓘ badge + per-character MSDF title) for every tile in `browseState.tiles`. Tiles accumulate as the user infinite-scrolls; after 5 pages = 100 tiles built, even though ~12-15 sit in the visible band. Off-band tiles get `object3D.visible = false` but the meshes, textures, and text glyphs remain resident.

The user also wonders whether browser HTTP caching pays off on LAN. Answer: modest by itself (a `304` saves milliseconds, not seconds). The big win comes from pairing browser revalidation with a **server-side cached snapshot**, so the server doesn't redo the expensive Stash queries on every revalidation either.

## Goals

- Panel opens to a visible tile grid within ~one Stash round-trip on cold cache.
- Tile entity count is bounded regardless of how far the user scrolls.
- Sidebar entity lists render before the matrix arrives.
- Reopen feels instant when nothing's changed (snapshot + `304`).

## Non-goals

- Paginating `FindScenesForFacetIndex`. The intersection feature requires the whole-library matrix.
- Changing the `/cover/{id}` heatmap composition. Covers are already browser-cacheable URL strings and load lazily.
- HereSphere / DeoVR endpoints.
- Any new wire protocol (no NDJSON, no SSE). Plain JSON is sufficient once the server batch fix lands.

## Design

### 1. Grid: batch fetch on the server, two-pass render on the client

**Server.** Replace the per-tile loop in `gridJSONHandler` ([grid_json.go:54-80](../../../internal/api/browse/grid_json.go#L54-L80)) with a single `GetScenesByIds(ctx, ids)` call ([scenes.go:72-115](../../../internal/library/scenes.go#L72-L115) — already exists, already singleflight-friendly through the underlying `FindScenes`). This collapses up to N sequential round-trips into one batch on cold cache. The wire format (`GridResponse`) is unchanged.

**Client — split the render into a stub pass and a hydration pass.**

`fetchGrid().then(json => ...)` already gets the full tile list in one shot. The reason tiles don't appear immediately today isn't the JSON wait — it's that `relayoutTiles` synchronously builds 20 a-cylinder covers, 20 ⓘ badges, and 20×N curved-MSDF title characters in a single frame. Split:

1. **Stub pass.** For each visible-band slot, create only a cylinder cover with a solid background color and the correct `theta-start`/`theta-length`. No texture, no title chars, no badge. One `createElement` + a few `setAttribute` calls per stub. Completes within one frame.
2. **Hydration pass.** For each stub now in the visible band, schedule across `requestAnimationFrame` ticks: set the cover material's `src` URL (texture loads async on the GPU), build the curved title characters via `placeCurvedString`, attach the ⓘ badge, wire the hover/preview handlers via `attachPreviewHandlers`.

Across-frame hydration keeps each frame's work bounded so the headset stays responsive on slow CPUs.

**Tile pool.** Maintain a fixed pool sized `cols × (visibleRows + bufferRows*2)`, where `bufferRows` defaults to 1 (one row of off-band buffer above and below the visible band). At 4 columns and ~15 visible rows that's a pool ceiling of `4 × (15 + 2) = 68` entities; at 6 columns with fewer visible rows, similar. The pool size is bounded regardless of scroll depth.

`relayoutTiles` becomes "given current scrollY + tile data, compute the logical row range that maps onto the pool, then rebind each pool slot to the right `tile.id`". Pool slot rebind:
- Update `theta-start`/`theta-length` and the row's `y`.
- Swap the cover material `src` URL only if the bound tile id changed.
- Rebuild title characters only when the bound tile id changes (not on pure scroll).
- Off-band tiles never have pool slots; scrolled-past tiles release their slot to incoming ones.

Off-pool tiles in `browseState.tiles` keep their data — we only ever discard *entities*, never tile metadata. This preserves the "load more on scroll" behavior; only the rendering footprint is bounded.

### 2. Filter-index: split into catalog + matrix, server-side snapshot, ETag

**Two endpoints replacing `/browse/filter-index`:**

- `GET /browse/filter-catalog` — `{performers, studios, tags}` only. Backed by the existing 4 GraphQL queries (`fetchPerformers`, `fetchStudiosDetailed`, `fetchTagsDetailed`, `FindAllTags`). Small payload, fast.
- `GET /browse/filter-matrix` — `{scenes: [{id, performerIds, studioIds, tagIds, rating100, oCount}, ...]}`. Backed by the existing `FindScenesForFacetIndex`. Large payload, slow on cold cache.

The client fires both in parallel on panel open. The catalog returns first → sidebar columns render with all entities visible (no intersection dimming yet). The matrix arrives later → intersection logic dims unreachable options. The user sees a fully functional sidebar within the catalog round-trip rather than blocked on the matrix.

**Server-side cached snapshot.** Both endpoints back onto an in-memory snapshot in `library.Service`:

- Lazy build on first request, deduped via `singleflight` (same pattern as `GetScenes`).
- Catalog and matrix carry independent monotonic-int versions. Bump only when the snapshot is actually rebuilt **and** content differs from the previous (cheap content hash at the end of rebuild). Identical content keeps the old version, keeping `304`s flowing on no-op rebuilds.
- Invalidation triggers:
  - First request after server start.
  - Whenever `library.Service` invalidates its scene cache via the existing `GetSections` rebuild path.
  - TTL backstop (default 5 minutes) so direct-to-Stash edits get picked up without requiring an index call.

**HTTP caching.** Both endpoints serve with:
- `ETag: W/"<version>"` (weak — semantic equality, not byte-comparison)
- `Cache-Control: no-cache` (force revalidation, never blind reuse)

The browser sends `If-None-Match` on reopen; if the version matches, the server returns `304 Not Modified` with no body. Reopen is two near-zero round-trips.

**Server restart safety.** Counter resets to 1 on restart. A stale browser ETag of `W/"5"` mismatches new `W/"1"`; server returns 200 with the new tag; browser updates. One extra fetch, no staleness — comparison is always per-process.

**Migration.** Keep `/browse/filter-index` for one cycle as a thin alias that composes catalog + matrix into the legacy payload shape, in case anything still depends on it. Remove in a follow-up commit once we confirm the client no longer hits it.

### 3. Verification

**Server-side timing instrumentation.** Zerolog `Dur` fields at:
- `gridJSONHandler`: `findIdsMs`, `batchFetchMs`, `encodeMs`, `totalMs`.
- `filterCatalogHandler`: `gqlMs`, `encodeMs`, `cacheHit` (bool).
- `filterMatrixHandler`: same plus `snapshotAgeMs`.

**Client-side perf marks** via `performance.mark` and one console log per panel open with the deltas:
- `vrbrowse.open` → `vrbrowse.grid.json.received` (server time on the wire)
- `vrbrowse.grid.json.received` → `vrbrowse.grid.stubs.rendered` (stub pass cost)
- `vrbrowse.grid.stubs.rendered` → `vrbrowse.grid.hydration.complete` (hydration cost)
- `vrbrowse.open` → `vrbrowse.facets.catalog.received`
- `vrbrowse.open` → `vrbrowse.facets.matrix.received`

**Manual acceptance scenarios:**

1. **Cold open** (fresh server start): panel opens → stub grid visible within one frame of JSON arrival → covers fill in as textures load → sidebar entity rows render before the matrix arrives.
2. **Warm reopen** (same session): both filter endpoints return `304`. Grid JSON returns from warm `vdCache`.
3. **Infinite-scroll stress**: paginate through 100+ tiles. Tile entity count stays at `cols × (visibleRows + 2)` ≈ 30-40 regardless. No frame drops, no accumulating GPU memory.

**Automated coverage.** No general test suite exists in the repo (per CLAUDE.md); checks are `go vet ./...` + `go build ./...`. The existing `filter_index_test.go` covers payload builder logic — extend it to cover the catalog/matrix split.

## Risks

- **Pool slot churn during fast scroll.** Rapid scroll could churn pool bindings if the buffer is too thin. Mitigation: bump `bufferRows` from 1 to 2 if the perf marks show hydration spikes during scroll. The pool ceiling roughly doubles in the worst case, still bounded.
- **TTL ≠ real invalidation.** A user editing tags directly in Stash sees stale facets for up to TTL seconds. Acceptable trade-off; ties matrix freshness to existing `vdCache` invalidation as primary, TTL as backstop. Tunable via env if it becomes annoying.
- **ETag drift across multi-process deployments.** Not applicable — stash-vr is single-process.

## Out of scope (deferred)

- Reworking `FindScenesForFacetIndex` to chunked / paginated form.
- A general client-side persistent cache (IndexedDB / localStorage) for matrix data.
- Cover image regeneration / heatmap compositing perf.
