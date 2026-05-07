# /browse — entity-first scene browser served by stash-vr

## Goal

Add an HTML browse surface served by stash-vr that addresses the gap that neither the HereSphere nor DeoVR player APIs can fill: **first-class browsing by performer / studio / tag**, plus inline rating and tag editing, all rendered inside DeoVR's built-in web browser.

This is a third surface alongside the existing `/heresphere` and `/deovr` player APIs. It is not a replacement for either. It complements the player libraries by giving entity-driven discovery and direct rating/edit without bouncing through HereSphere.

## Why this exists

Both HereSphere and DeoVR's library APIs expose only a flat structure: `[{name, list-of-scenes}]`. To get "all scenes featuring performer X" today you must create one Stash saved filter per performer — unworkable. The user's stated top pain is exactly this. Beyond that:

- DeoVR's player API has **no metadata write-back at all** ([internal/api/deovr/videodata.go](../../../internal/api/deovr/videodata.go) — no `Rating`, no `Tags`, no event server). Anything you'd "rate" inside DeoVR doesn't reach Stash.
- HereSphere has the write-back hooks and stash-vr already uses them, but it requires the user to be inside HereSphere with the player loaded.

`/browse` is plain HTML served by stash-vr, so it can:
1. Read whatever Stash data is needed (performers, studios, tags, scenes) without the constraints of either player API.
2. Write rating / favorite / tag / O-counter / organized changes **directly** to Stash via the same GraphQL mutations the HereSphere event handler already uses — no player needed as intermediary.
3. Hand off to DeoVR's player for actual playback via a regular link.

The user reaches `/browse` by typing the URL into DeoVR's built-in browser. They never leave the DeoVR app.

## Architecture

### Routes (mounted on the chi router in [internal/api/router.go](../../../internal/api/router.go))

| Route | Renders |
|---|---|
| `GET /browse` | Sidebar + default scene grid (all scenes, newest first, paginated) |
| `GET /browse/perf/{id}` | Sidebar + scene grid filtered by performer |
| `GET /browse/studio/{id}` | Sidebar + scene grid filtered by studio |
| `GET /browse/tag/{id}` | Sidebar + scene grid filtered by tag |
| `GET /browse/scene/{id}` | Scene detail page with rating / favorite / tag / O-counter / organized controls and a Play in DeoVR button |
| `POST /browse/scene/{id}/rating` | Set rating (Stash `SceneUpdateRating100`) |
| `POST /browse/scene/{id}/favorite` | Toggle favorite tag |
| `POST /browse/scene/{id}/tags/add` | Add a tag (creates it if new, via `TagCreate` then `SceneUpdateTags`) |
| `POST /browse/scene/{id}/tags/remove` | Remove a tag (`SceneUpdateTags`) |
| `POST /browse/scene/{id}/o/increment` | `SceneIncrementO` |
| `POST /browse/scene/{id}/o/decrement` | `SceneDecrementO` |
| `POST /browse/scene/{id}/organized` | Toggle `SceneUpdateOrganized` |

The four GET grid routes all render the **same persistent sidebar** plus a grid that varies by route. From the user's perspective the sidebar appears persistent; under the hood it is a normal full-page server render each time. No SPA, no HTMX, no client-side framework.

### Tech stack

- Go `html/template` server-rendered, same pattern as the existing config page in [internal/api/web/web.go](../../../internal/api/web/web.go).
- Templates live in [internal/static/](../../../internal/static/) (e.g. `browse.gohtml`, `browse_sidebar.gohtml`, `browse_grid.gohtml`, `browse_scene.gohtml`), embedded via the existing `static.Fs`.
- One small inline JS snippet for the per-tab filter `<input>` on the sidebar. Pure progressive enhancement; the page works without JS, the input just doesn't filter.
- POST handlers respond with `303 See Other` redirects so they work without JS (form submit, page reloads). Future JS-driven AJAX path can opt into `204 No Content` via `Accept: application/json`.

### New code lives in

- `internal/api/browse/` (new package, parallel to `internal/api/web`)
  - `router.go` — chi sub-router mounted from [internal/api/router.go](../../../internal/api/router.go)
  - `index.go` — `GET /browse` and the entity-filtered grid handlers
  - `scene.go` — `GET /browse/scene/{id}` and the POST mutation handlers
  - `entities.go` — sidebar entity-list fetch (singleflight-coalesced)
