package internal

import (
	"context"
	"fmt"
	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/build"
	"stash-vr/internal/config"
	"stash-vr/internal/library"
	"stash-vr/internal/logger"
	"stash-vr/internal/server"
	"stash-vr/internal/stash"
	"stash-vr/internal/subtitles"
)

func Run(ctx context.Context) error {
	config.Init()
	log.Logger = logger.New(config.Application().LogLevel, config.Application().DisableLogColor)
	zerolog.DefaultContextLogger = &log.Logger

	log.Info().Str("config", fmt.Sprintf("%+v", config.Application().Redacted())).Send()

	stashClient := stash.NewClient(config.Application().StashGraphQLUrl, config.Application().StashApiKey)
	logVersions(ctx, stashClient)

	libraryService := library.NewService(stashClient)
	subtitleService := subtitles.New(ctx, subtitles.Config{
		PythonExecutable: config.Application().CaptionPython,
		GeneratorPath:    config.Application().CaptionGeneratorPath,
		EnvFile:          config.Application().CaptionEnvFile,
		StashClient:      stashClient,
	})
	defer subtitleService.Close()
	if err := subtitleService.Prepare(ctx); err != nil {
		log.Warn().Err(err).Msg("Subtitle task plugin is unavailable")
	}

	err := server.Listen(ctx, config.Application().ListenAddress, config.Application().HTTPSListenAddress, libraryService, subtitleService)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}

func logVersions(ctx context.Context, client graphql.Client) {
	log.Info().Str("Stash-VR version", build.FullVersion()).Send()

	if version, err := stash.GetVersion(ctx, client); err != nil {
		log.Warn().Err(err).Msg("Failed to retrieve stash version")
	} else {
		log.Info().Str("Stash version", version).Send()
	}
}
