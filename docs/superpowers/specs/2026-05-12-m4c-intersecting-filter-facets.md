# M4c-followup design: intersecting VR filter facets

**Date:** 2026-05-12
**Status:** Approved by user (brainstorming session 2026-05-12).
**Predecessors:** [M4c design: in-VR search/browse](2026-05-09-m4c-in-vr-search.md), [M4c Browse panel — corrective redesign](2026-05-09-m4c-browse-redesign.md)
**Successors:** Writing plan + implementation for facet narrowing.

---

## 1. Context

M4c's VR filter picker currently loads global option lists from
`/browse/filter-options/{kind}` and caches them once. Selecting
`performer=Alice` narrows the grid, but the visible performer / studio / tag
rows do not react; they still show the full library-wide lists. That breaks
the user's mental model of faceted browsing. The requested behavior is:

- pick **Alice** → only performers who share at least one scene with Alice stay
  visible
- pick **Outdoor** too → all three entity columns recompute against
  `Alice AND Outdoor`
- selected rows remain visible so the user can see and remove the active
  filters even if the current combination yields zero scenes

The obvious server-driven fix would re-fetch constrained options on every tap,
but the user correctly called out the VR UX cost: repeated round trips would
make the picker feel laggy. This follow-up moves the intersection work local so
the picker reacts immediately after one upfront fetch.

## 2. Goal & non-goals

**Goal:** In the in-VR browse panel, make the visible **Performer**,
**Studio**, and **Tag** rows reflect the current matching scene set using the
same AND semantics as the grid. After the first index load, every entity-filter
tap should update the facet columns with no per-tap network call.

**Success criteria:**

1. Selecting a performer narrows all three entity columns to scenes containing
   that performer.
2. Adding a tag or studio further narrows all three entity columns using AND
   semantics.
3. Already-selected entity rows remain pinned in their selected sub-list even
   when the current combination yields zero matching scenes.
4. Filter-column updates feel immediate after the initial index fetch.
5. If the index fetch fails, the picker falls back to the current global-option
   behavior instead of breaking.

**Non-goals:**

- Replacing `/browse/grid` with client-side tile filtering. Grid fetch / paging
  stays server-backed.
- Making free-text `q` narrow facet visibility. `q` keeps its existing
  server-side semantics on the grid only.
- Fixing favorite-filter semantics in the grid. The current `/browse/grid`
  handler ignores `favorite`; this change does not widen that scope.
- Adding new filter kinds, saved-filter integration, or sort controls.

## 3. UX / behavior rules

### 3.1 Matching-set rule

The client computes a current matching scene set from the active filters using
the same semantics as M4c's grid:

- performers: AND across selected performer IDs
- studios: AND across selected studio IDs
- tags: AND across selected tag IDs
- stars: minimum stars threshold
- o-count: minimum o-count threshold

`favorite` and free-text `q` are **not** included in facet narrowing in this
slice, because `/browse/grid` does not currently enforce `favorite`, and `q`
uses Stash's full-text search semantics server-side. Including either locally
would make the columns disagree with the tile results.

### 3.2 Column derivation

For each entity column:

1. Start from the current matching scene set.
2. Collect the entity IDs of that kind present in those scenes.
3. Render the regular list from the existing server-provided sort order,
   filtered down to those collected IDs.
4. Exclude already-selected IDs from the regular list.
5. Render selected IDs in the blue selected sub-list exactly as today, in the
   user's selection order.

Example:

- Selected filters: `performer=[Alice]`
- Matching scenes: all scenes containing Alice
- Visible performers: Alice's collaborators from those scenes
- Visible studios / tags: studios and tags present on Alice's scenes

Then:

- Selected filters: `performer=[Alice]`, `tag=[Outdoor]`
- Matching scenes: scenes containing Alice **and** Outdoor
- All three columns are recomputed from that smaller scene set

If the current combination yields zero matching scenes, the regular lists are
empty and only the selected sub-lists remain visible.

## 4. Data model

The VR client gets one compact facet index for the session. It contains only
what the picker needs, not full scene detail:

