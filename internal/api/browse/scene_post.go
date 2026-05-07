package browse

import (
	"net/http"
	"net/url"
	"strconv"

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
	h.redirectBack(w, r, "")
}

// Stubs — replaced in Tasks 9-12.
func (h *httpHandler) sceneFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
func (h *httpHandler) sceneTagAddHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
func (h *httpHandler) sceneTagRemoveHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
func (h *httpHandler) sceneOIncrementHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
func (h *httpHandler) sceneODecrementHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
func (h *httpHandler) sceneOrganizedHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
