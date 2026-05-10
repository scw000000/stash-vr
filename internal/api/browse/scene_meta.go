package browse

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// SceneMetaResponse is the JSON returned by GET /browse/scene/{id}/meta.
// Used by the M4c seamless scene swap to refresh playback-panel state
// (title, duration, caption tracks, scene markers) for the newly-loaded
// scene without exiting the WebXR session.
type SceneMetaResponse struct {
	Title        string        `json:"title"`
	DurationSec  float64       `json:"durationSec"`
	Captions     []CaptionRef  `json:"captions"`
	SceneMarkers []SceneMarker `json:"sceneMarkers"`
}

func (h *httpHandler) sceneMetaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: meta scene not found")
		http.NotFound(w, r)
		return
	}
	sp := vd.SceneParts
	out := SceneMetaResponse{Title: vd.Title()}
	if len(sp.Files) > 0 && sp.Files[0] != nil {
		out.DurationSec = sp.Files[0].Duration
	}
	out.Captions = make([]CaptionRef, 0, len(sp.Captions))
	for _, c := range sp.Captions {
		if c == nil {
			continue
		}
		out.Captions = append(out.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
		})
	}
	out.SceneMarkers = make([]SceneMarker, 0, len(sp.Scene_markers))
	for _, m := range sp.Scene_markers {
		if m == nil {
			continue
		}
		out.SceneMarkers = append(out.SceneMarkers, SceneMarker{
			Seconds: m.Seconds,
			Title:   m.Title,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode scene meta")
	}
}
