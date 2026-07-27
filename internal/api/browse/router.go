package browse

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"stash-vr/internal/library"
	"stash-vr/internal/subtitles"
)

type httpHandler struct {
	libraryService  *library.Service
	subtitleService *subtitles.Service
	filterCache     *filterCache
}

func Router(libraryService *library.Service, subtitleService *subtitles.Service) http.Handler {
	h := httpHandler{
		libraryService:  libraryService,
		subtitleService: subtitleService,
		filterCache:     newFilterCache(loadFilterIndexCatalog, buildMatrixSeeds, 5*time.Minute),
	}
	r := chi.NewRouter()

	r.Get("/", h.indexHandler)
	r.Get("/grid", h.gridJSONHandler)
	r.Get("/filter-index", h.filterIndexHandler)
	r.Get("/filter-catalog", h.filterCatalogHandler)
	r.Get("/filter-matrix", h.filterMatrixHandler)
	r.Get("/filter-options/{kind}", h.filterOptionsHandler)
	r.Get("/scene-options/{kind}", h.sceneVREditOptionsHandler)
	r.Get("/scene/{id}/stash-boxes", h.sceneVRStashBoxesHandler)
	r.Get("/perf/{id}", h.entityHandler("perf"))
	r.Get("/studio/{id}", h.entityHandler("studio"))
	r.Get("/tag/{id}", h.entityHandler("tag"))

	r.Get("/scene/{id}", h.sceneDetailHandler)
	r.Get("/scene/{id}/stream", h.sceneStreamHandler)
	r.Get("/scene/{id}/caption", h.sceneCaptionHandler)
	r.Get("/scene/{id}/subtitles", h.sceneSubtitlesHandler)
	r.Post("/scene/{id}/subtitles/generate", h.sceneSubtitleGenerateHandler)
	r.Post("/scene/{id}/subtitles/delete", h.sceneSubtitleDeleteHandler)
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
	r.Post("/scene/{id}/play/increment", h.scenePlayIncrementHandler)
	r.Post("/scene/{id}/organized", h.sceneOrganizedHandler)
	r.Post("/scene/{id}/update", h.sceneVRUpdateHandler)
	r.Post("/scene/{id}/relations", h.sceneVRRelationsHandler)
	r.Post("/scene/{id}/file/{fileID}/{action}", h.sceneVRFileActionHandler)
	r.Post("/scene/{id}/marker", h.sceneVRMarkerHandler)
	r.Post("/scene/{id}/marker/{markerID}", h.sceneVRMarkerHandler)
	r.Post("/scene/{id}/marker/{markerID}/delete", h.sceneVRMarkerHandler)
	r.Post("/scene/{id}/history/{kind}/{action}", h.sceneVRHistoryHandler)
	r.Post("/scene/{id}/delete", h.sceneVRDeleteHandler)
	r.Post("/scene/{id}/operation/{operation}", h.sceneVROperationHandler)

	return r
}
