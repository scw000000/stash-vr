# Auto-generated HereSphere Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Tier 1 auto-generated HereSphere sections (per-performer, per-tag, aggregates) as first-class records in `UserConfig.Filters`, materialized from existing Stash data.

**Architecture:** Add a reconcile pass to `library.Service.GetSections` that computes the expected set of `auto:*` records from Stash queries (`findPerformers`, `findTags`) and env-driven enables, then merges into `UserConfig.Filters` with hard-prune semantics. Materialize each record at section-build time via the existing `FindSceneIdsByFilter` call, dispatched by an ID-prefix discriminator. Reuse the existing `/filters` web UI for rename / disable / reorder.

**Tech Stack:** Go 1.24, genqlient (Stash GraphQL client gen), chi router, viper (config), zerolog. **No automated tests** — per spec, the codebase has no test infrastructure and adding it is a separate cycle. Each task verifies via `go vet ./...`, `go build ./...`, and (for behavioral tasks) curl against a locally running stash-vr + Stash.

**Spec:** [docs/superpowers/specs/2026-05-07-auto-generated-sections-design.md](../specs/2026-05-07-auto-generated-sections-design.md)

---

## File Structure

**New files:**
- `internal/library/autosection.go` — record kinds, ID parsing, expected-set computation, reconcile loop.
- `internal/library/autosection_materialize.go` — per-kind materializers + dispatcher.

**Modified files:**
- `internal/stash/gql/documents/query.graphql` — add `FindPerformersWithSceneCount` op.
- `internal/stash/gql/generated.go` — regenerated via `go generate ./cmd/stash-vr` (do not hand-edit).
- `internal/library/sections.go` — `getFilters` returns a typed-union list; `getSectionsByFilters` dispatches; `GetSections` calls `reconcileAutoSections` first.
- `internal/config/application.go` — 12 new env vars (see Task 1).
- `internal/api/web/web.go` — `filterOverrideRows` includes auto records and resolves their display names.

**Unchanged:**
- `internal/config/user.go` — `Filter` struct reused as-is; ID prefix encodes kind.
- `internal/static/index.gohtml` — existing form (`id`, `sourceName`, `targetName`, `disabled` fields) is shape-compatible.
- `internal/api/heresphere/*` — auto sections flow through the existing index path with no HereSphere-side awareness.

**Verification environment assumed throughout:**
- Stash running at `http://localhost:9999`
- Stash-VR built via `go build -o stash-vr.exe ./cmd/stash-vr` (Windows) or `go build -o stash-vr ./cmd/stash-vr` (POSIX)
- Stash-VR run with `STASH_GRAPHQL_URL=http://localhost:9999/graphql` plus the env vars under test

---

### Task 1: Add env vars for auto-section configuration

**Files:**
- Modify: `internal/config/application.go`

- [ ] **Step 1: Add env-key constants**

In `internal/config/application.go`, after the existing `envKeyGenerateSummaryIds = "GENERATE_SUMMARY_IDS"` line in the `const` block, append:

```go
	envKeyAutoSectionsPerformers = "AUTO_SECTIONS_PERFORMERS"
	envKeyMinScenesPerPerformer  = "MIN_SCENES_PER_PERFORMER"
	envKeyMaxPerformerSections   = "MAX_PERFORMER_SECTIONS"
	envKeyAutoSectionsTags       = "AUTO_SECTIONS_TAGS"
	envKeyTopNTags               = "TOP_N_TAGS"
	envKeyAutoSectionsAggregates = "AUTO_SECTIONS_AGGREGATES"
	envKeyAggregateRecentAdded   = "AGGREGATE_RECENT_ADDED"
	envKeyAggregateRecentPlayed  = "AGGREGATE_RECENT_PLAYED"
	envKeyAggregateHighlyRated   = "AGGREGATE_HIGHLY_RATED"
	envKeyAggregateUnwatched     = "AGGREGATE_UNWATCHED"
	envKeyHighlyRatedThreshold   = "HIGHLY_RATED_THRESHOLD"
	envKeyAggregateLimit         = "AGGREGATE_LIMIT"
```

- [ ] **Step 2: Extend `ApplicationConfig` struct**

In the same file, add the following fields to `ApplicationConfig` (after `GenerateSummaryIds`):

```go
	AutoSectionsPerformers bool
	MinScenesPerPerformer  int
	MaxPerformerSections   int
	AutoSectionsTags       bool
	TopNTags               int
	AutoSectionsAggregates bool
	AggregateRecentAdded   bool
	AggregateRecentPlayed  bool
	AggregateHighlyRated   bool
	AggregateUnwatched     bool
	HighlyRatedThreshold   int
	AggregateLimit         int
```

- [ ] **Step 3: Bind pflags inside `Init()`**

In `Init()`, after the existing `GenerateSummaryIds` pflag binding (around the `pflag.Parse()` call but before it), add:

```go
	pflag.Bool(envKeyAutoSectionsPerformers, false, "Generate one HereSphere section per performer above MIN_SCENES_PER_PERFORMER")
	_ = viper.BindPFlag(envKeyAutoSectionsPerformers, pflag.Lookup(envKeyAutoSectionsPerformers))

	pflag.Int(envKeyMinScenesPerPerformer, 5, "Minimum scene count for a performer to get an auto-section")
	_ = viper.BindPFlag(envKeyMinScenesPerPerformer, pflag.Lookup(envKeyMinScenesPerPerformer))

	pflag.Int(envKeyMaxPerformerSections, 50, "Cap on per-performer auto-sections (ranked by scene_count desc)")
	_ = viper.BindPFlag(envKeyMaxPerformerSections, pflag.Lookup(envKeyMaxPerformerSections))

	pflag.Bool(envKeyAutoSectionsTags, false, "Generate one HereSphere section per tag in the top TOP_N_TAGS by scene_count")
	_ = viper.BindPFlag(envKeyAutoSectionsTags, pflag.Lookup(envKeyAutoSectionsTags))

	pflag.Int(envKeyTopNTags, 20, "Number of tag auto-sections")
	_ = viper.BindPFlag(envKeyTopNTags, pflag.Lookup(envKeyTopNTags))

	pflag.Bool(envKeyAutoSectionsAggregates, false, "Generate aggregate sections (Recently Added/Played/Highly Rated/Unwatched)")
	_ = viper.BindPFlag(envKeyAutoSectionsAggregates, pflag.Lookup(envKeyAutoSectionsAggregates))

	pflag.Bool(envKeyAggregateRecentAdded, true, "Sub-toggle for the Recently Added aggregate section")
	_ = viper.BindPFlag(envKeyAggregateRecentAdded, pflag.Lookup(envKeyAggregateRecentAdded))

	pflag.Bool(envKeyAggregateRecentPlayed, true, "Sub-toggle for the Recently Played aggregate section")
	_ = viper.BindPFlag(envKeyAggregateRecentPlayed, pflag.Lookup(envKeyAggregateRecentPlayed))

	pflag.Bool(envKeyAggregateHighlyRated, true, "Sub-toggle for the Highly Rated aggregate section")
	_ = viper.BindPFlag(envKeyAggregateHighlyRated, pflag.Lookup(envKeyAggregateHighlyRated))

	pflag.Bool(envKeyAggregateUnwatched, true, "Sub-toggle for the Unwatched aggregate section")
	_ = viper.BindPFlag(envKeyAggregateUnwatched, pflag.Lookup(envKeyAggregateUnwatched))

	pflag.Int(envKeyHighlyRatedThreshold, 80, "Stash 0-100 rating; scenes >= threshold qualify for Highly Rated")
	_ = viper.BindPFlag(envKeyHighlyRatedThreshold, pflag.Lookup(envKeyHighlyRatedThreshold))

	pflag.Int(envKeyAggregateLimit, 100, "Per-aggregate max scene count")
	_ = viper.BindPFlag(envKeyAggregateLimit, pflag.Lookup(envKeyAggregateLimit))
```