- `internal/static/browse.gohtml` and partials
- New genqlient queries in `internal/stash/gql/documents/query.graphql`:
  - `FindAllPerformersWithCount` (or reuse/extend existing `FindPerformersWithSceneCount`)
  - `FindAllStudiosWithCount` (new)
  - `FindAllTagsWithCount` (extend existing `FindAllTags`)
  - `FindAllSceneIdsSorted` (new — paged scene IDs sorted by `created_at desc`)
- One refactor: lift the rating/favorite/tag/O-counter/organized write logic out of [internal/api/heresphere/event.go](../../../internal/api/heresphere/event.go) into `library.Service` methods so both HereSphere event handlers and `/browse` POST handlers call into a single source of truth for write semantics.

### Caching

- **Sidebar entity lists**: `singleflight` per type, keyed `"browse:performers"`, `"browse:studios"`, `"browse:tags"`. No TTL — re-resolve on each request but coalesce concurrent ones. Same dedup pattern used today by `GetSections`.
- **Scene metadata**: read through the existing `library.Service` cache untouched. `/browse` benefits automatically.
- **`<datalist>` of all tag names on the detail page**: same query as the Tags tab in the sidebar, server-rendered at request time. No async fetch.
- **Default-grid scene IDs (`/browse?page=N`)**: not cached. Newest scenes change often; pagination is cheap on Stash.

After a successful write to a scene, that scene is invalidated in `library.Service` so the next read returns fresh state. Sidebar counts may be briefly stale until the next sidebar fetch — acceptable for MVP.

## Sidebar (the persistent panel)

**Position:** right side of the screen.
**Width:** ~280px on desktop / VR-browser canvases.
**Inner structure:**

```
+-----------------------------------+
| [Performers] [Studios] [Tags]     |  3 tabs (radio-style); ?tab=perf|studio|tag
+-----------------------------------+
| [filter........]                  |  per-tab text filter (vanilla JS, optional)
+-----------------------------------+
| Jade Valentine               23   |
| Claire Roos                  14   |
| LaSirena69                   42   |
| Maddie Perez                 11   |
| ...                               |
+-----------------------------------+
```

- **Tabs**: only one tab's list visible at a time. Active tab carries through navigation via `?tab=perf|studio|tag` so when you click `Jade Valentine` and the page reloads, the tab is still on Performers.
- **Active-entity highlight**: when on `/browse/perf/123`, the row for performer 123 in the Performers tab gets `aria-current="page"` and a subtle background highlight.
- **Per-row content**: entity name (left, ellipsis-truncated) + scene count (right). Nothing else — no avatars, no tag colors.
- **Sort**: by scene count desc, then name asc.
- **Filter input**: small text `<input>` per tab that hides non-matching `<li>` items via vanilla JS. Falls back gracefully if JS is disabled.
- **Hidden entities**: tags whose `Sort_name == EXCLUDE_SORT_NAME` (default `hidden`) are filtered out server-side, matching the player views.
- **Zero-scene entities**: omitted server-side.
- **Empty tab**: render "No performers yet." (or studios/tags) inside the panel.
- **Pagination**: none in MVP. Full list rendered. ~500 rows × 3 columns is well under 100KB.
- **Collapse/hide control**: not in MVP.

## Grid (the main scene area)

### Header strip

| Route | Header text |
|---|---|
| `/browse` | `All scenes — newest first` · `Page X / Y` |
| `/browse/perf/{id}` | `Performer: <Name>` · `<N> scenes` · `Page X / Y` |
| `/browse/studio/{id}` | `Studio: <Name>` · `<N> scenes` · `Page X / Y` |
| `/browse/tag/{id}` | `Tag: <Name>` · `<N> scenes` · `Page X / Y` |

For entity-filtered routes, a `<a href="/browse">← All scenes</a>` link sits at the start of the header.

### Scene card

```
+-----------------------+
|                       |
|       thumbnail       |
|                       |
|                ▶ 30:21|   ▶ = small "quick play in DeoVR" overlay button
+-----------------------+
| Title (1 line, ellip) |
| Performers · Studio   |
+-----------------------+
```

