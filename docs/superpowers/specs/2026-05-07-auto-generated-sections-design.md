# Auto-generated Sections — Design Spec

**Date:** 2026-05-07
**Status:** Approved (pending implementation plan)
**Scope:** Tier 1 of the broader stash-vr / HereSphere improvement roadmap.

## Goal

Generate HereSphere library sections directly from Stash data, so users no longer need a corresponding saved filter in Stash for every browsing axis they want. Three section *kinds* are in scope for v1:

- **Per-performer** — one section per performer above a configurable scene-count threshold.
- **Per-tag** — top-N tags by `scene_count`, one section each.
- **Aggregates** — single-section "shelves": Recently Added, Recently Played, Highly Rated, Unwatched.

Per-studio sections, configurable section-order policies (Tier 3), section-structure caching (Tier 4), play-history-driven ordering, and performer images are explicitly out of scope.

## Non-goals

- No new HTTP endpoints.
- No HereSphere protocol changes — all output flows through the existing `/heresphere` index.
- No breaking changes to existing `config.json` schema.
- No DeoVR changes (the auto-section sources are HereSphere-only for v1, and DeoVR's section model is different enough that it's a separate exercise).
- No tests added beyond what currently exists in stash-vr (none) — verification is manual against a running Stash. Adding Go test infrastructure is a separate cycle.

## Architectural shape

Auto-sections are **first-class persistent records** in the existing `UserConfig.Filters` list. Each record's identity is determined by an opaque ID:

| `ID` shape                | Meaning |
| ------------------------- | ------- |
| `<numeric>`               | Existing Stash saved-filter id (unchanged). |
| `auto:perf:<stash-id>`    | Per-performer auto-section. |
| `auto:tag:<stash-id>`     | Per-tag auto-section. |
| `auto:agg:<slug>`         | Aggregate (`recent_added`, `recent_played`, `highly_rated`, `unwatched`). |

Reusing the `Filter` struct unchanged means existing rename / disable / reorder logic, the existing `/filters` web UI, and `buildFiltersByUserConfig` ordering all continue to work without modification.

`Filter.Name` semantics are unchanged: empty means "use the entity's current name from Stash", non-empty means "user has renamed this section, honor it." This is consistent with how renames already work for saved filters.

### Reconciliation (the sync step)

On every index build, before sections materialize, run a reconcile pass:

1. **Compute the expected set** of `auto:*` IDs from current Stash data and env config:
   - **Per-performer**: `findPerformers` ordered by `scene_count` desc, filtered to `scene_count >= MIN_SCENES_PER_PERFORMER`, capped at `MAX_PERFORMER_SECTIONS`. Emit `auto:perf:<id>` for each.
   - **Per-tag**: reuse existing `FindTags` (already exposes `scene_count`). Take top `TOP_N_TAGS` by `scene_count`. Skip tags whose `sort_name == EXCLUDE_SORT_NAME` (existing convention). Emit `auto:tag:<id>` for each.
   - **Aggregates**: emit a fixed set of `auto:agg:*` IDs for each enabled aggregate sub-toggle.

2. **Reconcile against the existing `auto:*` records** in `UserConfig.Filters`:
   - Existing record whose ID is in the expected set → keep (preserves rename, position, `Disabled` flag).
   - Expected ID not yet in config → append a new default record at the end of its kind group (see Default ordering below).
   - Existing record whose ID is *not* in the expected set → **hard-prune**.

3. **Persist `UserConfig`** if anything changed and `CONFIG_PATH` is set.

4. **Materialize** each non-disabled record into a `library.Section`.

This piggybacks on the existing `singleflight`-deduped `GetSections` path. Cost on each unique index build: 2 extra GraphQL queries (`findPerformers`, `findTags`) plus N `FindSceneIdsByFilter` calls (one per auto-section), all parallelizable like saved filters already are.

### Materialization

For each non-disabled record, build the `Section{Name, Ids}`:

| Record kind                 | `Name` source                                    | `Ids` source |
| --------------------------- | ------------------------------------------------ | ------------ |
| Saved filter                | Stash filter name (or user override)             | Existing path. |
| `auto:perf:<id>`            | Performer name (or user override)                | `FindSceneIdsByFilter` with `scene_filter: { performers: { value: [<id>], modifier: INCLUDES } }`, `sort: "date"` desc. |
| `auto:tag:<id>`             | Tag name (or user override)                      | `FindSceneIdsByFilter` with `scene_filter: { tags: { value: [<id>], modifier: INCLUDES, depth: -1 } }`, `sort: "date"` desc. |
| `auto:agg:recent_added`     | `"Recently Added"` (or override)                 | `FindSceneIdsByFilter` no scene_filter, `sort: "created_at"` desc, `per_page: AGGREGATE_LIMIT`. |
| `auto:agg:recent_played`    | `"Recently Played"` (or override)                | `sort: "last_played_at"` desc, `per_page: AGGREGATE_LIMIT`, `scene_filter: { play_count: { value: 0, modifier: GREATER_THAN } }`. |
| `auto:agg:highly_rated`     | `"Highly Rated"` (or override)                   | `sort: "rating"` desc, `per_page: AGGREGATE_LIMIT`, `scene_filter: { rating100: { value: HIGHLY_RATED_THRESHOLD, modifier: GREATER_THAN_OR_EQUAL } }`. |
| `auto:agg:unwatched`        | `"Unwatched"` (or override)                      | `sort: "created_at"` desc, `per_page: AGGREGATE_LIMIT`, `scene_filter: { play_count: { value: 0, modifier: EQUALS } }`. |