- [ ] **Step 4: Read values into `applicationConfig` after `viper.AutomaticEnv()`**

After the existing `applicationConfig.GenerateSummaryIds = viper.GetBool(envKeyGenerateSummaryIds)` line, append:

```go
	applicationConfig.AutoSectionsPerformers = viper.GetBool(envKeyAutoSectionsPerformers)
	applicationConfig.MinScenesPerPerformer = viper.GetInt(envKeyMinScenesPerPerformer)
	applicationConfig.MaxPerformerSections = viper.GetInt(envKeyMaxPerformerSections)
	applicationConfig.AutoSectionsTags = viper.GetBool(envKeyAutoSectionsTags)
	applicationConfig.TopNTags = viper.GetInt(envKeyTopNTags)
	applicationConfig.AutoSectionsAggregates = viper.GetBool(envKeyAutoSectionsAggregates)
	applicationConfig.AggregateRecentAdded = viper.GetBool(envKeyAggregateRecentAdded)
	applicationConfig.AggregateRecentPlayed = viper.GetBool(envKeyAggregateRecentPlayed)
	applicationConfig.AggregateHighlyRated = viper.GetBool(envKeyAggregateHighlyRated)
	applicationConfig.AggregateUnwatched = viper.GetBool(envKeyAggregateUnwatched)
	applicationConfig.HighlyRatedThreshold = viper.GetInt(envKeyHighlyRatedThreshold)
	applicationConfig.AggregateLimit = viper.GetInt(envKeyAggregateLimit)
```

- [ ] **Step 5: Verify build and vet**

Run: `go vet ./... && go build ./...`
Expected: exit 0, no warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/config/application.go
git commit -m "config: add env vars for auto-section generation"
```

---

### Task 2: Add `FindPerformersWithSceneCount` GraphQL query

**Files:**
- Modify: `internal/stash/gql/documents/query.graphql`
- Regenerate: `internal/stash/gql/generated.go` (via `go generate`)

- [ ] **Step 1: Add the new query operation**

Append the following to the END of `internal/stash/gql/documents/query.graphql` (after `query FindSampleSceneCover` and before the `fragment SavedFilterParts` block):

```graphql
query FindPerformersWithSceneCount(
    $min_scene_count: Int!, $per_page: Int!){
    findPerformers(
        performer_filter: {scene_count: {value: $min_scene_count, modifier: GREATER_THAN_OR_EQUAL}}
        filter: {sort: "scenes_count", direction: DESC, per_page: $per_page}
    ){
        performers {
            id, name
            scene_count
        }
    }
}
```

**Note on the `scene_count` modifier:** the genqlient enum (`internal/stash/gql/generated.go:36-65`) does NOT include `GREATER_THAN_OR_EQUAL` — the Stash schema does, but only certain modifiers are exposed. If the regenerate step in Step 3 fails with "unknown enum value", change the modifier to `GREATER_THAN` and pass `min_scene_count - 1` from Go (semantically equivalent for integer scene counts). The genqlient enum file lists the valid modifiers — check there if uncertain.

- [ ] **Step 2: Verify scene_count and sort string against the schema**

Run: `grep -n "scene_count" internal/stash/gql/schema/local.graphql | head -5`
Expected: see `scene_count: Int!` on a `Performer` field. (Schema is at `internal/stash/gql/schema/local.graphql:1029` for the Performer block.)

The sort string `"scenes_count"` is the Stash convention for performer scene-count sorting. If a later runtime check shows zero results, try `"scene_count"` instead — Stash has historically used both spellings.

- [ ] **Step 3: Regenerate the genqlient client**

Run: `go generate ./cmd/stash-vr`
Expected: no errors. The file `internal/stash/gql/generated.go` is rewritten and now contains a `FindPerformersWithSceneCount` function and corresponding response types.

If genqlient errors on the modifier enum, adjust per Step 1's note and re-run.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 5: Sanity check the generated function exists**

Run: `grep -n "func FindPerformersWithSceneCount" internal/stash/gql/generated.go`
Expected: one match showing the function signature.

- [ ] **Step 6: Commit**

```bash
git add internal/stash/gql/documents/query.graphql internal/stash/gql/generated.go
git commit -m "stash/gql: add FindPerformersWithSceneCount query"
```

---

### Task 3: Define auto-section record types and ID parsing

**Files:**
- Create: `internal/library/autosection.go`

- [ ] **Step 1: Create the file with kind enum, ID format, and parsing helpers**

Write the following to `internal/library/autosection.go`:

```go
package library

import (
	"strings"
)

type autoKind int

const (
	autoKindUnknown autoKind = iota
	autoKindPerformer
	autoKindTag
	autoKindAggregate
)

const (
	autoIDPrefix      = "auto:"
	autoPerfPrefix    = "auto:perf:"
	autoTagPrefix     = "auto:tag:"
	autoAggPrefix     = "auto:agg:"
	aggSlugRecentAdded  = "recent_added"
	aggSlugRecentPlayed = "recent_played"
	aggSlugHighlyRated  = "highly_rated"
	aggSlugUnwatched    = "unwatched"
)

type parsedAutoID struct {
	Kind  autoKind
	Value string // entity stash ID for perf/tag, slug for aggregate
}

func isAutoID(id string) bool {
	return strings.HasPrefix(id, autoIDPrefix)
}

