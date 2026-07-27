package browse

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/static"
	"stash-vr/internal/subtitles"
)

var sceneTmpl = template.Must(template.New("browse_scene.gohtml").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"le":  func(a, b int) bool { return a <= b },
	"marshal": func(v interface{}) (template.JS, error) {
		b, err := json.Marshal(v)
		return template.JS(b), err
	},
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
		ID:      id,
		Title:   vd.Title(),
		BackURL: backURL(r),
	}

	if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Screenshot != nil {
		data.ThumbnailURL = heatmap.GetCoverUrl(baseURL, id)
	}

	if vd.SceneParts.Paths != nil && vd.SceneParts.Paths.Stream != nil {
		// Proxy the stream through stash-vr so the browser fetches it from the
		// same origin as the page. Same-origin avoids CORS issues for WebGL
		// texture upload and works around Stash hosts that aren't reachable
		// from every device's browser context.
		data.DirectStreamURL = "/browse/scene/" + url.PathEscape(id) + "/stream"
	}

	for _, p := range vd.SceneParts.Performers {
		if p == nil {
			continue
		}
		data.Performers = append(data.Performers, EntityRef{ID: p.Id, Name: p.Name})
	}

	if vd.SceneParts.Studio != nil {
		data.Studio = &EntityRef{ID: vd.SceneParts.Studio.Id, Name: vd.SceneParts.Studio.Name}
	}
	if vd.SceneParts.Date != nil {
		data.Date = *vd.SceneParts.Date
	}
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		data.Duration = formatDuration(vd.SceneParts.Files[0].Duration)
	}

	// Captions: language tracks for the subtitle picker.
	for _, c := range vd.SceneParts.Captions {
		if c == nil {
			continue
		}
		data.Captions = append(data.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
			Label:        c.Language_code + " · " + c.Caption_type,
		})
	}
	if h.subtitleService != nil {
		data.Captions = appendSidecarCaptions(
			data.Captions,
			subtitles.RelatedFiles(sceneVideoPaths(vd)),
		)
	}

	// Scene markers: chapter dots on the scrub bar.
	for _, m := range vd.SceneParts.Scene_markers {
		if m == nil {
			continue
		}
		data.SceneMarkers = append(data.SceneMarkers, SceneMarker{
			Seconds: m.Seconds,
			Title:   m.Title,
		})
	}

	// Duration in seconds: needed for marker positioning + scrub-bar math.
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		data.DurationSec = vd.SceneParts.Files[0].Duration
	}

	if vd.SceneParts.Rating100 != nil {
		data.Rating1to5 = *vd.SceneParts.Rating100 / 20
	}

	data.Tags, data.IsFavorite = projectTags(vd.SceneParts)
	// Projection detection needs every tag (including ancestor-injected),
	// so build that input list separately from the chip list above.
	tagInputs := make([]apiinternal.TagInput, 0, len(vd.SceneParts.Tags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		tagInputs = append(tagInputs, apiinternal.TagInput{Name: t.TagParts.Name, Aliases: t.Aliases})
	}
	basename := ""
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		basename = vd.SceneParts.Files[0].Basename
	}
	data.Projection = apiinternal.Detect(tagInputs, basename)

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

	// Disable browser caching of the scene-player page. The HTML is
	// re-rendered per request anyway (and embeds JS that depends on the
	// current binary), so caching it just means stale code after a
	// stash-vr upgrade until the user manually clears the cache.
	w.Header().Set("Cache-Control", "no-store")
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
