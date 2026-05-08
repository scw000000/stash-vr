package browse

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	"stash-vr/internal/library"
	"stash-vr/internal/stash/gql"
	"stash-vr/internal/util"
)

const pageSize = 30

// fetchSceneIDs runs FindSceneIdsByFilter and returns (ids, totalCount).
// sceneFilter may be nil (all scenes); page is 1-indexed.
// q is an optional full-text search string (passed to FindFilterType.Q).
func fetchSceneIDs(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, q string, page int) ([]string, int, error) {
	findFilter := &gql.FindFilterType{
		Page:      util.Ptr(page),
		Per_page:  util.Ptr(pageSize),
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

// buildCards converts scene IDs into Card structs, fetching scene metadata
// through the library service so the existing cache and decorateTags apply.
// IDs that fail to fetch are skipped with a warning log entry.
func buildCards(ctx context.Context, lib *library.Service, baseURL string, ids []string) ([]Card, error) {
	vds, err := lib.GetScenesByIds(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("GetScenesByIds: %w", err)
	}
	cards := make([]Card, 0, len(vds))
	for i, vd := range vds {
		if vd == nil {
			log.Ctx(ctx).Warn().Str("id", ids[i]).Msg("browse: skip scene (not found)")
			continue
		}
		if vd.SceneParts == nil || len(vd.SceneParts.Files) == 0 || vd.SceneParts.Files[0] == nil {
			continue
		}
		c := Card{
			ID:           ids[i],
			Title:        vd.Title(),
			Duration:     formatDuration(vd.SceneParts.Files[0].Duration),
			DetailURL:    "/browse/scene/" + url.PathEscape(ids[i]),
			DeoVRPlayURL: "/deovr/" + url.PathEscape(ids[i]),
		}
		if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
			c.ThumbnailURL = heatmap.GetCoverUrl(baseURL, ids[i])
		}
		// Performers comma-joined.
		names := make([]string, 0, len(vd.SceneParts.Performers))
		for _, p := range vd.SceneParts.Performers {
			if p == nil {
				continue
			}
			names = append(names, p.Name)
		}
		c.Performers = strings.Join(names, ", ")
		if vd.SceneParts.Studio != nil {
			c.Studio = vd.SceneParts.Studio.Name
		}
		cards = append(cards, c)
	}
	return cards, nil
}

// formatDuration returns "M:SS" or "H:MM:SS" depending on length.
func formatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// pagerURLs returns prev/next URL strings preserving non-page query params.
// Returns empty strings on edges.
func pagerURLs(basePath string, page, pageMax int, extraParams url.Values) (prev, next string) {
	mk := func(p int) string {
		v := url.Values{}
		for k, vs := range extraParams {
			for _, x := range vs {
				v.Add(k, x)
			}
		}
		v.Set("page", fmt.Sprintf("%d", p))
		return basePath + "?" + v.Encode()
	}
	if page > 1 {
		prev = mk(page - 1)
	}
	if page < pageMax {
		next = mk(page + 1)
	}
	return
}
