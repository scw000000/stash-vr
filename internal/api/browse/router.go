package browse

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"stash-vr/internal/library"
)

type httpHandler struct {
	libraryService *library.Service
	filterCache    *filterCache
}

func Router(libraryService *library.Service) http.Handler {
	h := httpHandler{
		libraryService: libraryService,
		filterCache:    newFilterCache(loadFilterIndexCatalog, buildMatrixSeeds, 5*time.Minute),
	}
	r := chi.NewRouter()

	r.Get("/", h.indexHandler)
	r.Get("/grid", h.gridJSONHandler)
	r.Get("/filter-index", h.filterIndexHandler)
	r.Get("/filter-catalog", h.filterCatalogHandler)
	r.Get("/filter-options/{kind}", h.filterOptionsHandler)
	r.Get("/perf/{id}", h.entityHandler("perf"))
	r.Get("/studio/{id}", h.entityHandler("studio"))
	r.Get("/tag/{id}", h.entityHandler("tag"))

	r.Get("/scene/{id}", h.sceneDetailHandler)
	r.Get("/scene/{id}/stream", h.sceneStreamHandler)
	r.Get("/scene/{id}/caption", h.sceneCaptionHandler)
	r.Get("/scene/{id}/preview", h.scenePreviewHandler)
	r.Get("/scene/{id}/sprite", h.sceneSpriteHandler)
	r.Get("/scene/{id}/meta", h.sceneMetaHandler)
	r.Post("/scene/{id}/rating", h.sceneRatingHandler)
	r.Post("/scene/{id}/favorite", h.sceneFavoriteHandler)
	r.Post("/scene/{id}/tags/add", h.sceneTagAddHandler)
	r.Post("/scene/{id}/tags/remove", h.sceneTagRemoveHandler)
	r.Post("/scene/{id}/projection", h.sceneProjectionHandler)
	r.Post("/scene/{id}/o/increment", h.sceneOIncrementHandler)
	r.Post("/scene/{id}/o/decrement", h.sceneODecrementHandler)
	r.Post("/scene/{id}/organized", h.sceneOrganizedHandler)

	return r
}
