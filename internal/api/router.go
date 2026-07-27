package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"stash-vr/internal/api/browse"
	"stash-vr/internal/api/deovr"
	"stash-vr/internal/api/heatmap"
	"stash-vr/internal/api/heresphere"
	"stash-vr/internal/api/web"
	"stash-vr/internal/cert"
	"stash-vr/internal/config"
	"stash-vr/internal/library"
	"stash-vr/internal/static"
	"stash-vr/internal/subtitles"
	"stash-vr/internal/util"
	"time"
)

func Router(libraryService *library.Service, subtitleService *subtitles.Service) *chi.Mux {
	router := chi.NewRouter()

	router.Use(requestLogger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Compress(5, "application/json"))

	//router.Mount("/debug", middleware.Profiler())

	router.Mount("/heresphere", logMod("heresphere", heresphere.Router(libraryService)))
	router.Mount("/deovr", logMod("deovr", deovr.Router(libraryService)))
	router.Mount("/browse", logMod("browse", browse.Router(libraryService, subtitleService)))

	router.Post("/filters", logMod("filters", web.FiltersUpdateHandler()).ServeHTTP)
	router.Get("/cover/{videoId}", logMod("heatmap", heatmap.CoverHandler(libraryService)).ServeHTTP)
	router.Get("/ca.crt", caCertHandler)

	router.Get("/", web.IndexHandler(libraryService).ServeHTTP)

	router.Get("/*", http.FileServerFS(static.Fs).ServeHTTP)

	return router
}

func caCertHandler(w http.ResponseWriter, r *http.Request) {
	certPath, _, err := cert.Paths()
	if err != nil {
		http.Error(w, "cert path unavailable", http.StatusInternalServerError)
		return
	}
	data, err := os.ReadFile(certPath)
	if err != nil {
		http.Error(w, "self-signed cert not yet generated; HTTPS may be disabled", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="stash-vr-ca.crt"`)
	w.Write(data)
}

func logMod(value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.Ctx(r.Context()).With().Str("mod", value).Logger().WithContext(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme := util.GetScheme(r)
		url := scheme + "://" + config.Redacted(r.Host) + r.RequestURI

		baseLogger := log.Ctx(r.Context()).With().
			Str("method", r.Method).
			Str("url", url).Logger()

		baseLogger.Debug().
			Str("proto", r.Proto).
			Str("user_agent", r.UserAgent()).
			Msg("Incoming request")

		start := time.Now()
		next.ServeHTTP(w, r)

		baseLogger.Trace().
			Dur("ms", time.Since(start)).
			Msg("Request handled")
	})
}
