package browse

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash"
)

// sceneStreamHandler proxies the Stash direct stream so the browser fetches
// from the same origin as the /browse page. Without this, A-Frame's WebGL
// texture upload taints on cross-origin video and the headset's browser may
// fail to reach the Stash host directly. Forwards the Range header to keep
// byte-range scrubbing working.
var streamCopiedHeaders = []string{
	"Content-Length",
	"Content-Range",
	"Accept-Ranges",
	"Last-Modified",
	"ETag",
	"Cache-Control",
}

// pickContentType prefers the file-extension-derived MIME type because Stash
// frequently mis-labels MP4 files as video/mpeg, which most browsers refuse
// to play.
func pickContentType(basename, upstream string) string {
	if ext := strings.ToLower(filepath.Ext(basename)); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" && strings.HasPrefix(mt, "video/") {
			return mt
		}
	}
	if upstream != "" && upstream != "video/mpeg" && upstream != "application/octet-stream" {
		return upstream
	}
	return "video/mp4"
}

func (h *httpHandler) sceneStreamHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil ||
		vd.SceneParts.Paths == nil || vd.SceneParts.Paths.Stream == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("stream: scene or stream URL missing")
		http.NotFound(w, r)
		return
	}

	upstreamURL := stash.ApiKeyed(*vd.SceneParts.Paths.Stream)

	method := r.Method
	if method != http.MethodGet && method != http.MethodHead {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(r.Context(), method, upstreamURL, nil)
	if err != nil {
		http.Error(w, "stream: build upstream request", http.StatusInternalServerError)
		return
	}
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("stream: upstream request failed")
		http.Error(w, "stream: upstream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for _, key := range streamCopiedHeaders {
		if v := resp.Header.Get(key); v != "" {
			w.Header().Set(key, v)
		}
	}
	basename := ""
	if len(vd.SceneParts.Files) > 0 && vd.SceneParts.Files[0] != nil {
		basename = vd.SceneParts.Files[0].Basename
	}
	w.Header().Set("Content-Type", pickContentType(basename, resp.Header.Get("Content-Type")))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)

	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Ctx(r.Context()).Debug().Err(err).Str("id", id).Msg("stream: copy interrupted")
	}
}
