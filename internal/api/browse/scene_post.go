package browse

import (
	"fmt"
	"net/http"
)

func (h *httpHandler) sceneRatingHandler(w http.ResponseWriter, r *http.Request)     { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneFavoriteHandler(w http.ResponseWriter, r *http.Request)   { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneTagAddHandler(w http.ResponseWriter, r *http.Request)     { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneTagRemoveHandler(w http.ResponseWriter, r *http.Request)  { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneOIncrementHandler(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneODecrementHandler(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "stub") }
func (h *httpHandler) sceneOrganizedHandler(w http.ResponseWriter, r *http.Request)  { fmt.Fprintln(w, "stub") }
