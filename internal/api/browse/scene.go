package browse

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/stash"
	"stash-vr/internal/static"
)

var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"le":  func(a, b int) bool { return a <= b },
}).ParseFS(static.Fs, "browse_scene.gohtml"))

func (h *httpHandler) sceneDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: scene not found")
		http.NotFound(w, r)
		return
	}

	baseURL := apiinternal.GetBaseUrl(r)

	data := SceneDetailData{
		ID:           id,
		Title:        vd.Title(),
		BackURL:      backURL(r),
		DeoVRPlayURL: "/deovr/videoData/" + url.PathEscape(id),
		ErrMessage:   r.URL.Query().Get("err"),
	}

	if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
		if vd.SceneParts.Interactive && vd.SceneParts.Paths.Interactive_heatmap != nil {
			data.ThumbnailURL = heatmap.GetCoverUrl(baseURL, id)
		} else {
			data.ThumbnailURL = stash.ApiKeyed(*vd.SceneParts.Paths.Screenshot)
		}
	}

	performerNames := make([]string, 0, len(vd.SceneParts.Performers))
	for _, p := range vd.SceneParts.Performers {
		if p == nil {
			continue
		}
		performerNames = append(performerNames, p.Name)
	}
	data.Performers = strings.Join(performerNames, ", ")

	if vd.SceneParts.Studio != nil {
		data.Studio = vd.SceneParts.Studio.Name
	}
	if vd.SceneParts.Date != nil {
		data.Date = *vd.SceneParts.Date
	}
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		data.Duration = formatDuration(vd.SceneParts.Files[0].Duration)
	}

	if vd.SceneParts.Rating100 != nil {
		data.Rating1to5 = *vd.SceneParts.Rating100 / 20
	}

	favTag := config.Application().FavoriteTag
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		// Skip ancestor-injected tags (decorateTags adds these with prefix.SvrAncestor in Sort_name).
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && t.TagParts.Name == favTag {
			data.IsFavorite = true
			continue
		}
		data.Tags = append(data.Tags, t.TagParts.Name)
	}

	if vd.SceneParts.O_counter != nil {
		data.OCounter = *vd.SceneParts.O_counter
	}
	data.Organized = vd.SceneParts.Organized

	// Datalist of all tag names for the add-tag input.
	tags, err := fetchTags(r.Context(), h.libraryService.StashClient)
	if err == nil {
		data.AllTagNames = make([]string, 0, len(tags))
		for _, t := range tags {
			data.AllTagNames = append(data.AllTagNames, t.Name)
		}
	}

	if err := sceneTmpl.Execute(w, data); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: render scene detail")
	}
}

// backURL returns the Referer if present, else /browse.
func backURL(r *http.Request) string {
	if ref := r.Header.Get("Referer"); ref != "" {
		return ref
	}
	return "/browse"
}
