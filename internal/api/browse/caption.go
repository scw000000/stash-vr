package browse

import (
	"io"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
	"stash-vr/internal/subtitles"
)

// sceneCaptionHandler proxies Stash's caption file (typically VTT) for a
// given scene and language/type combo. Same pattern as the /browse/scene/{id}/stream
// proxy: same-origin so the browser can fetch without CORS, and the
// Stash API key is appended server-side.
func (h *httpHandler) sceneCaptionHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q := r.URL.Query()
	if fileKey := q.Get("file"); fileKey != "" {
		_, paths, ok := h.subtitleScene(w, r)
		if !ok {
			return
		}
		file, err := subtitles.OpenFile(paths, fileKey)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			http.Error(w, "subtitle unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-subrip; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		return
	}

	lang := q.Get("lang")
	captionType := q.Get("type")
	if lang == "" || captionType == "" {
		http.Error(w, "lang and type required", http.StatusBadRequest)
		return
	}

	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil || vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Caption == nil {
		http.NotFound(w, r)
		return
	}

	upstream := *vd.SceneParts.Paths.Caption + "?lang=" + url.QueryEscape(lang) + "&type=" + url.QueryEscape(captionType)
	upstream = stash.ApiKeyed(upstream)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch caption upstream")
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
		upstreamCT = "text/vtt; charset=utf-8"
	}
	w.Header().Set("Content-Type", upstreamCT)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: copy caption body")
	}
}
