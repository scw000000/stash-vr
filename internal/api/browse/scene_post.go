package browse

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"stash-vr/internal/config"
	"stash-vr/internal/prefix"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// redirectBack issues a 303 to Referer (or /browse/scene/{id} fallback),
// setting (or clearing) the `err` query param. Using url.Values.Set avoids
// stacking multiple err= entries on consecutive failed POSTs.
func (h *httpHandler) redirectBack(w http.ResponseWriter, r *http.Request, errMsg string) {
	target := r.Header.Get("Referer")
	if target == "" {
		target = "/browse/scene/" + chi.URLParam(r, "id")
	}
	u, parseErr := url.Parse(target)
	if parseErr != nil {
		// Fallback: ignore broken Referer; redirect to the scene page without err param.
		http.Redirect(w, r, "/browse/scene/"+chi.URLParam(r, "id"), http.StatusSeeOther)
		return
	}
	q := u.Query()
	if errMsg == "" {
		q.Del("err")
	} else {
		q.Set("err", errMsg)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

func (h *httpHandler) sceneRatingHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.redirectBack(w, r, "bad form")
		return
	}
	val, parseErr := strconv.Atoi(r.PostForm.Get("value"))
	if parseErr != nil || val < 0 || val > 5 {
		h.redirectBack(w, r, "bad rating")
		return
	}

	// If the user clicked the same star that's already set, clear the rating.
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

	// Translate 0..5 into UpdateRating's *float32 in the 1..5 range. 0 -> nil clears.
	var rating5 *float32
	if val > 0 {
		f := float32(val)
		rating5 = &f
	}

	if err := h.libraryService.UpdateRating(r.Context(), id, rating5); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: update rating")
		h.redirectBack(w, r, "rating update failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	favTag := config.Application().FavoriteTag
	if favTag == "" {
		h.redirectBack(w, r, "FAVORITE_TAG not configured")
		return
	}

	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		h.redirectBack(w, r, "scene not found")
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
		h.redirectBack(w, r, "favorite toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneTagAddHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.redirectBack(w, r, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		h.redirectBack(w, r, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		h.redirectBack(w, r, "scene not found")
		return
	}
	current := make([]string, 0, len(vd.SceneParts.Tags)+1)
	exists := false
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		// Skip ancestor-only tags injected by decorateTags.
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
		h.redirectBack(w, r, "tag add failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneTagRemoveHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		h.redirectBack(w, r, "bad form")
		return
	}
	tagName := strings.TrimSpace(r.PostForm.Get("tag"))
	if tagName == "" {
		h.redirectBack(w, r, "empty tag")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		h.redirectBack(w, r, "scene not found")
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
		h.redirectBack(w, r, "tag remove failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneOIncrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.IncrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: increment O")
		h.redirectBack(w, r, "O increment failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneODecrementHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.DecrementO(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: decrement O")
		h.redirectBack(w, r, "O decrement failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

func (h *httpHandler) sceneOrganizedHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		h.redirectBack(w, r, "scene not found")
		return
	}
	newState := !vd.SceneParts.Organized
	if err := h.libraryService.SetOrganized(r.Context(), id, newState); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: toggle organized")
		h.redirectBack(w, r, "organized toggle failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.redirectBack(w, r, "")
}

// refreshSceneCache forceFetches the scene to refresh the in-memory cache.
// Called after a successful mutation so that the next read (typically the
// post-redirect detail page) reflects the new state. Errors are logged but
// not surfaced — the mutation itself already succeeded.
func (h *httpHandler) refreshSceneCache(r *http.Request, id string) {
	if _, err := h.libraryService.GetScene(r.Context(), id, true); err != nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: refresh scene cache after mutation")
	}
}
