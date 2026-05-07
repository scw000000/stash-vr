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

	return Section{Name: resolveName(rec.Name, "Performer "+performerID), Ids: ids}, nil
}

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

	return Section{Name: resolveName(rec.Name, "Tag "+tagID), Ids: ids}, nil
}

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
		// BETWEEN [threshold, 100] is used because the genqlient enum has no
		// GREATER_THAN_OR_EQUAL; both ends are inclusive, matching the spec.
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
