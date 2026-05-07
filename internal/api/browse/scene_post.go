package browse

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// redirectBack issues a 303 to Referer (or /browse/scene/{id} fallback),
// optionally appending an `err` query param when err != "".
func (h *httpHandler) redirectBack(w http.ResponseWriter, r *http.Request, err string) {
	target := r.Header.Get("Referer")
	if target == "" {
		target = "/browse/scene/" + chi.URLParam(r, "id")
	}
	if err != "" {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "err=" + url.QueryEscape(err)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
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
