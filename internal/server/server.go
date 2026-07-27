package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"net/http"
	"stash-vr/internal/api"
	"stash-vr/internal/cert"
	"stash-vr/internal/library"
	"stash-vr/internal/subtitles"
	"time"
)

func Listen(ctx context.Context, listenAddress string, httpsListenAddress string, libraryService *library.Service, subtitleService *subtitles.Service) error {
	handler := api.Router(libraryService, subtitleService)

	httpServer := http.Server{Addr: listenAddress, Handler: handler}
	servers := []*http.Server{&httpServer}

	var httpsServer *http.Server
	var certPath, keyPath string
	if httpsListenAddress != "" {
		var err error
		certPath, keyPath, err = cert.Paths()
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("HTTPS disabled: cannot resolve cert path")
		} else if err := cert.EnsureSelfSigned(certPath, keyPath); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("HTTPS disabled: cannot create self-signed cert")
		} else {
			httpsServer = &http.Server{Addr: httpsListenAddress, Handler: handler}
			servers = append(servers, httpsServer)
		}
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Ctx(ctx).Info().Msg(fmt.Sprintf("HTTP server listening at %s", listenAddress))
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen http: %w", err)
		}
		return nil
	})

	if httpsServer != nil {
		g.Go(func() error {
			log.Ctx(ctx).Info().Str("cert", certPath).Msg(fmt.Sprintf("HTTPS server listening at %s", httpsListenAddress))
			if err := httpsServer.ListenAndServeTLS(certPath, keyPath); !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("listen https: %w", err)
			}
			return nil
		})
	}

	g.Go(func() error {
		<-gCtx.Done()

		ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		go func() {
			<-ctxShutdown.Done()
			if errors.Is(ctxShutdown.Err(), context.DeadlineExceeded) {
				log.Ctx(ctx).Warn().Err(ctxShutdown.Err()).Msg("Shutdown timed out")
			}
		}()

		for _, s := range servers {
			if err := s.Shutdown(ctxShutdown); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Server shutdown error")
			}
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	log.Ctx(ctx).Debug().Msg("Server stopped without error")
	return nil
}
