package browse

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/stash/gql"
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

func (h *httpHandler) entityHandler(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}

		sidebar, err := LoadSidebar(r.Context(), h.libraryService.StashClient, kind, id)
		if err != nil {
			log.Ctx(r.Context()).Err(err).Msg("browse: load sidebar")
			http.Error(w, "Couldn't reach Stash — check stash-vr logs.", http.StatusInternalServerError)
			return
		}

		var sceneFilter *gql.SceneFilterType
		switch kind {
		case "perf":
			sceneFilter = &gql.SceneFilterType{
				Performers: &gql.MultiCriterionInput{
					Value:    []string{id},
					Modifier: gql.CriterionModifierIncludesAll,
				},
			}
		case "studio":
			sceneFilter = &gql.SceneFilterType{
				Studios: &gql.HierarchicalMultiCriterionInput{
					Value:    []string{id},
					Modifier: gql.CriterionModifierIncludesAll,
				},
			}
		case "tag":
			sceneFilter = &gql.SceneFilterType{
				Tags: &gql.HierarchicalMultiCriterionInput{
					Value:    []string{id},
					Modifier: gql.CriterionModifierIncludesAll,
				},
			}
		default:
			http.NotFound(w, r)
			return
		}

		ids, total, err := fetchSceneIDs(r.Context(), h.libraryService.StashClient, sceneFilter, page)
		if err != nil {
			log.Ctx(r.Context()).Err(err).Msg("browse: fetch entity scenes")
			http.Error(w, "Couldn't list scenes for this entity.", http.StatusInternalServerError)
			return
		}

		baseURL := apiinternal.GetBaseUrl(r)
		cards, err := buildCards(r.Context(), h.libraryService, baseURL, ids)
		if err != nil {
			log.Ctx(r.Context()).Err(err).Msg("browse: build cards")
			http.Error(w, "Couldn't render scenes.", http.StatusInternalServerError)
			return
		}

		entityName := lookupEntityName(sidebar, kind, id)
		header := entityHeader(kind, entityName)

		pageMax := (total + pageSize - 1) / pageSize
		if pageMax < 1 {
			pageMax = 1
		}
		extra := r.URL.Query()
		extra.Del("page")
		prev, next := pagerURLs("/browse/"+kind+"/"+id, page, pageMax, extra)

		data := PageData{
			Sidebar: sidebar,
			BackURL: "/browse",
			Header:  header,
			SubHead: fmt.Sprintf("%d scenes", total),
			Cards:   cards,
			PrevURL: prev,
			NextURL: next,
			PageNum: page,
			PageMax: pageMax,
		}
		if err := browseTmpl.Execute(w, data); err != nil {
			log.Ctx(r.Context()).Err(err).Msg("browse: render entity")
		}
	}
}

func lookupEntityName(sb SidebarData, kind, id string) string {
	var list []Entity
	switch kind {
	case "perf":
		list = sb.Performers
	case "studio":
		list = sb.Studios
	case "tag":
		list = sb.Tags
	}
	for _, e := range list {
		if e.ID == id {
			return e.Name
		}
	}
	return id
}

func entityHeader(kind, name string) string {
	switch kind {
	case "perf":
		return "Performer: " + name
	case "studio":
		return "Studio: " + name
	case "tag":
		return "Tag: " + name
	}
	return name
}
