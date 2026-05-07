package browse

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
	"stash-vr/internal/static"
)

var browseTmpl = template.Must(template.ParseFS(static.Fs, "browse.gohtml"))

func (h *httpHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")

	sidebar, err := LoadSidebar(r.Context(), h.libraryService.StashClient, tab, "")
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load sidebar")
		http.Error(w, "failed to load sidebar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := PageData{
		Sidebar: sidebar,
		Header:  "All scenes — newest first",
		SubHead: "",
		Cards:   nil, // grid implemented in next task
	}

	if err := browseTmpl.Execute(w, data); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: render index")
	}
}

// entityHandler still a stub; replaced in Task 6.
func (h *httpHandler) entityHandler(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "entity browse not yet implemented", http.StatusNotImplemented)
	}
}
