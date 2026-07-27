package browse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"stash-vr/internal/config"
	"stash-vr/internal/library"
	"stash-vr/internal/prefix"
	"stash-vr/internal/stash/gql"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// writeState writes a 200 with the post-mutation SceneState as JSON.
// Caller has already done refreshSceneCache(r, id) so the read sees
// the just-written state.
func (h *httpHandler) writeState(w http.ResponseWriter, r *http.Request, id string) {
	state, err := buildSceneState(r.Context(), h.libraryService, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build state failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode SceneState")
	}
}

// writeErr writes an error envelope at the given status.
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(SceneState{Err: msg})
}

// projectTags partitions a scene's tags into user-facing chip refs.
// Ancestor-injected tags (sort_name prefixed by SvrAncestor) are
// excluded; the configured FAVORITE_TAG is consumed into isFavorite.
// Callers needing every tag (e.g. for projection detection) should
// iterate parts.Tags directly.
func projectTags(parts *gql.SceneParts) (chips []EntityRef, isFavorite bool) {
	favTag := config.Application().FavoriteTag
	for _, t := range parts.Tags {
		if t == nil {
			continue
		}
		name := t.TagParts.Name
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if favTag != "" && name == favTag {
			isFavorite = true
			continue
		}
		chips = append(chips, EntityRef{ID: t.TagParts.Id, Name: name})
	}
	return
}

// buildSceneState reads a fresh scene from the cache and projects it to
// SceneState — applying the same FAVORITE_TAG and ancestor-tag filters
// that scene.go's GET path applies. Centralized here so the GET render
// and the POST response can never drift.
func buildSceneState(ctx context.Context, svc *library.Service, id string) (SceneState, error) {
	vd, err := svc.GetScene(ctx, id, false)
	if err != nil {
		return SceneState{}, err
	}
	if vd == nil || vd.SceneParts == nil {
		return SceneState{}, fmt.Errorf("scene %s not found", id)
	}
	state := SceneState{}
	if vd.SceneParts.Rating100 != nil {
		state.Rating1to5 = *vd.SceneParts.Rating100 / 20
	}
	if vd.SceneParts.O_counter != nil {
		state.OCounter = *vd.SceneParts.O_counter
	}
	if vd.SceneParts.Play_count != nil {
		state.PlayCount = *vd.SceneParts.Play_count
	}
	state.Organized = vd.SceneParts.Organized
	state.Tags, state.IsFavorite = projectTags(vd.SceneParts)
	return state, nil
}

func (h *httpHandler) sceneRatingHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	val, parseErr := strconv.Atoi(r.PostForm.Get("value"))
	if parseErr != nil || val < 0 || val > 5 {
		writeErr(w, http.StatusBadRequest, "bad rating")
		return
	}
	if val > 0 {
		vd, err := h.libraryService.GetScene(r.Context(), id, true)
		currentVal := 0
		if err == nil && vd != nil && vd.SceneParts != nil && vd.SceneParts.Rating100 != nil {
			currentVal = *vd.SceneParts.Rating100 / 20
		}
		if val == currentVal {
			val = 0
		}
	}
	var rating5 *float32
	if val > 0 {
		f := float32(val)
		rating5 = &f
	}
	if err := h.libraryService.UpdateRating(r.Context(), id, rating5); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: update rating")
		writeErr(w, http.StatusInternalServerError, "rating update failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	favTag := config.Application().FavoriteTag
	if favTag == "" {
		writeErr(w, http.StatusBadRequest, "FAVORITE_TAG not configured")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	currentlyFav := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if t.TagParts.Name == favTag {
			currentlyFav = true
			break
		}
	}
	if err := h.libraryService.UpdateFavorite(r.Context(), id, !currentlyFav); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: toggle favorite")
		writeErr(w, http.StatusInternalServerError, "favorite toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneTagAddHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		writeErr(w, http.StatusBadRequest, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	current := make([]string, 0, len(vd.SceneParts.Tags)+1)
	exists := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		current = append(current, t.TagParts.Name)
		if strings.EqualFold(t.TagParts.Name, tagName) {
			exists = true
		}
	}
	if !exists {
		current = append(current, tagName)
	}
	if err := h.libraryService.UpdateTags(r.Context(), id, current); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("tag", tagName).Msg("browse: add tag")
		writeErr(w, http.StatusInternalServerError, "tag add failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneTagRemoveHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		writeErr(w, http.StatusBadRequest, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	remaining := make([]string, 0, len(vd.SceneParts.Tags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		if strings.EqualFold(t.TagParts.Name, tagName) {
			continue
		}
		remaining = append(remaining, t.TagParts.Name)
	}
	if err := h.libraryService.UpdateTags(r.Context(), id, remaining); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("tag", tagName).Msg("browse: remove tag")
		writeErr(w, http.StatusInternalServerError, "tag remove failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneOIncrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.IncrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: increment O")
		writeErr(w, http.StatusInternalServerError, "O increment failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneODecrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.DecrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: decrement O")
		writeErr(w, http.StatusInternalServerError, "O decrement failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) scenePlayIncrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.IncrementPlayCount(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: increment play count")
		writeErr(w, http.StatusInternalServerError, "play increment failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func (h *httpHandler) sceneOrganizedHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusInternalServerError, "scene not found")
		return
	}
	newState := !vd.SceneParts.Organized
	if err := h.libraryService.SetOrganized(r.Context(), id, newState); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: toggle organized")
		writeErr(w, http.StatusInternalServerError, "organized toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

// refreshSceneCache forceFetches the scene to refresh the in-memory cache.
// Called after a successful mutation so that buildSceneState reads the
// new state. Errors are logged but not surfaced — the mutation already
// succeeded.
func (h *httpHandler) refreshSceneCache(r *http.Request, id string) {
	if _, err := h.libraryService.GetScene(r.Context(), id, true); err != nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: refresh scene cache after mutation")
	}
}