All materialization runs in parallel inside the existing `getSectionsByFilters` `sync.WaitGroup` loop. Per-section failures log + skip without aborting the whole index, matching saved-filter behavior. An auto-section materializing to 0 scenes is dropped from the response (existing `slices.DeleteFunc` at [internal/library/sections.go:124](../../../internal/library/sections.go#L124)) — but the `config.json` record is kept (empty result ≠ entity below threshold).

### Default ordering (fresh install)

Tier 3 (full configurable ordering) is out of scope. We only set the *initial* placement of newly-materialized records. Once a record exists, the user can drag it anywhere in the existing UI and that position is preserved on subsequent reconciles.

When a brand-new auto-section is appended to the config, it goes to the end of its **kind group**, with kind groups ordered:

1. Saved-filter records (existing, frontpage-ordered, unchanged).
2. Aggregates — emitted in fixed slug order: `recent_added`, `recent_played`, `highly_rated`, `unwatched`.
3. Per-performer auto-sections — in `findPerformers` order (scene_count desc).
4. Per-tag auto-sections — in `FindTags` order (scene_count desc).

Reasoning: saved filters first because they're the user's explicit curation. Aggregates second because they're cheap, single-section, and high-utility. Per-performer / per-tag last because they fan out and are noisiest.

**Caveat — frontpage vs user-config code path.** Today's `getFilters` chooses between `buildFiltersByFrontpage` and `buildFiltersByUserConfig` based on whether `UserConfig.Filters` is non-empty. Once auto-sections become first-class records, simply reconciling them into `UserConfig.Filters` would flip every install into the user-config path, suppressing the Stash frontpage ordering for saved filters. To avoid that:

- `getFilters` continues to source saved filters from Stash (frontpage-ordered by default) — same as today.
- `UserConfig.Filters` is consulted only for: (a) explicit position overrides on saved filters (existing behavior) and (b) auto-section records (new). The "user has touched the filter list, switch to manual ordering" trigger should look at saved-filter entries specifically — not be flipped just because auto-section records exist.

Concrete implementation: split `UserConfig.Filters` interpretation into "saved-filter overrides" (numeric IDs) and "auto-section records" (`auto:*` IDs). The former retains today's all-or-nothing semantics; the latter is purely additive.

## Configuration surface

All new config is env-only. No new schema in `config.json` beyond the existing `Filter` list (which already supports auto-section records via the ID prefix).

| Env var                       | Default | Purpose |
| ----------------------------- | ------- | ------- |
| `AUTO_SECTIONS_PERFORMERS`    | `false` | Master enable for per-performer sections. |
| `MIN_SCENES_PER_PERFORMER`    | `5`     | Threshold to materialize a performer auto-section. |
| `MAX_PERFORMER_SECTIONS`      | `50`    | Hard cap (ranked by `scene_count` desc). Prevents HereSphere overload on large libraries. |
| `AUTO_SECTIONS_TAGS`          | `false` | Master enable for per-tag sections. |
| `TOP_N_TAGS`                  | `20`    | Max number of tag auto-sections (ranked by `scene_count` desc). |
| `AUTO_SECTIONS_AGGREGATES`    | `false` | Master enable for the aggregate set. |
| `AGGREGATE_RECENT_ADDED`      | `true`  | Sub-toggle (only honored if aggregates master is on). |
| `AGGREGATE_RECENT_PLAYED`     | `true`  | Sub-toggle. |
| `AGGREGATE_HIGHLY_RATED`      | `true`  | Sub-toggle. |
| `AGGREGATE_UNWATCHED`         | `true`  | Sub-toggle. |
| `HIGHLY_RATED_THRESHOLD`      | `80`    | Stash 0–100 rating; scenes ≥ threshold qualify. |
| `AGGREGATE_LIMIT`             | `100`   | Per-aggregate max scenes. |

`EXCLUDE_SORT_NAME` (existing) is honored for tag auto-sections — tags marked hidden in Stash don't get sections.

Three reasons for opt-in defaults:

1. The brainstorm doc explicitly flagged "auto-section generation should be opt-in until battle-tested."
2. Existing users won't see surprise sections after upgrading.
3. Easier rollback: flipping a master switch off causes the next reconcile to hard-prune all corresponding `auto:*` records from `config.json`.

## Naming

Default (un-renamed) auto-section names use the entity's name as-is, no prefix:

- Per-performer: `<Performer Name>`
- Per-tag: `<Tag Name>`
- Aggregates: fixed labels — `Recently Added`, `Recently Played`, `Highly Rated`, `Unwatched`.

No prefix because HereSphere shows section names verbatim, and prefixing every auto-section with `Performer:` or `Tag:` adds visual noise without disambiguation benefit.

Naming collisions with saved-filter names are tolerated. HereSphere's index allows duplicate library names; the user can rename either side via the web UI.

## Edge cases

- **Stash unreachable mid-reconcile**: if `findPerformers` or `findTags` fails, skip auto-section reconcile entirely for this index build. Existing `auto:*` records remain as-is until the next successful reconcile. Per-section materialization failures log + skip individually.
- **No `CONFIG_PATH` set**: reconcile still happens in-memory each request (the existing `User()` returns a transient config). User customizations don't survive a restart, matching today's saved-filter rename behavior.
- **Performer renamed in Stash**: identity is by stable `<stash-id>`, so the record is preserved. Default `Name` resolves through Stash on each materialize, so the rename is visible. User overrides (non-empty `Filter.Name`) continue to win.
- **Performer deleted in Stash**: the `auto:perf:<id>` is hard-pruned on the next reconcile.
- **Migration**: purely additive. Old stash-vr versions reading a new `config.json` would not understand `auto:*` IDs; this is not a downgrade scenario we support.

## File layout

### New files

- `internal/library/autosection.go` — reconcile pass, expected-set computation, hard-prune logic. Pure functions where possible; only the persistence step touches `config.User`.
- `internal/library/autosection_materialize.go` — materializers per kind (performer / tag / aggregate). Each takes a `*Filter` record and returns `(Section, error)`.

### Modified files

- `internal/stash/gql/documents/query.graphql` — new `FindPerformers` op (with `scene_count`, ordered by `scene_count` desc, paginated). Existing `FindTags` already suffices.
- `internal/library/sections.go` — `getFilters` extended to merge `auto:*` records returned by reconcile alongside saved filters. `getSectionsByFilters` dispatches by ID prefix to the right materializer.
- `internal/config/application.go` — add the env vars from the configuration surface table.
- `internal/api/web/index.gohtml` and the `/filters` POST handler — auto-section records appear in the existing list and are reorderable / disable-able / renameable like saved filters. Optional: small "auto" badge next to auto-section rows so users can tell them apart. (Defer if it adds meaningful UI complexity.)

### Unchanged

- `internal/config/user.go` — no schema change; `Filter` struct reused as-is.
- `internal/api/heresphere/*` — auto-sections flow through the existing index path with no HereSphere-side awareness.

## Code flow on an index request

```
GET /heresphere
  → library.GetSections (singleflight-deduped)
      → reconcileAutoSections(ctx)              [new]
          ├─ findPerformers (if performers enabled)
          ├─ findTags (if tags enabled, reuses existing call)
          ├─ compute expected auto:* IDs
          ├─ merge into UserConfig.Filters (add new, prune missing)
          └─ persist if CONFIG_PATH set
      → getFilters (existing, now sees auto:* records)
      → getSectionsByFilters
          ├─ saved-filter records → existing path
          └─ auto:* records → new materializers (parallel)
      → rebuild scene cache (existing)
  → buildIndex (existing) → JSON response
```

## Out of scope (deferred to later cycles)

- Section ordering customization beyond the existing UI's reorder support (Tier 3).
- Section-structure response caching with TTL (Tier 4).
- Per-studio sections (opted out for v1).
- Adaptive ordering by play history (Tier 3).
- Performer / tag images or avatars in the section list.
- DeoVR-side equivalent (separate cycle).
- Go test infrastructure.
