package browse

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
)

// sceneSpriteHandler proxies Stash's sprite-sheet (typically a 3x3 JPEG grid
// of 9 frames at Paths.Sprite) for a given scene. Same pattern as the
// preview proxy: same-origin so the browser can fetch without CORS, and the
// Stash API key is appended server-side. Used by the in-VR browse panel's
// tile-hover preview as a fallback when the WebM preview clip is missing.
func (h *httpHandler) sceneSpriteHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil || vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Sprite == nil || *vd.SceneParts.Paths.Sprite == "" {
		http.NotFound(w, r)
		return
	}

	upstream := stash.ApiKeyed(*vd.SceneParts.Paths.Sprite)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch sprite upstream")
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	upstreamCT := resp.Header.Get("Content-Type")
	if upstreamCT == "" {
		upstreamCT = "image/jpeg"
	}
	w.Header().Set("Content-Type", upstreamCT)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: copy sprite body")
	}
}
