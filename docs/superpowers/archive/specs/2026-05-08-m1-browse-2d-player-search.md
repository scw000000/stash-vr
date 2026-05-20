# M1 design: /browse 2D player + catalog search

**Date:** 2026-05-08
**Status:** Approved by user (`/brainstorming` session 2026-05-08).
**Successor:** This is Milestone 1 of a multi-milestone project. M2 = WebXR VR
player. M3 = multi-format VR. M4 = sync polish + legacy endpoint decisions.
M2-M4 each get their own spec/plan/implementation cycle later.

---

## 1. Context (why this milestone)

Earlier sessions established:

1. The DeoVR app's in-VR Chromium browser does **not** support WebXR
   (manually verified 2026-05-08 against
   https://immersive-web.github.io/webxr-samples/immersive-vr-session.html —
   "VR not found"). Quest's **Meta Browser** does. Forward UX target is
   therefore Meta Browser, not the DeoVR app.
2. SLR's right-panel UX (filter facets) is rendered by SLR's own Astro/Solid
   SPA + WebGL+WebXR player inside Quest's Chromium webview, keyed off
   DeoVR's hardcoded `sexlikereal.com` host name. We cannot trigger that
   mode for our domain. Full evidence:
   [docs/superpowers/research/2026-05-08-slr-playback-hook/notes.md](../research/2026-05-08-slr-playback-hook/notes.md).
3. DeoVR's JSON schema (`/deovr` endpoint) provably cannot express
   facets / hierarchy / pagination — see
   [docs/superpowers/research/2026-05-08-deovr-shape/notes.md](../research/2026-05-08-deovr-shape/notes.md).
4. Stash-vr's existing `/browse` HTML surface (~1200 LoC) already has
   sidebar facets, scene grid, scene detail with rating/favorite/tag/
   O-counter/organized mutations, pagination, dark theme, right-side
   sidebar layout. The user picked **"extend /browse incrementally"** as
   the M1 framing — server-rendered Go templates throughout, no SPA
   framework, no build pipeline.

The actual gaps between today's `/browse` and a self-sufficient discovery
+ playback surface in Meta Browser are narrow. M1 closes the two largest
ones: in-page 2D playback and catalog-wide title search. Other gaps (multi-
select facets, CSS polish, funscript, heatmap-on-player, watch-resume) are
explicitly deferred.

## 2. Goal & non-goals

**Goal:** make `/browse` self-sufficient for discovery + 2D playback inside
Meta Browser on Quest 3. Two concrete additions: an inline `<video>` on
scene detail, and a catalog-wide title search box on the index. Remove the
now-unused DeoVR launch buttons.

**Success criteria (binary, manually verified on Quest 3 / Meta Browser):**

1. Click any scene tile on `/browse` → scene detail loads.
2. Scene detail shows a video element. Click play → audio audible, frames
   visible. Seek scrubber works.
3. `/browse?q=foo` returns scenes whose title/details/path match "foo".
4. Empty `q` → behaves identically to today's `/browse` (no filter).
5. Entity-filtered routes (`/browse/perf/12?q=foo`) scope the search to
   that entity's scenes.
6. The `▶` overlay on tiles and the "Play in DeoVR" button on scene
   detail are gone.
7. All existing mutations still work: rating, favorite, tag add/remove,
   O-counter +/-, organized toggle.

**Non-goals (deferred to later milestones):**

- WebXR / VR playback. (M2.)
- Multi-format projection support. (M3.)
- New mutations or sync features. (M4. The existing ones already work.)
- Multi-select facets — combining performer + tag + studio AND-stacked.
  (Possibly M2 or never; out of scope here.)
- CSS polish redesign. Small adjustments allowed where we touch a surface
  (e.g. removing the `.quickplay` style); no broader visual refresh.
- Funscript timeline, heatmap-on-player scrubber, in-page lightbox preview,
  watch-resume / continue-watching, video resolution selector. All later
  or never.
- Dropping `/deovr` and `/heresphere` JSON endpoints. They stay running
  for users who keep using those apps. The fate of those endpoints is a
  separate decision deferred to M4 or beyond.

## 3. 2D player on scene detail

Replace the existing "Play in DeoVR" button on `/browse/scene/{id}` with
an inline `<video>` element:

```html
<video controls playsinline autoplay muted preload="metadata"
       src="{{.DirectStreamURL}}"></video>
```

**Rationale for each attribute:**

- `controls` — browser-default UI. No custom controls bar; YAGNI.
- `playsinline` — Quest 3 / iOS Safari / Meta Browser will otherwise
  fullscreen-takeover on play.
- `autoplay muted` — `autoplay` alone is blocked by browser policy unless
  the document has user activation. The `muted` form is allowed
  unconditionally. User unmutes via the controls' volume button.
  Acceptable trade-off for one-click playback.
- `preload="metadata"` — load only enough to render the scrubber, not the
  whole file. Stash's direct stream serves byte-range; the browser
  reaches further as the user scrubs/plays.