func parseAutoID(id string) (parsedAutoID, bool) {
	switch {
	case strings.HasPrefix(id, autoPerfPrefix):
		v := strings.TrimPrefix(id, autoPerfPrefix)
		if v == "" {
			return parsedAutoID{}, false
		}
		return parsedAutoID{Kind: autoKindPerformer, Value: v}, true
	case strings.HasPrefix(id, autoTagPrefix):
		v := strings.TrimPrefix(id, autoTagPrefix)
		if v == "" {
			return parsedAutoID{}, false
		}
		return parsedAutoID{Kind: autoKindTag, Value: v}, true
	case strings.HasPrefix(id, autoAggPrefix):
		v := strings.TrimPrefix(id, autoAggPrefix)
		if v == "" {
			return parsedAutoID{}, false
		}
		return parsedAutoID{Kind: autoKindAggregate, Value: v}, true
	}
	return parsedAutoID{}, false
}

func makePerformerAutoID(stashID string) string { return autoPerfPrefix + stashID }
func makeTagAutoID(stashID string) string       { return autoTagPrefix + stashID }
func makeAggregateAutoID(slug string) string    { return autoAggPrefix + slug }
```

- [ ] **Step 2: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0, no warnings.

- [ ] **Step 3: Commit**

```bash
git add internal/library/autosection.go
git commit -m "library: introduce auto-section record kinds and ID parsing"
```

---

### Task 4: Implement expected-set computation

**Files:**
- Modify: `internal/library/autosection.go`

- [ ] **Step 1: Add imports and the expected-set struct**

Update the import block at the top of `internal/library/autosection.go`:

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/config"
	"stash-vr/internal/stash/gql"
)
```

Then below the existing helpers, add the expected-set type and aggregate-slug helper:

```go
type expectedAutoSection struct {
	ID          string // full auto:* ID
	DefaultName string // resolved entity name (or fixed label for aggregates)
}

func defaultAggregateName(slug string) string {
	switch slug {
	case aggSlugRecentAdded:
		return "Recently Added"
	case aggSlugRecentPlayed:
		return "Recently Played"
	case aggSlugHighlyRated:
		return "Highly Rated"
	case aggSlugUnwatched:
		return "Unwatched"
	}
	return slug
}
```

- [ ] **Step 2: Add the per-performer expected-set fetch**

Append to `internal/library/autosection.go`:

```go
func expectedPerformerAutoSections(ctx context.Context, client graphql.Client) ([]expectedAutoSection, error) {
	cfg := config.Application()
	if !cfg.AutoSectionsPerformers {
		return nil, nil
	}
	minScenes := cfg.MinScenesPerPerformer
	if minScenes < 1 {
		minScenes = 1
	}
	maxCount := cfg.MaxPerformerSections
	if maxCount < 1 {
		return nil, nil
	}
	resp, err := gql.FindPerformersWithSceneCount(ctx, client, minScenes, maxCount)
	if err != nil {
		return nil, fmt.Errorf("FindPerformersWithSceneCount: %w", err)
	}
	out := make([]expectedAutoSection, 0, len(resp.FindPerformers.Performers))
	for _, p := range resp.FindPerformers.Performers {
		out = append(out, expectedAutoSection{
			ID:          makePerformerAutoID(p.Id),
			DefaultName: p.Name,
		})
	}
	return out, nil
}
```

**Note:** The exact argument names on `gql.FindPerformersWithSceneCount` depend on what genqlient generated in Task 2. If the generated signature differs (e.g., the `Per_page` argument is `*int` instead of `int`), adjust the call accordingly. Check the function signature near `func FindPerformersWithSceneCount(` in `internal/stash/gql/generated.go`.

- [ ] **Step 3: Add the per-tag expected-set fetch**

Append:

```go
func expectedTagAutoSections(ctx context.Context, client graphql.Client) ([]expectedAutoSection, error) {
	cfg := config.Application()
	if !cfg.AutoSectionsTags {
		return nil, nil
	}
	limit := cfg.TopNTags
	if limit < 1 {
		return nil, nil
	}
	// Reuse existing FindTags. Sort by scene_count desc.
	sortStr := "scenes_count"
	dir := gql.SortDirectionEnumDesc
	resp, err := gql.FindTags(ctx, client, nil, &sortStr, &dir)
	if err != nil {
		return nil, fmt.Errorf("FindTags: %w", err)
	}
	excludeSort := config.Application().ExcludeSortName
	out := make([]expectedAutoSection, 0, limit)
	for _, t := range resp.FindTags.Tags {
		if t.TagParts.Sort_name == excludeSort {
			continue
		}
		out = append(out, expectedAutoSection{
			ID:          makeTagAutoID(t.TagParts.Id),
			DefaultName: t.TagParts.Name,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
```

**Note:** `gql.FindTags` already exists (see `internal/stash/gql/documents/query.graphql:27`). The third argument is `tag_filter`, fourth is `sort` (`*string`), fifth is `direction` (`*SortDirectionEnum`). Confirm the precise argument order against the generated function signature before running. The `TagParts` fragment exposes `Id`, `Name`, `Sort_name`. If genqlient generates these as `t.Id`, `t.Name`, `t.Sort_name` (without an intermediate `TagParts` field), drop the `TagParts.` prefix.

- [ ] **Step 4: Add the aggregate expected-set computation**

Append:

```go
func expectedAggregateAutoSections() []expectedAutoSection {
	cfg := config.Application()
	if !cfg.AutoSectionsAggregates {
		return nil
	}
	var out []expectedAutoSection
	if cfg.AggregateRecentAdded {
		out = append(out, expectedAutoSection{
			ID:          makeAggregateAutoID(aggSlugRecentAdded),
			DefaultName: defaultAggregateName(aggSlugRecentAdded),
		})
	}
	if cfg.AggregateRecentPlayed {
		out = append(out, expectedAutoSection{
			ID:          makeAggregateAutoID(aggSlugRecentPlayed),
			DefaultName: defaultAggregateName(aggSlugRecentPlayed),
		})
	}
	if cfg.AggregateHighlyRated {
		out = append(out, expectedAutoSection{
			ID:          makeAggregateAutoID(aggSlugHighlyRated),
			DefaultName: defaultAggregateName(aggSlugHighlyRated),
		})
	}
	if cfg.AggregateUnwatched {
		out = append(out, expectedAutoSection{
			ID:          makeAggregateAutoID(aggSlugUnwatched),
			DefaultName: defaultAggregateName(aggSlugUnwatched),
		})
	}
	return out
}
```

- [ ] **Step 5: Add a wrapper that returns all three groups in kind-group order**

Append:

```go
// computeExpectedAutoSections returns the union of all expected auto:*
// sections in kind-group order: aggregates, then performers, then tags.
// This order is the default placement for newly-materialized records on
// fresh installs (saved filters precede this block).
func computeExpectedAutoSections(ctx context.Context, client graphql.Client) []expectedAutoSection {
	aggs := expectedAggregateAutoSections()
	perfs, err := expectedPerformerAutoSections(ctx, client)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("auto-sections: skipping performer reconcile this build")
		perfs = nil
	}
	tags, err := expectedTagAutoSections(ctx, client)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("auto-sections: skipping tag reconcile this build")
		tags = nil
	}
	out := make([]expectedAutoSection, 0, len(aggs)+len(perfs)+len(tags))
	out = append(out, aggs...)
	out = append(out, perfs...)
	out = append(out, tags...)
	return out
}
```

