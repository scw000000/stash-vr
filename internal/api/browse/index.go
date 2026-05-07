package browse

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/static"
)

var browseTmpl = template.Must(template.ParseFS(static.Fs, "browse.gohtml"))

func (h *httpHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tab := q.Get("tab")

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}

	sidebar, err := LoadSidebar(r.Context(), h.libraryService.StashClient, tab, "")
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load sidebar")
		http.Error(w, "Couldn't reach Stash — check stash-vr logs.", http.StatusInternalServerError)
		return
	}

	ids, total, err := fetchSceneIDs(r.Context(), h.libraryService.StashClient, nil, page)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: fetch default scene ids")
		http.Error(w, "Couldn't list scenes.", http.StatusInternalServerError)
		return
	}

	baseURL := apiinternal.GetBaseUrl(r)
	cards, err := buildCards(r.Context(), h.libraryService, baseURL, ids)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: build cards")
		http.Error(w, "Couldn't render scenes.", http.StatusInternalServerError)
		return
	}

	pageMax := (total + pageSize - 1) / pageSize
	if pageMax < 1 {
		pageMax = 1
	}
	extra := r.URL.Query()
	extra.Del("page")
	prev, next := pagerURLs("/browse", page, pageMax, extra)

	data := PageData{
		Sidebar: sidebar,
		Header:  "All scenes — newest first",
		SubHead: fmt.Sprintf("%d scenes", total),
		Cards:   cards,
		PrevURL: prev,
		NextURL: next,
		PageNum: page,
		PageMax: pageMax,
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