**Source URL:** Stash's direct stream URL, built via the existing
`stash.GetDirectStream(vd.SceneParts)` helper. This URL has the API key
appended via `stash.ApiKeyed` and points directly at Stash (not through
stash-vr). Same URL `/deovr/{id}` already hands to DeoVR, so the byte-
range path is well-trodden.

**Layout:** video sits above the metadata block on the scene detail page.
Vertical flow: video → title → metadata (rating / favorite / tags /
O-counter / organized). Same pattern as YouTube / Vimeo scene pages. No
columns, no fancy responsive grid.

**End-of-video behavior:** the video stops. No auto-next, no loop. M4
might add navigation; not here.

**Watch-resume:** out of scope. M4 if at all. Stash supports `resume_time`
in the GraphQL schema; we just don't read or write it in M1.

**Resolution selector:** out of scope. HTML5 native video doesn't support
in-player switching without HLS/DASH segmentation, which Stash doesn't
provide by default. The single direct stream is what plays.

## 4. Catalog-wide search

A single text input above the scene grid on `/browse` (and on entity-
filtered routes — search scopes to the current view). Submits on Enter
via a tiny `<form method="GET">` with one input named `q`.

**URL patterns:**
- `/browse?q=foo` — global catalog search.
- `/browse/perf/12?q=foo` — search within Performer 12's scenes.
- `/browse/studio/3?q=foo` — search within Studio 3's scenes.
- `/browse/tag/45?q=foo` — search within Tag 45's scenes.
- Empty `q` (or absent) — no filter; current behavior.

**Server-side mechanism:**

Stash's GraphQL `FindFilterType` already has a `Q` field for full-text
search across title, details, path, oshash, etc. — same field Stash's own
web UI uses. We thread it through:

- `fetchSceneIDs` in [internal/api/browse/grid.go](../../../internal/api/browse/grid.go)
  gains a `q string` parameter. When non-empty, sets
  `FindFilterType.Q = util.Ptr(q)`. Otherwise leaves it nil.
- `indexHandler` and `entityHandler` in
  [internal/api/browse/index.go](../../../internal/api/browse/index.go)
  read `q` from `r.URL.Query()` and pass it through.