- [ ] **Step 6: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add internal/library/autosection.go
git commit -m "library: compute expected auto-section set from Stash + env config"
```

---

### Task 5: Implement reconcile (merge expected with UserConfig.Filters)

**Files:**
- Modify: `internal/library/autosection.go`

- [ ] **Step 1: Add the reconcile function**

Append to `internal/library/autosection.go`:

```go
// reconcileAutoSections merges the current expected auto-section set into
// UserConfig.Filters with hard-prune semantics:
//   - Each expected ID already in UserConfig.Filters is preserved verbatim
//     (so user rename / disabled / order is kept).
//   - Each expected ID NOT in UserConfig.Filters is appended at the end of
//     its kind group (which on a fresh install is just the end of the list,
//     since no auto records exist yet).
//   - Each existing auto:* record whose ID is NOT in the expected set is
//     hard-pruned.
// Saved-filter records (numeric IDs) are never touched.
// Returns true if UserConfig was mutated (caller may persist).
func reconcileAutoSections(ctx context.Context, client graphql.Client) bool {
	expected := computeExpectedAutoSections(ctx, client)
	expectedByID := make(map[string]struct{}, len(expected))
	for _, e := range expected {
		expectedByID[e.ID] = struct{}{}
	}

	cur := config.User(ctx)
	out := make([]config.Filter, 0, len(cur.Filters)+len(expected))
	keptAuto := make(map[string]struct{})
	mutated := false

	// Pass 1: keep saved-filter records and still-expected auto records as-is.
	for _, f := range cur.Filters {
		if !isAutoID(f.ID) {
			out = append(out, f)
			continue
		}
		if _, ok := expectedByID[f.ID]; ok {
			out = append(out, f)
			keptAuto[f.ID] = struct{}{}
		} else {
			// Hard-prune: not in expected set anymore.
			mutated = true
			log.Ctx(ctx).Debug().Str("id", f.ID).Msg("auto-sections: pruning record")
		}
	}

	// Pass 2: append newly-expected auto records that weren't already present,
	// preserving the kind-group order from computeExpectedAutoSections.
	for _, e := range expected {
		if _, ok := keptAuto[e.ID]; ok {
			continue
		}
		out = append(out, config.Filter{ID: e.ID})
		mutated = true
		log.Ctx(ctx).Debug().Str("id", e.ID).Str("name", e.DefaultName).Msg("auto-sections: adding record")
	}

	if !mutated {
		return false
	}

	cur.Filters = out
	config.Save(ctx, cur)
	return true
}
```

- [ ] **Step 2: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add internal/library/autosection.go
git commit -m "library: reconcile auto-section records into UserConfig"
```

---

### Task 6: Implement materializers (per-record → Section)

**Files:**
- Create: `internal/library/autosection_materialize.go`

- [ ] **Step 1: Create the file with the dispatcher and shared scaffolding**

Write `internal/library/autosection_materialize.go`:

```go
package library

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"stash-vr/internal/config"
	"stash-vr/internal/stash/gql"
)

// materializeAutoSection turns one auto:* record (taken from
// UserConfig.Filters) into a populated Section. The display Name comes
// from the Filter.Name override if set, else from a current-Stash lookup
// (performer name, tag name) or a fixed label (aggregates).
func materializeAutoSection(ctx context.Context, client graphql.Client, rec config.Filter) (Section, error) {
	parsed, ok := parseAutoID(rec.ID)
	if !ok {
		return Section{}, fmt.Errorf("invalid auto id: %s", rec.ID)
	}
	switch parsed.Kind {
	case autoKindPerformer:
		return materializePerformerSection(ctx, client, rec, parsed.Value)
	case autoKindTag:
		return materializeTagSection(ctx, client, rec, parsed.Value)
	case autoKindAggregate:
		return materializeAggregateSection(ctx, client, rec, parsed.Value)
	}
	return Section{}, fmt.Errorf("unknown auto kind for id: %s", rec.ID)
}

func resolveName(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}
```

- [ ] **Step 2: Add the per-performer materializer**

Append to the same file:

```go
func materializePerformerSection(ctx context.Context, client graphql.Client, rec config.Filter, performerID string) (Section, error) {
	perPage := -1
	sort := "date"
	dir := gql.SortDirectionEnumDesc
	sceneFilter := gql.SceneFilterType{
		Performers: &gql.MultiCriterionInput{
			Modifier: gql.CriterionModifierIncludes,
			Value:    []string{performerID},
		},
	}
	findFilter := gql.FindFilterType{
		Sort:      &sort,
		Direction: &dir,
		Per_page:  &perPage,
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, client, &sceneFilter, &findFilter)
	if err != nil {
		return Section{}, fmt.Errorf("FindSceneIdsByFilter (performer %s): %w", performerID, err)
	}
	ids := make([]string, len(resp.FindScenes.Scenes))
	for i, s := range resp.FindScenes.Scenes {
		ids[i] = s.Id
	}

	// Resolve default name from a one-shot performer lookup. We could cache
	// this from the reconcile pass, but a fresh lookup per index build is
	// cheap (singleflight dedupes the whole GetSections call) and avoids
	// stale names if the user renamed the performer in Stash.
	name := rec.Name
	if name == "" {
		// Fallback: use the override stored at reconcile time isn't available
		// from the Filter struct. For v1, leave a placeholder — in practice
		// the web UI will resolve names via a separate lookup (see Task 8).
		name = "Performer " + performerID
	}
	return Section{Name: name, Ids: ids}, nil
}
```

**Note on naming:** the placeholder above (`"Performer " + performerID`) is a fallback only used if the web UI hasn't already populated `rec.Name` and the materializer can't reach Stash. In normal operation, Task 7 will pre-resolve performer/tag names by passing the `expectedAutoSection.DefaultName` through to materialization. Step 4 of this task adds that pre-resolution map.

- [ ] **Step 3: Add the per-tag materializer**

Append:

```go
func materializeTagSection(ctx context.Context, client graphql.Client, rec config.Filter, tagID string) (Section, error) {
	perPage := -1
	sort := "date"
	dir := gql.SortDirectionEnumDesc
	depth := -1
	sceneFilter := gql.SceneFilterType{
		Tags: &gql.HierarchicalMultiCriterionInput{
			Modifier: gql.CriterionModifierIncludes,
			Value:    []string{tagID},
			Depth:    &depth,
		},
	}
	findFilter := gql.FindFilterType{
		Sort:      &sort,
		Direction: &dir,
		Per_page:  &perPage,
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, client, &sceneFilter, &findFilter)
	if err != nil {
		return Section{}, fmt.Errorf("FindSceneIdsByFilter (tag %s): %w", tagID, err)
	}
	ids := make([]string, len(resp.FindScenes.Scenes))
	for i, s := range resp.FindScenes.Scenes {
		ids[i] = s.Id
	}

	name := rec.Name
	if name == "" {
		name = "Tag " + tagID
	}
	return Section{Name: name, Ids: ids}, nil
}
```