- **Layout**: CSS grid `repeat(auto-fill, minmax(280px, 1fr))`, ~16px gap.
- **Thumbnail source**: for interactive scenes, the existing `/cover/{id}` heatmap-composite URL ([heatmap.GetCoverUrl](../../../internal/api/heatmap/)); otherwise `vd.SceneParts.Paths.Screenshot` via `stash.ApiKeyed`. Same logic as [internal/api/heresphere/videodata.go:84-90](../../../internal/api/heresphere/videodata.go).
- **Duration overlay**: bottom-right, formatted `MM:SS` or `HH:MM:SS` from `vd.SceneParts.Files[0].Duration`.
- **Quick-play "▶" button**: small overlay anchored bottom-right of thumbnail, links to `/deovr/videoData/{id}` directly so a tap on it skips the detail page.
- **Card body tap (anywhere except quick-play)**: navigates to `/browse/scene/{id}` (the detail page).
- **Title**: `vd.Title()` (scene title or filename fallback).
- **Secondary line**: comma-joined performers, then ` · `, then studio. Either side hidden if missing.

### Pagination

- 30 scenes per page.
- `[< Prev]  Page X / Y  [Next >]` at the bottom of the grid.
- Plain `<a href="?page=N">` links so they work without JS.
- `?page=N` preserves other query params (e.g. `?tab=perf`).

### Sort order

- Default `/browse`: `created_at desc` (newest first). Hardcoded in MVP.
- Entity-filtered routes: same sort by default. Hardcoded in MVP.
- Sort control UI is explicitly **not in MVP** (deferred).

### Scene-data fetch

One page = 30 scene IDs from Stash. Reuse `library.Service.GetScenes` (or per-id `GetScene`) so the existing in-memory cache and singleflight are reused. Cold-cache page render does one batch `gql.FindScenes(ids)` call.

### Empty / out-of-range

- `/browse` empty → "No scenes in your Stash library yet."
- Entity route empty → "No scenes for this {performer|studio|tag}."
- Page out of range → empty grid with "Page out of range" + `[← Page 1]`.

## Scene detail page (`/browse/scene/{id}`)

```
+--------------------------------------------------+
| < Back                                           |
+--------------------------------------------------+
|                                                  |
|         +------------------------+                |
|         |                        |                |
|         |   thumbnail / preview  |                |
|         |                        |                |
|         +------------------------+                |
|                                                  |
|  Title                                           |
|  Performers · Studio · 2024-08-12 · 30:21        |
|                                                  |
|  Rating: ☆ ☆ ☆ ☆ ☆       ♥ Favorite              |
|                                                  |
|  Tags: [VR ✕] [POV ✕] [Blowjob ✕]  [+ add tag]   |
|                                                  |
|  O-counter: [ – ]  3  [ + ]    Organized: [ ]    |
|                                                  |
|  [           Play in DeoVR           ]           |
|                                                  |
+--------------------------------------------------+
```

- `< Back` returns to the originating grid via `Referer`, falling back to `/browse`.
- **Stars** are five `<button>`s in a `<form>`; clicking submits `POST /browse/scene/{id}/rating` with `value=1..5`. Clicking the currently-selected star sends `value=0` and clears the rating (Stash treats `0` as null). Server maps `1..5` to Stash's 0–100 scale (×20).
- **♥ Favorite** is a `<form>` with one `<button>`; toggles by adding/removing the `config.Application().FavoriteTag` tag, matching `isFavorite()` in [internal/api/heresphere/videodata.go:148](../../../internal/api/heresphere/videodata.go).
- **Tags as chips**: each chip is a `<form>` with `<button name="tag" value="VR">VR ✕</button>` POSTing to `/browse/scene/{id}/tags/remove`.
- **Add-tag input**: `<input list="all-tags" name="tag">` plus a `<datalist id="all-tags">` of all existing tag names. Submit → `POST /browse/scene/{id}/tags/add`.
  - Existing tag match (case-insensitive): add via `SceneUpdateTags` (current ∪ new).
  - No match: `gql.TagCreate` first, then add. Mirrors HereSphere tag handling in [internal/api/heresphere/tag.go](../../../internal/api/heresphere/tag.go).
- **O-counter**: two `<form>`s (`–` / `+`) hitting `/browse/scene/{id}/o/decrement` and `/o/increment`. The number between them is rendered server-side from current state.
- **Organized**: a single `<form>` with one `<button>` toggling state.
- **`[Play in DeoVR]`**: plain `<a href="/deovr/videoData/{id}">`. DeoVR's browser navigates to it and hands off to the player.
- **No HereSphere launch button** in MVP — not needed since rating/edit happens here directly.

### POST endpoint shapes

All POST handlers:
- Read current state from `library.Service.GetScene(id, forceFetch=true)`.
- Apply the requested mutation via the generated GraphQL client.
- Invalidate the cached scene in `library.Service`.
- Respond `303 See Other` to `Referer` (or `/browse/scene/{id}` if absent), preserving the no-JS round trip.
- On error: redirect with `?err=<short-code>`; the detail page renders a small banner.

