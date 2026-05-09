package browse

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
)

// scenePreviewHandler proxies Stash's preview clip (typically WebM) for a
// given scene. Same pattern as the caption proxy: same-origin so the browser
// can fetch without CORS, and the Stash API key is appended server-side.
// Used by the in-VR browse panel's tile-hover preview.
func (h *httpHandler) scenePreviewHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil || vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Preview == nil {
		http.NotFound(w, r)
		return
	}

	upstream := stash.ApiKeyed(*vd.SceneParts.Paths.Preview)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch preview upstream")
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
		upstreamCT = "video/webm"
	}
	w.Header().Set("Content-Type", upstreamCT)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: copy preview body")
	}
}