- [ ] **Step 4: Add the aggregate materializer**

Append:

```go
func materializeAggregateSection(ctx context.Context, client graphql.Client, rec config.Filter, slug string) (Section, error) {
	cfg := config.Application()
	limit := cfg.AggregateLimit
	if limit < 1 {
		limit = 100
	}
	dir := gql.SortDirectionEnumDesc

	var sort string
	var sceneFilter gql.SceneFilterType
	switch slug {
	case aggSlugRecentAdded:
		sort = "created_at"
	case aggSlugRecentPlayed:
		sort = "last_played_at"
		zero := 0
		sceneFilter.Play_count = &gql.IntCriterionInput{
			Modifier: gql.CriterionModifierGreaterThan,
			Value:    zero,
		}
	case aggSlugHighlyRated:
		sort = "rating"
		threshold := cfg.HighlyRatedThreshold
		hundred := 100
		sceneFilter.Rating100 = &gql.IntCriterionInput{
			Modifier: gql.CriterionModifierBetween,
			Value:    threshold,
			Value2:   &hundred,
		}
	case aggSlugUnwatched:
		sort = "created_at"
		zero := 0
		sceneFilter.Play_count = &gql.IntCriterionInput{
			Modifier: gql.CriterionModifierEquals,
			Value:    zero,
		}
	default:
		return Section{}, fmt.Errorf("unknown aggregate slug: %s", slug)
	}

	findFilter := gql.FindFilterType{
		Sort:      &sort,
		Direction: &dir,
		Per_page:  &limit,
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, client, &sceneFilter, &findFilter)
	if err != nil {
		return Section{}, fmt.Errorf("FindSceneIdsByFilter (aggregate %s): %w", slug, err)
	}
	ids := make([]string, len(resp.FindScenes.Scenes))
	for i, s := range resp.FindScenes.Scenes {
		ids[i] = s.Id
	}

	name := resolveName(rec.Name, defaultAggregateName(slug))
	return Section{Name: name, Ids: ids}, nil
}
```

**Note on field names:** the exact Go field names on `gql.IntCriterionInput`, `gql.MultiCriterionInput`, `gql.HierarchicalMultiCriterionInput`, `gql.SceneFilterType` come from genqlient. Stash uses `snake_case` GraphQL fields and genqlient capitalizes the first letter only — so `play_count` → `Play_count`, `rating100` → `Rating100`, `value2` → `Value2`. If a field name above doesn't match the generated type, search for it in `internal/stash/gql/generated.go` and use the actual generated identifier.

- [ ] **Step 5: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0. If genqlient field names differ, fix per the note in Step 4.

- [ ] **Step 6: Commit**

```bash
git add internal/library/autosection_materialize.go
git commit -m "library: materialize auto-section records via FindSceneIdsByFilter"
```

---

### Task 7: Wire reconcile and dispatch into `sections.go`

**Files:**
- Modify: `internal/library/sections.go`

This task introduces a typed-union list (`filterEntry`) so saved-filter and auto-section records can flow through the same pipeline while keeping their behaviors distinct.

- [ ] **Step 1: Add the union type and a helper to walk UserConfig.Filters**

At the top of `internal/library/sections.go`, just under the imports, add:

```go
// filterEntry is the typed result of getFilters: each entry is either a
// saved Stash filter (use SavedFilter) or an auto-section record (use
// AutoID + DefaultName). Disabled and Name come from UserConfig.Filter.
type filterEntry struct {
	SavedFilter *gql.SavedFilterParts
	AutoID      string // empty if SavedFilter is set
	DefaultName string // resolved entity name, used when Name is empty
	Name        string // user override
	Disabled    bool
}
```

- [ ] **Step 2: Replace `getFilters` to return `[]filterEntry` mixing saved + auto**

Replace the body of `getFilters` (currently `func (libraryService *Service) getFilters(ctx context.Context) ([]gql.SavedFilterParts, error)`) with:

```go
func (libraryService *Service) getFilters(ctx context.Context) ([]filterEntry, error) {
	savedFiltersResp, err := gql.FindSavedSceneFilters(ctx, libraryService.StashClient)
	if err != nil {
		return nil, fmt.Errorf("failed to find saved filters: %w", err)
	}

	userCfg := config.User(ctx)

	// Split user config: saved-filter overrides (numeric ids) drive ordering;
	// auto records always come from user config in their stored order.
	var savedOverrides []config.Filter
	autoRecords := make(map[string]config.Filter)
	autoOrder := make([]string, 0)
	for _, f := range userCfg.Filters {
		if isAutoID(f.ID) {
			autoRecords[f.ID] = f
			autoOrder = append(autoOrder, f.ID)
			continue
		}
		savedOverrides = append(savedOverrides, f)
	}

	// Resolve saved-filter ordering using existing logic.
	var savedSlice []gql.SavedFilterParts
	if len(savedOverrides) == 0 {
		savedSlice, err = libraryService.buildFiltersByFrontpage(ctx, savedFiltersResp)
		if err != nil {
			return nil, err
		}
	} else {
		savedSlice = buildFiltersByUserConfig(ctx, savedFiltersResp, savedOverrides)
	}
	out := make([]filterEntry, 0, len(savedSlice)+len(autoOrder))
	for i := range savedSlice {
		// Honor name override / disabled flag from UserConfig if present.
		var override config.Filter
		for _, ov := range savedOverrides {
			if ov.ID == savedSlice[i].Id {
				override = ov
				break
			}
		}
		out = append(out, filterEntry{
			SavedFilter: &savedSlice[i],
			Name:        override.Name,
			Disabled:    override.Disabled,
		})
	}

	// Append auto records in their UserConfig.Filters order. DefaultName
	// resolution happens in the materializer; the entry just carries IDs and
	// user override fields here.
	for _, id := range autoOrder {
		f := autoRecords[id]
		out = append(out, filterEntry{
			AutoID:   f.ID,
			Name:     f.Name,
			Disabled: f.Disabled,
		})
	}

	return out, nil
}
```

- [ ] **Step 3: Update `getSectionsByFilters` to take `[]filterEntry` and dispatch**

Replace the existing `func (libraryService *Service) getSectionsByFilters(ctx context.Context, filters []gql.SavedFilterParts) ([]Section, error)` with:

```go
func (libraryService *Service) getSectionsByFilters(ctx context.Context, entries []filterEntry) ([]Section, error) {
	sections := make([]Section, len(entries))

	wg := sync.WaitGroup{}
	wg.Add(len(entries))

	for i, e := range entries {
		go func(i int, e filterEntry) {
			defer wg.Done()
			if e.Disabled {
				return
			}
			if e.SavedFilter != nil {
				libraryService.buildSavedFilterSection(ctx, i, sections, *e.SavedFilter, e.Name)
				return
			}
			if e.AutoID != "" {
				libraryService.buildAutoSection(ctx, i, sections, config.Filter{ID: e.AutoID, Name: e.Name})
			}
		}(i, e)
	}
	wg.Wait()
	sections = slices.DeleteFunc(sections, func(s Section) bool {
		return len(s.Ids) == 0
	})
	return sections, nil
}

func (libraryService *Service) buildSavedFilterSection(ctx context.Context, idx int, out []Section, f gql.SavedFilterParts, nameOverride string) {
	flog := log.Ctx(ctx).With().Str("filterId", f.Id).Str("name", f.Name).Logger()

	sceneFilter, err := filter.SavedFilterToSceneFilter(ctx, f)
	if err != nil {
		flog.Warn().Err(err).Interface("savedFilter", f).Msg("Failed to convert filter, skipping")
		return
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, libraryService.StashClient, &sceneFilter.SceneFilter, &sceneFilter.FilterOpts)
	if err != nil {
		flog.Err(err).Interface("savedFilter", f).Interface("sceneFilter", sceneFilter).Msg("Failed to find scenes by filter, skipping")
		return
	}
	if len(resp.FindScenes.Scenes) == 0 {
		flog.Debug().Msg("Filter skipped: 0 scenes")
		return
	}

	name := f.Name
	if nameOverride != "" {
		name = nameOverride
	}
	out[idx] = Section{Name: name, Ids: make([]string, len(resp.FindScenes.Scenes))}
	for j, v := range resp.FindScenes.Scenes {
		out[idx].Ids[j] = v.Id
	}
	flog.Debug().Int("scenes", len(out[idx].Ids)).Msg("Section built")
}

func (libraryService *Service) buildAutoSection(ctx context.Context, idx int, out []Section, rec config.Filter) {
	flog := log.Ctx(ctx).With().Str("autoId", rec.ID).Logger()
	sec, err := materializeAutoSection(ctx, libraryService.StashClient, rec)
	if err != nil {
		flog.Warn().Err(err).Msg("auto-section materialize failed, skipping")
		return
	}
	if len(sec.Ids) == 0 {
		flog.Debug().Msg("auto-section skipped: 0 scenes")
		return
	}
	if sec.Name == "" {
		flog.Warn().Msg("auto-section produced empty name; skipping")
		return
	}
	out[idx] = sec
	flog.Debug().Int("scenes", len(sec.Ids)).Msg("auto-section built")
}
```

`buildSavedFilterSection` is the existing logic, factored out of the old loop body so the dispatch in `getSectionsByFilters` stays compact. Behavior is unchanged for saved filters.

- [ ] **Step 4: Insert the reconcile call into `GetSections`**

In `internal/library/sections.go`, locate the `GetSections` body — specifically the `singleflight.Do` callback. Currently it starts with:

```go
filters, err := libraryService.getFilters(ctx)
if err != nil {
    return nil, err
}
```

Replace with:

```go
// Reconcile auto-section records into UserConfig.Filters before reading.
// This adds new entries for entities that crossed the threshold and
// hard-prunes records whose entities no longer qualify.
reconcileAutoSections(ctx, libraryService.StashClient)

filters, err := libraryService.getFilters(ctx)
if err != nil {
    return nil, err
}
```

- [ ] **Step 5: Update the call site in the same callback**

The `GetSections` callback currently branches on `len(filters) == 0` and either calls `getDefaultSections` or `getSectionsByFilters`. Update the second branch to pass the new `[]filterEntry` type:

```go
var sections []Section
if len(filters) == 0 {
    log.Ctx(ctx).Info().Msg("No saved scene filters found, creating default section with ALL scenes")
    sections, err = libraryService.getDefaultSections(ctx)
} else {
    sections, err = libraryService.getSectionsByFilters(ctx, filters)
}
```

The signatures of `getDefaultSections` (returns `[]Section`) and `getSectionsByFilters` (now takes `[]filterEntry`) align with `filters` being `[]filterEntry`. No other changes here.

- [ ] **Step 6: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0. Common errors:
- "filter declared but not used" — the local var `filter` may shadow the package; check imports.
- Field-name mismatches on the genqlient types — see the Task 6 Step 4 note about generated identifiers.

- [ ] **Step 7: Manual smoke test against a running Stash**

Start stash-vr with all auto-section toggles OFF (default), pointing at your local Stash:

```bash
go run ./cmd/stash-vr --STASH_GRAPHQL_URL=http://localhost:9999/graphql
```

Hit the index in another terminal:

```bash
curl -s http://localhost:9666/heresphere | head -40
```

Expected: existing saved-filter sections render unchanged (auto sections all disabled). No errors in the stash-vr log.

Stop stash-vr (Ctrl+C).

Now enable aggregates only and a CONFIG_PATH so we can inspect persistence:

```bash
mkdir -p /tmp/svr-config
go run ./cmd/stash-vr \
  --STASH_GRAPHQL_URL=http://localhost:9999/graphql \
  --AUTO_SECTIONS_AGGREGATES=true \
  --CONFIG_PATH=/tmp/svr-config
```

(On Windows PowerShell substitute `$env:TEMP\svr-config` or any writable directory; pflag also accepts env-var form: `$env:AUTO_SECTIONS_AGGREGATES="true"`.)

Hit `/heresphere`, then inspect the persisted config:

```bash
cat /tmp/svr-config/config.json
```

Expected: `Filters` contains four entries with IDs `auto:agg:recent_added`, `auto:agg:recent_played`, `auto:agg:highly_rated`, `auto:agg:unwatched`.

Hit `/heresphere` again:

```bash
curl -s http://localhost:9666/heresphere | jq '.library | map(.name)'
```

Expected: includes `Recently Added`, `Recently Played`, `Highly Rated`, `Unwatched` (any that materialized to >0 scenes; an empty Unwatched on a fully-watched library is fine and gets dropped).

- [ ] **Step 8: Commit**

```bash
git add internal/library/sections.go
git commit -m "library: wire reconcile + dispatch auto-section records in GetSections"
```

---

### Task 8: Surface auto records in the `/filters` web UI

**Files:**
- Modify: `internal/api/web/web.go`

The existing template iterates `FilterOverrides` rows generated by `filterOverrideRows`. Today that helper only includes rows whose ID matches a saved Stash filter. Auto records have no matching Stash filter, so they're invisible — even though they exist in `UserConfig.Filters`. We need to also emit rows for auto records, with a resolved display name (performer/tag name from Stash, or fixed aggregate label) as the `SourceName`.