- `pagerURLs()` already preserves arbitrary query params via its
  `extraParams` argument; `?q=foo&page=2` works for free as long as the
  handlers `extra.Set("q", q)` (or just don't `extra.Del("q")`) before
  building the pager.
- `PageData` gets a new `SearchQuery string` field so the template can
  pre-fill the input with the current query.

**No client-side filtering of grid contents.** Search is server-rendered
on form submit. The current `filterList()` JS that filters the sidebar
in-place stays unchanged — different concern (sidebar navigation
shortcut, not catalog search).

**Sidebar behavior:** the sidebar continues to show the **full catalog's**
performers/studios/tags regardless of search. Sidebar is a navigation
shortcut, not bound to the search result. (Making the sidebar respect the
search filter is a different, larger change involving the
`LoadSidebar` query — explicitly out of scope.)

**Search scoped to current view:** the entity handlers already build a
`gql.SceneFilterType` (e.g., `Performers: {Value: [12]}`). We pass that
filter PLUS the Q parameter. Stash applies them as AND. So
`/browse/perf/12?q=foo` returns Performer 12's scenes whose title/etc.
matches "foo".

## 5. What gets removed

Removed code paths — these are dead post-M1:

- `Card.DeoVRPlayURL` field in `internal/api/browse/data.go`. The
  per-tile `▶` overlay that linked to it is removed too.
- `SceneDetailData.DeoVRPlayURL` field in the same file. The "Play in
  DeoVR" button block in `browse_scene.gohtml` is replaced by the
  `<video>` element.
- The `c.DeoVRPlayURL = "/deovr/" + url.PathEscape(...)` assignment in
  `buildCards()` — gone.
- Any `DeoVRPlayURL = ...` building inside
  [internal/api/browse/scene.go](../../../internal/api/browse/scene.go).
- The `.quickplay` CSS rule in `browse.gohtml` — no longer referenced.

`/deovr` and `/heresphere` JSON endpoints stay running. We're only
removing the `/browse` UI's links into them. Direct visitors to
`/deovr/{id}` (DeoVR app users) get exactly the same response they did
before.

## 6. What stays untouched

- Sidebar (perf/studio/tag tabs + client-side `filterList()` JS that
  filters within the visible tab).
- All mutation handlers and their UI: rating stars, favorite toggle, tag
  add/remove form, O-counter +/-, organized toggle. All POST routes in
  [internal/api/browse/scene_post.go](../../../internal/api/browse/scene_post.go)
  unchanged.
- Pagination + `pagerURLs()`.
- Entity-filtered routes (`/browse/perf/{id}`, `/browse/studio/{id}`,
  `/browse/tag/{id}`).
- `library.Service`, GraphQL client, generated bindings.
- Auto-section logic. (Still has the 400-section problem for `/deovr`
  consumers, but that's a separate fight, deferred.)
- `/cover/{id}` heatmap proxy. Still used for thumbnails.
- HereSphere two-way tag sync, legend strings, all of `/heresphere`.
- HTTPS / Caddy / DuckDNS setup.

## 7. Files touched

| File | Change |
|---|---|
| `internal/api/browse/grid.go` | `fetchSceneIDs` gains `q string` parameter; sets `FindFilterType.Q = util.Ptr(q)` if non-empty. `buildCards` drops the `DeoVRPlayURL` assignment. |
| `internal/api/browse/index.go` | `indexHandler` and `entityHandler` read `q` from `r.URL.Query()`, pass to `fetchSceneIDs`, ensure `extra.Set("q", q)` so the pager preserves it (or simply don't `extra.Del("q")`), populate `PageData.SearchQuery`. |
| `internal/api/browse/data.go` | Remove `Card.DeoVRPlayURL`, `SceneDetailData.DeoVRPlayURL`. Add `PageData.SearchQuery string`. Add `SceneDetailData.DirectStreamURL string`. |
| `internal/api/browse/scene.go` | Stop building `SceneDetailData.DeoVRPlayURL`. Build `SceneDetailData.DirectStreamURL` via `stash.GetDirectStream(vd.SceneParts)` (first source URL). |
| `internal/static/browse.gohtml` | Add search `<form>` above `.grid` with a single `<input name="q" value="{{.SearchQuery}}">`. Remove the `.quickplay` overlay markup AND its CSS rule. |
| `internal/static/browse_scene.gohtml` | Replace the "Play in DeoVR" button block with a `<video controls playsinline autoplay muted preload="metadata" src="{{.DirectStreamURL}}">` element. |

**No new files. No new packages. No new env vars. No router changes. No
GraphQL document changes.**

## 8. Validation plan

Manual verification only, per [CLAUDE.md](../../../CLAUDE.md) (no test
suite).

**Build-level (each task):**
- `go vet ./...` clean.
- `go build ./...` clean.

**Curl-level (catches regressions cheaply):**
- `curl -s http://localhost:9666/browse | grep -i 'name="q"'` — search
  input present.
- `curl -s http://localhost:9666/browse?q=test` — returns HTML, status 200.
- `curl -s http://localhost:9666/browse/scene/<known-id>` — HTML contains
  `<video` and a `src=` attribute pointing at the Stash direct stream URL.
- `curl -s http://localhost:9666/browse | grep -i 'quickplay\|deovrplay'`
  — no matches (DeoVR launch UI fully removed).

**Quest 3 / Meta Browser (the actual UX validation):**
- Open `https://stash-vr.duckdns.org/browse`. Search box visible.
- Click any tile → scene detail loads.
- Video plays inline (audio + frames). Seek works.
- Search "<known title fragment>" → grid filters to matching scenes.
- Clear search → full grid back.
- Click sidebar entity (e.g. Performer X) → entity-filtered scenes.
- On the entity-filtered route, type a query → search scopes to that
  entity's scenes only.
- Rating stars / favorite / tag add/remove / O+/- / organized toggle —
  click each, observe the change persists on reload (sanity regression).
- DeoVR launch UI absent: no `▶` overlay on tiles, no "Play in DeoVR"
  button on scene detail.

## 9. Risks (small but worth flagging)

- **`autoplay muted` blocked by Quest 3 / Meta Browser autoplay policy.**
  If it doesn't autoplay, user clicks play once. Acceptable degraded
  behavior; no fallback needed.
- **CORS on the Stash direct stream URL when fetched from
  `stash-vr.duckdns.org`.** Stash typically permits cross-origin via
  its own CORS settings; the same URL works in DeoVR app and the
  existing `/heresphere` flow. If it fails: stash-vr already proxies
  thumbnails via `/cover/{id}`; a parallel `/stream/{id}` proxy would
  be the fix. **Not building it preemptively** — gate on observed
  failure.
- **`FindFilterType.Q` field name in genqlient bindings.** The Stash
  GraphQL schema spells it `q` (lowercase). The genqlient-generated Go
  field will be `Q` per Go convention. If genqlient generated something
  else, the implementation has to adapt — sanity-check the generated
  code at the start of the implementation.
- **Search performance.** Q is full-text and Stash's index speed is
  unknown for the user's library size. If queries get slow, that's a
  Stash configuration concern, not ours. Document in result and move on.
- **Search interacting with `Sort: created_at desc`.** Currently the grid
  is sorted by created_at desc. With Q set, Stash typically orders by
  match relevance unless explicitly overridden. We pass an explicit
  `Sort = created_at` regardless — preserves consistent ordering.

## 10. After this milestone

If M1 ships green, M2 starts: WebXR VR player. M2's working hypothesis is
"add an A-Frame `<a-scene>` block to `browse_scene.gohtml` that, when the
user clicks an Enter VR button, takes over the page and renders the same
direct stream URL as a stereo videosphere." That's a separate spec/plan
session.

If M1 surfaces something unexpected — autoplay policy hostile, CORS bites,
search slow, regression in mutations — we pause and re-spec before M2.
