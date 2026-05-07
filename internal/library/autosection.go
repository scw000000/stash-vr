package library

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/config"
	"stash-vr/internal/stash/gql"
)

type autoKind int

const (
	autoKindUnknown autoKind = iota
	autoKindPerformer
	autoKindTag
	autoKindAggregate
)

const (
	autoIDPrefix        = "auto:"
	autoPerfPrefix      = "auto:perf:"
	autoTagPrefix       = "auto:tag:"
	autoAggPrefix       = "auto:agg:"
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
	// GREATER_THAN_OR_EQUAL was not in the genqlient schema enum, so the
	// .graphql query uses GREATER_THAN. Pass minScenes-1 to get >= semantics.
	threshold := minScenes - 1
	if threshold < 0 {
		threshold = 0
	}
	resp, err := gql.FindPerformersWithSceneCount(ctx, client, threshold, maxCount)
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

// reconcileAutoSections merges the current expected auto-section set into
// UserConfig.Filters with hard-prune semantics:
//   - Each expected ID already in UserConfig.Filters is preserved verbatim
//     (so user rename / disabled / order is kept).
//   - Each expected ID NOT in UserConfig.Filters is appended at the end of
//     its kind group (which on a fresh install is just the end of the list,
//     since no auto records exist yet).
//   - Each existing auto:* record whose ID is NOT in the expected set is
//     hard-pruned.
//
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