- [ ] **Step 1: Extend `filterOverrideRows` to include auto records**

Replace the current `filterOverrideRows` function in `internal/api/web/web.go` with:

```go
func filterOverrideRows(ctx context.Context, stashClient graphql.Client, stashFilters []filterData) []filterOverride {
	cfg := config.User(ctx)

	// Build saved-filter rows (existing behavior).
	rows := make([]filterOverride, 0, len(stashFilters)+len(cfg.Filters))
	seenSaved := map[string]struct{}{}
	for _, cf := range cfg.Filters {
		for _, sf := range stashFilters {
			if sf.Id == cf.ID {
				name := cf.Name
				if name == "" {
					name = sf.Name
				}
				rows = append(rows, filterOverride{ID: sf.Id, SourceName: sf.Name, Name: name, Disabled: cf.Disabled})
				seenSaved[sf.Id] = struct{}{}
				break
			}
		}
	}
	for _, s := range stashFilters {
		if _, ok := seenSaved[s.Id]; ok {
			continue
		}
		rows = append(rows, filterOverride{ID: s.Id, SourceName: s.Name, Name: s.Name, Disabled: false})
	}

	// Append auto-record rows in their UserConfig order.
	autoNames := resolveAutoSectionNames(ctx, stashClient, cfg.Filters)
	for _, cf := range cfg.Filters {
		if !isWebAutoID(cf.ID) {
			continue
		}
		source := autoNames[cf.ID]
		if source == "" {
			source = cf.ID // last-resort fallback
		}
		display := cf.Name
		if display == "" {
			display = source
		}
		rows = append(rows, filterOverride{ID: cf.ID, SourceName: source, Name: display, Disabled: cf.Disabled})
	}

	return rows
}
```

- [ ] **Step 2: Add the auto-name resolver**

Append two helpers to the same file (above or below `filterOverrideRows`):

```go
// isWebAutoID is a duplicate of library.isAutoID kept package-local to avoid
// importing the library package here. The auto: prefix is stable per the spec.
func isWebAutoID(id string) bool {
	const prefix = "auto:"
	return len(id) > len(prefix) && id[:len(prefix)] == prefix
}

// resolveAutoSectionNames resolves display names for auto:* records by
// looking up the corresponding Stash entity. Performer / tag names come
// from FindPerformerByName-style queries adapted to lookup-by-id; aggregate
// slugs map to fixed labels. On any Stash error we fall back to the slug
// or stash id, so the UI still renders.
func resolveAutoSectionNames(ctx context.Context, client graphql.Client, filters []config.Filter) map[string]string {
	out := make(map[string]string, len(filters))

	// Collect needed performer + tag IDs.
	var performerIDs, tagIDs []string
	for _, f := range filters {
		const (
			perfPrefix = "auto:perf:"
			tagPrefix  = "auto:tag:"
			aggPrefix  = "auto:agg:"
		)
		switch {
		case len(f.ID) > len(perfPrefix) && f.ID[:len(perfPrefix)] == perfPrefix:
			performerIDs = append(performerIDs, f.ID[len(perfPrefix):])
		case len(f.ID) > len(tagPrefix) && f.ID[:len(tagPrefix)] == tagPrefix:
			tagIDs = append(tagIDs, f.ID[len(tagPrefix):])
		case len(f.ID) > len(aggPrefix) && f.ID[:len(aggPrefix)] == aggPrefix:
			slug := f.ID[len(aggPrefix):]
			out[f.ID] = aggregateLabel(slug)
		}
	}

	if len(performerIDs) > 0 {
		// One findPerformers call by IDs.
		resp, err := gql.FindPerformersByIDs(ctx, client, performerIDs)
		if err == nil {
			for _, p := range resp.FindPerformers.Performers {
				out["auto:perf:"+p.Id] = p.Name
			}
		}
	}
	if len(tagIDs) > 0 {
		// FindAllTags is already cheap-ish; a per-id batch is a future
		// optimization. For now, find each by id via a single call to
		// FindTagsByIDs (added below).
		resp, err := gql.FindTagsByIDs(ctx, client, tagIDs)
		if err == nil {
			for _, t := range resp.FindTags.Tags {
				out["auto:tag:"+t.Id] = t.Name
			}
		}
	}
	return out
}

func aggregateLabel(slug string) string {
	switch slug {
	case "recent_added":
		return "Recently Added"
	case "recent_played":
		return "Recently Played"
	case "highly_rated":
		return "Highly Rated"
	case "unwatched":
		return "Unwatched"
	}
	return slug
}
```

- [ ] **Step 3: Add the two new GraphQL ops referenced above**

Append to `internal/stash/gql/documents/query.graphql`:

```graphql
query FindPerformersByIDs($ids: [ID!]!){
    findPerformers(ids: $ids){
        performers {
            id, name
        }
    }
}

query FindTagsByIDs($ids: [ID!]!){
    findTags(ids: $ids){
        tags {
            id, name
        }
    }
}
```

**Note:** `findTags` currently does NOT take an `ids` arg in some Stash versions — check `internal/stash/gql/schema/local.graphql` for `findTags(`. If the schema doesn't support `ids:`, fall back to filtering by name via the existing `FindTagByName` op (one call per tag, accept the cost; tag count is bounded by `TOP_N_TAGS`). Adjust the materializer to loop accordingly.

- [ ] **Step 4: Regenerate genqlient and update the call site signature**

Run: `go generate ./cmd/stash-vr`
Expected: success.

If genqlient's signature for `FindPerformersByIDs` differs (e.g., it takes `ids []string` directly vs. as a pointer), adjust the call in `resolveAutoSectionNames` accordingly. Same for `FindTagsByIDs`.

- [ ] **Step 5: Update the `IndexHandler` call site**

Find the line:

```go
data.StashData.FilterOverrides = filterOverrideRows(r.Context(), data.StashData.FilterData)
```

Replace with:

```go
data.StashData.FilterOverrides = filterOverrideRows(r.Context(), libraryService.StashClient, data.StashData.FilterData)
```

- [ ] **Step 6: Verify build**

Run: `go vet ./... && go build ./...`
Expected: exit 0.

- [ ] **Step 7: Manual UI verification**

Restart stash-vr with both aggregates and performers enabled, plus a CONFIG_PATH, e.g.:

```bash
go run ./cmd/stash-vr \
  --STASH_GRAPHQL_URL=http://localhost:9999/graphql \
  --AUTO_SECTIONS_AGGREGATES=true \
  --AUTO_SECTIONS_PERFORMERS=true \
  --MIN_SCENES_PER_PERFORMER=2 \
  --CONFIG_PATH=/tmp/svr-config
```

(MIN_SCENES_PER_PERFORMER lowered to 2 so test data with few scenes still produces at least one performer auto-section.)

