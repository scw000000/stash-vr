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
	"stash-vr/internal/stash"
	"stash-vr/internal/stash/gql"
	"stash-vr/internal/util"
)

const pageSize = 30

// fetchSceneIDs runs FindSceneIdsByFilter and returns (ids, totalCount).
// sceneFilter may be nil (all scenes); page is 1-indexed.
func fetchSceneIDs(ctx context.Context, client graphql.Client, sceneFilter *gql.SceneFilterType, page int) ([]string, int, error) {
	resp, err := gql.FindSceneIdsByFilter(ctx, client, sceneFilter, &gql.FindFilterType{
		Page:      util.Ptr(page),
		Per_page:  util.Ptr(pageSize),
		Sort:      util.Ptr("created_at"),
		Direction: util.Ptr(gql.SortDirectionEnumDesc),
	})
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
	cards := make([]Card, 0, len(ids))
	for _, id := range ids {
		vd, err := lib.GetScene(ctx, id, false)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Str("id", id).Msg("browse: skip scene (fetch error)")
			continue
		}
		if vd == nil || vd.SceneParts == nil || len(vd.SceneParts.Files) == 0 {
			continue
		}
		c := Card{
			ID:           id,
			Title:        vd.Title(),
			Duration:     formatDuration(vd.SceneParts.Files[0].Duration),
			DetailURL:    "/browse/scene/" + url.PathEscape(id),
			DeoVRPlayURL: "/deovr/videoData/" + url.PathEscape(id),
		}
		// Thumbnail: heatmap composite for interactive scenes; screenshot otherwise.
		if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
			if vd.SceneParts.Interactive && vd.SceneParts.Paths.Interactive_heatmap != nil {
				c.ThumbnailURL = heatmap.GetCoverUrl(baseURL, id)
			} else {
				c.ThumbnailURL = stash.ApiKeyed(*vd.SceneParts.Paths.Screenshot)
			}
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