### Refactor: lifting write logic out of `event.go`

The HereSphere event handler currently translates incoming events into Stash mutations directly. To avoid duplicating this in `/browse` POST handlers, the write logic moves to small, focused methods on `library.Service` (some, like `UpdateRating`, exist already; the rest are added):

- `library.Service.SetRating(ctx, sceneId, rating0to100)`
- `library.Service.SetFavorite(ctx, sceneId, on bool)`
- `library.Service.AddTag(ctx, sceneId, tagName)` — handles `TagCreate` if needed
- `library.Service.RemoveTag(ctx, sceneId, tagName)`
- `library.Service.IncrementO(ctx, sceneId)` / `DecrementO(...)`
- `library.Service.SetOrganized(ctx, sceneId, on bool)`

[internal/api/heresphere/event.go](../../../internal/api/heresphere/event.go) gets shorter — each event becomes one library call. `/browse` POSTs call the same methods.

This keeps a single source of truth for write semantics: invalidation, error mapping, and any future audit logging happen in one place.

## Errors and observability

| Failure | UX |
|---|---|
| Stash unreachable on page load | Page chrome with inline error banner: "Couldn't reach Stash — check stash-vr logs." Sidebar/grid empty. |
| Stash returns 401 | Banner: "Stash rejected the API key." |
| POST mutation fails | Redirect to `Referer` with `?err=<code>`; small error banner above the affected control. |
| Scene id not found | 404 with link back to `/browse`. |
| Page out of range | Empty grid + `[← Page 1]` link. Not an error. |

**Logging**: structured zerolog with `mod=browse` middleware field, plus `videoId=<id>` on per-scene routes. Matches the convention in existing handlers.

## Auth

None added. Inherits the existing chi router's trust model (LAN-trusted). Adding auth on `/browse` is a follow-up if/when needed.

## Scale assumptions

The MVP is designed for:
- Up to ~5,000 scenes total → 30/page pagination is comfortable.
- Up to ~1,000 of each entity type → sidebar lists render to <500KB HTML; client-side filter input keeps lists usable.

Above those thresholds, pagination on the sidebar columns and async `<datalist>` loading are the right follow-ups. Not in MVP.

## Testing

- The project has no test suite (per CLAUDE.md). MVP follows that convention; verification is manual.
- `go vet ./...` and `go build ./...` must pass.
- Manual verification checklist (each item must be exercised before declaring done):
  1. `GET /browse` renders without errors; sidebar has all three tabs populated.
  2. Click each sidebar tab → switches without losing tab state on subsequent clicks.
  3. Sidebar filter input filters rows.
  4. Click a performer name → grid refilters; sidebar persists; active row highlighted.
  5. Click a studio name → same.
  6. Click a tag name → same.
  7. Pagination Prev/Next works on `/browse` and entity-filtered routes; `?page` is preserved across `?tab` changes.
  8. Click a scene card → detail page loads with correct metadata.
  9. Quick-play "▶" overlay → DeoVR player launches.
  10. On detail page: set rating; reload; rating persists; visible in Stash web UI.
  11. Toggle favorite; verify in Stash that the favorite tag was added/removed.
  12. Add an existing tag via datalist; verify in Stash.
  13. Add a brand-new tag (typed); verify Stash creates it and adds it to the scene.
  14. Remove a tag chip; verify in Stash.
  15. Increment / decrement O-counter; verify in Stash.
  16. Toggle organized; verify in Stash.
  17. Click `[Play in DeoVR]`; verify DeoVR player launches.
  18. With Stash unreachable: page renders with error banner, no panic.

## Out of scope (explicit follow-ups)

So they aren't lost during implementation:

1. Sort controls on the grid (date / rating / play count / duration).
2. Combined filters (performer X AND tag Y AND rating ≥ 4).
3. Global search.
4. Sidebar pagination (when entity counts exceed ~1,000 per type).
5. Filter-toggles sidebar à la SLR (POV / 4K / 8K / 120fps / etc.) — likely driven by tag presence.
6. Markers / timestamps editor on the detail page.
7. HereSphere launch button on the detail page.
8. Auth on `/browse` routes.
9. Bulk operations (multi-select rate / tag).
10. Sidebar count refresh after writes.
11. Async `<datalist>` for tag autocomplete at scale.
12. Mobile-narrow / responsive collapse of the sidebar.