Open `http://localhost:9666/` in a browser. Under "Configure filter overrides", expand the section.

Expected: rows for saved Stash filters PLUS rows for each `auto:agg:*` and any `auto:perf:*` records. Aggregate rows show "Recently Added", "Recently Played", etc. as the placeholder. Performer rows show the actual performer name resolved from Stash. Each row has a drag handle, a Disable checkbox, the auto:* ID, and a rename input.

- [ ] **Step 8: Commit**

```bash
git add internal/api/web/web.go internal/stash/gql/documents/query.graphql internal/stash/gql/generated.go
git commit -m "web: render auto-section records in /filters UI with resolved names"
```

---

### Task 9: End-to-end manual verification + regression check

This task has no code changes — it is a structured pass to confirm spec coverage and to look for regressions in unrelated paths.

**Files:** none.

- [ ] **Step 1: Cold start with no auto-sections enabled**

```bash
rm -rf /tmp/svr-config
go run ./cmd/stash-vr \
  --STASH_GRAPHQL_URL=http://localhost:9999/graphql \
  --CONFIG_PATH=/tmp/svr-config
```

Hit `/heresphere`, `/deovr`, `/`, and `/cover/<some-id>`.

Expected:
- Saved-filter sections render exactly as before (compare against pre-change behavior if possible).
- No `auto:*` records in `/tmp/svr-config/config.json` (file may not exist).
- No errors logged.

Stop the server.

- [ ] **Step 2: Enable each auto-section type in turn, verify materialization**

For each of the toggles below, restart with the toggle on (and aggregates' sub-toggles default to true):

```bash
# performers only
go run ./cmd/stash-vr ... --AUTO_SECTIONS_PERFORMERS=true --MIN_SCENES_PER_PERFORMER=2 --CONFIG_PATH=/tmp/svr-config
# tags only
go run ./cmd/stash-vr ... --AUTO_SECTIONS_TAGS=true --TOP_N_TAGS=5 --CONFIG_PATH=/tmp/svr-config
# aggregates only
go run ./cmd/stash-vr ... --AUTO_SECTIONS_AGGREGATES=true --CONFIG_PATH=/tmp/svr-config
# all three together
go run ./cmd/stash-vr ... \
  --AUTO_SECTIONS_PERFORMERS=true --MIN_SCENES_PER_PERFORMER=2 \
  --AUTO_SECTIONS_TAGS=true --TOP_N_TAGS=5 \
  --AUTO_SECTIONS_AGGREGATES=true \
  --CONFIG_PATH=/tmp/svr-config
```

After each, `curl http://localhost:9666/heresphere | jq '.library | map(.name)'` and confirm sections of the expected types appear with sensible names.

- [ ] **Step 3: Rename one auto-section via the web UI; verify persistence**

With everything enabled, open `http://localhost:9666/` in a browser, find an `auto:perf:*` row, type a new name in the rename input, click Save. Verify:

- `cat /tmp/svr-config/config.json` shows the renamed entry's `Name` field updated.
- `curl http://localhost:9666/heresphere | jq '.library | map(.name)'` shows the new name in place of the original.
- Restart stash-vr — the rename survives.

- [ ] **Step 4: Disable one auto-section via the web UI; verify it doesn't render**

Same procedure — toggle the Disable checkbox on one auto record, click Save. Verify:

- `config.json` has `"Disabled": true` on that entry.
- The corresponding section is absent from `/heresphere`.

- [ ] **Step 5: Verify hard-prune by flipping a master toggle off**

Stop and restart with `--AUTO_SECTIONS_PERFORMERS=false` (others as before). Verify after one `/heresphere` hit:

- All `auto:perf:*` entries are gone from `config.json`.
- Renamed/disabled customizations on those entries are lost (this is the intended hard-prune behavior).
- `auto:tag:*` and `auto:agg:*` entries are unaffected.

- [ ] **Step 6: Spot-check no-regression on un-modified paths**

With everything enabled, hit:

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9666/heresphere
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9666/deovr
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9666/
```

Expected: `200` for all three. Open the `/heresphere` URL inside an actual HereSphere session if possible and confirm sections browse normally.

- [ ] **Step 7: Document any deviations**

If any step found a behavior that differs from the spec, append a short note to the PR description (or a follow-up issue) describing the gap. Do NOT attempt to fix it as part of this plan — file a separate cycle for any bugs found.

---

## Self-Review

**Spec coverage** (each spec section → task that implements it):

| Spec section                          | Task(s)              |
| ------------------------------------- | -------------------- |
| Goal: per-performer / per-tag / aggregates | 4, 5, 6        |
| Data model: `auto:*` ID prefix scheme | 3                    |
| Reconciliation: expected set + hard-prune | 4, 5             |
| Materialization: per-record → Section | 6                    |
| Default ordering (kind groups)        | 4 (kind-group order in `computeExpectedAutoSections`) + 5 (append-at-end via reconcile) |
| Frontpage caveat (saved-filter ordering preserved) | 7 (split UserConfig.Filters in `getFilters`) |
| Configuration surface (12 env vars)   | 1                    |
| Naming conventions (no prefix, fixed labels for aggregates) | 6, 8 |
| Edge: Stash unreachable mid-reconcile | 4 (warn-and-skip in `computeExpectedAutoSections`) |
| Edge: no `CONFIG_PATH`                | inherited from existing `config.User` / `config.Save` behavior — not specifically tested here |
| Edge: performer renamed in Stash      | implicit — `materializePerformerSection` resolves name fresh |
| Edge: performer deleted               | 5 (hard-prune on next reconcile) |
| File layout                           | task list mirrors spec's File layout section |
| Web UI render                         | 8                    |

**Placeholder scan:** none — every step contains real code or real commands.

**Type consistency:** `filterEntry` introduced in Task 7 is consumed by `getSectionsByFilters` in the same task; `materializeAutoSection` from Task 6 is called by `buildAutoSection` in Task 7; `parseAutoID` from Task 3 is used by `materializeAutoSection` in Task 6 and `reconcileAutoSections` in Task 5; `expectedAutoSection` from Task 4 is used by `reconcileAutoSections` in Task 5. Names and shapes line up.

**Known soft spots:**
- Genqlient enum and field-name uncertainty (Task 2 Step 1, Task 6 Step 4 notes): the implementer must verify against the regenerated `generated.go`. Fixes are local — tightly scoped.
- Sort string `"scenes_count"` vs `"scene_count"` (Task 2 Step 2 note): runtime-verifiable; both spellings have been valid in different Stash versions.
- The `FindTagsByIDs` op (Task 8 Step 3) is conditional on Stash supporting `ids:` on `findTags`. Fallback path is documented inline.

These soft spots are flagged in their respective steps so the implementer doesn't rediscover them mid-task.
