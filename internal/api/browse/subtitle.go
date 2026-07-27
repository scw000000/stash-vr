package browse

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/library"
	"stash-vr/internal/subtitles"
)

func sceneVideoPaths(vd *library.VideoData) []string {
	if vd == nil || vd.SceneParts == nil {
		return nil
	}
	paths := make([]string, 0, len(vd.SceneParts.Files))
	for _, file := range vd.SceneParts.Files {
		if file == nil || strings.TrimSpace(file.Path) == "" {
			continue
		}
		paths = append(paths, file.Path)
	}
	return paths
}

func (h *httpHandler) subtitleScene(w http.ResponseWriter, r *http.Request) (string, []string, bool) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return "", nil, false
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: subtitle scene not found")
		http.NotFound(w, r)
		return "", nil, false
	}
	paths := sceneVideoPaths(vd)
	if len(paths) == 0 {
		http.Error(w, "scene has no local video file", http.StatusConflict)
		return "", nil, false
	}
	return id, paths, true
}

func (h *httpHandler) sceneSubtitlesHandler(w http.ResponseWriter, r *http.Request) {
	id, paths, ok := h.subtitleScene(w, r)
	if !ok {
		return
	}
	writeSubtitleState(w, http.StatusOK, h.subtitleService.State(id, paths))
}

func (h *httpHandler) sceneSubtitleGenerateHandler(w http.ResponseWriter, r *http.Request) {
	id, paths, ok := h.subtitleScene(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeSubtitleError(w, http.StatusBadRequest, h.subtitleService.State(id, paths), "invalid form")
		return
	}
	options := subtitles.Options{
		SourceLanguage:       r.FormValue("sourceLanguage"),
		Mode:                 r.FormValue("mode"),
		TranscriptionService: r.FormValue("transcriptionService"),
		TranscriptionModel:   r.FormValue("transcriptionModel"),
		TranslationService:   r.FormValue("translationService"),
		TranslationModel:     r.FormValue("translationModel"),
	}
	if _, err := h.subtitleService.Start(r.Context(), id, paths[0], options); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already active") {
			status = http.StatusConflict
		}
		writeSubtitleError(w, status, h.subtitleService.State(id, paths), err.Error())
		return
	}
	writeSubtitleState(w, http.StatusAccepted, h.subtitleService.State(id, paths))
}

func (h *httpHandler) sceneSubtitleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, paths, ok := h.subtitleScene(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeSubtitleError(w, http.StatusBadRequest, h.subtitleService.State(id, paths), "invalid form")
		return
	}
	key := strings.TrimSpace(r.FormValue("key"))
	if key == "" {
		writeSubtitleError(w, http.StatusBadRequest, h.subtitleService.State(id, paths), "subtitle key is required")
		return
	}
	if err := h.subtitleService.DeleteFile(id, paths, key); err != nil {
		writeSubtitleError(w, http.StatusBadRequest, h.subtitleService.State(id, paths), err.Error())
		return
	}
	writeSubtitleState(w, http.StatusOK, h.subtitleService.State(id, paths))
}

func appendSidecarCaptions(captions []CaptionRef, files []subtitles.File) []CaptionRef {
	for _, file := range files {
		captions = append(captions, CaptionRef{
			Label:       file.Name,
			CaptionType: "srt",
			FileKey:     file.Key,
			Generated:   true,
		})
	}
	return captions
}

func writeSubtitleError(w http.ResponseWriter, status int, state subtitles.State, message string) {
	state.Err = message
	writeSubtitleState(w, status, state)
}

func writeSubtitleState(w http.ResponseWriter, status int, state subtitles.State) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(state); err != nil {
		log.Error().Err(err).Msg("browse: encode subtitle state")
	}
}
