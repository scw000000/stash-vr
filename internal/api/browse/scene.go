package browse

import (
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/api/heatmap"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/config"
	"stash-vr/internal/prefix"
	"stash-vr/internal/static"
)

// vrFilenameRe matches common VR markers in a basename or path. Hand-rolled
// boundaries because Go's \b treats _ as a word char (so "_VR_" wouldn't
// satisfy \bVR\b). MKX matches MKX, MKX200, MKX220, etc.
var vrFilenameRe = regexp.MustCompile(`(?i)(^|[^a-z0-9])(180|360|MKX[0-9]*|FB360|FISHEYE|DOME|SBS|EAC|RF52)([^a-z0-9]|$)`)

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
		ID:         id,
		Title:      vd.Title(),
		BackURL:    backURL(r),
		ErrMessage: r.URL.Query().Get("err"),
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
	is180SBS := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		// Detect VR projection BEFORE the ancestor skip so an ancestor-injected
		// VR / DOME / SBS tag still counts. Match any tag whose name contains
		// "VR" (case-insensitive) — Stash's VR scrapers add tags like "VR",
		// "vr_180", etc. — and the explicit DOME / SBS projection tags.
		if !is180SBS {
			upper := strings.ToUpper(name)
			if strings.Contains(upper, "VR") || upper == apiinternal.TagVR_DOME || upper == apiinternal.TagVR_SBS {
				is180SBS = true
			}
		}
		// Skip ancestor-injected tags from the chip list.
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			data.IsFavorite = true
			continue
		}
		data.Tags = append(data.Tags, name)
	}
	// Filename heuristic backstop for libraries that don't tag at all.
	if !is180SBS {
		for _, f := range vd.SceneParts.Files {
			if f == nil {
				continue
			}
			if vrFilenameRe.MatchString(f.Basename) || vrFilenameRe.MatchString(f.Path) {
				is180SBS = true
				break
			}
		}
	}
	// Mode dispatch: VR scenes get the immersive 180° SBS sphere; everything
	// else gets a flat plane in 3D space (a virtual cinema). The Enter VR
	// button always shows when there's a stream — user can watch any video
	// in headset.
	if is180SBS {
		data.VRMode = "180sbs"
	} else {
		data.VRMode = "flat"
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
