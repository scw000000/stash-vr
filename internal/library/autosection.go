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