```json
{
  "performers": [{ "id": "1", "name": "Alice" }],
  "studios": [{ "id": "7", "name": "Studio X" }],
  "tags": [{ "id": "9", "name": "Outdoor" }],
  "scenes": [
    {
      "id": "123",
      "performerIds": ["1", "2"],
      "studioIds": ["7"],
      "tagIds": ["9", "10"],
      "stars": 4,
      "oCount": 2
    }
  ]
}
```

Rules:

- `performers`, `studios`, and `tags` are the display catalogs. Their order is
  the order the picker preserves when narrowing regular rows.
- `scenes` contains only membership and scalar-filter data needed for local
  intersection.
- Scene titles, covers, stream URLs, captions, and other detail stay out of
  this payload.
- Tag inclusion follows the same exclusion rules as today's tag picker:
  skip tags hidden by `EXCLUDE_SORT_NAME` and ancestor-injected tags.
- `studioIds` is an array, even though scenes are effectively single-studio
  today, so the client and server stay structurally aligned with the AND logic.

## 5. Backend shape

Add a new endpoint:

- `GET /browse/filter-index`

Behavior:

- Returns the compact facet index described above.
- Builds the entity catalogs using the same sorted sources the current picker
  uses (`fetchPerformers`, `fetchStudios`, `fetchTags`), so the narrowed lists
  keep today's ordering.
- Builds the per-scene membership rows from a dedicated scene-membership query
  (or equivalent server-side projection) that includes only scene ID,
  performer IDs, studio ID(s), selectable tag IDs, `rating100`, and `o_counter`.
- Does not depend on the current active filters; this is a one-time session
  dataset.

The existing `GET /browse/filter-options/{kind}` route stays in place as a
fallback / compatibility path, but the VR picker stops depending on it in the
happy path.

## 6. Client changes

### 6.1 Loading

When the user first opens the VR filters area, the client:

1. Starts fetching `/browse/filter-index` if it is not already cached.
2. Shows a simple loading state in the three entity columns.
3. On success, caches the index in memory for the rest of the page session.
4. On failure, marks the index unavailable and falls back to the current
   `/browse/filter-options/{kind}` flow.

### 6.2 State

`browseState` grows a small facet-index state bucket, for example:

```js
facetIndex: null,
facetIndexStatus: 'idle' // 'idle' | 'loading' | 'ready' | 'failed'
```

The existing active-filter state (`browseState.filters` and
`browseState.filterNames`) stays the source of truth for selections.

### 6.3 Computation

When performer / studio / tag / stars / o-count changes:

1. Recompute the matching scene subset from `facetIndex.scenes`.
2. Recompute visible entity IDs for performer / studio / tag from that subset.
3. Re-render the three columns from the filtered catalogs.
4. Re-fetch the grid through the existing `/browse/grid` path, exactly as
   today.

This preserves the current tile-fetch architecture while removing the facet
latency that a server round trip on every tap would introduce.

## 7. Risks & guardrails

- **Large libraries:** the facet index is a one-time cost. Keeping it compact
  is the main mitigation; do not serialize fields the picker does not use.
- **Grid / facet drift:** this spec explicitly limits local facet narrowing to
  filters the grid already enforces (`performer`, `studio`, `tag`, `stars`,
  `o-count`). `favorite` and `q` stay out to avoid disagreement.
- **Fallback:** if `/browse/filter-index` fails, the user still gets the
  current picker behavior rather than a broken UI.

## 8. Files likely touched

- `internal/api/browse/router.go`
- `internal/api/browse/data.go`
- `internal/api/browse/` (new filter-index handler + supporting query/projection)
- `internal/stash/gql/documents/query.graphql`
- `internal/stash/gql/generated.go` (regenerated)
- `internal/static/browse_scene.gohtml`

## 9. Manual verification

1. Open VR browse panel, then filters. Confirm entity columns show a loading
   state once, then populate.
2. Select one performer. Confirm grid narrows and all three entity columns
   shrink to that performer's scene neighborhood.
3. Add a tag. Confirm the columns shrink again using AND semantics.
4. Remove the tag. Confirm the columns expand back to the previous performer-only
   neighborhood.
5. Choose a performer + tag combination with no shared scenes. Confirm regular
   lists empty and selected rows remain pinned / removable.
6. Reload the page with the filter index intentionally broken. Confirm the
   picker falls back to the current global lists.
