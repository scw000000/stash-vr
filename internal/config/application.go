package config

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"strings"
)

const (
	envKeyListenAddress      = "LISTEN_ADDRESS"
	envKeyStashGraphQLUrl    = "STASH_GRAPHQL_URL"
	envKeyStashApiKey        = "STASH_API_KEY"
	envKeyFavoriteTag        = "FAVORITE_TAG"
	envKeyLogLevel           = "LOG_LEVEL"
	envKeyDisableLogColor    = "DISABLE_LOG_COLOR"
	envKeyDisableRedact      = "DISABLE_REDACT"
	envKeyForceHTTPS         = "FORCE_HTTPS"
	envKeyHeatmapHeightPx    = "HEATMAP_HEIGHT_PX"
	envKeyExcludeSortName    = "EXCLUDE_SORT_NAME"
	envKeyUserConfigPath     = "CONFIG_PATH"
	envKeyGenerateSummaryIds = "GENERATE_SUMMARY_IDS"
	envKeyAutoSectionsPerformers = "AUTO_SECTIONS_PERFORMERS"
	envKeyMinScenesPerPerformer  = "MIN_SCENES_PER_PERFORMER"
	envKeyMaxPerformerSections   = "MAX_PERFORMER_SECTIONS"
	envKeyAutoSectionsTags       = "AUTO_SECTIONS_TAGS"
	envKeyTopNTags               = "TOP_N_TAGS"
	envKeyAutoSectionsAggregates = "AUTO_SECTIONS_AGGREGATES"
	envKeyAggregateRecentAdded   = "AGGREGATE_RECENT_ADDED"
	envKeyAggregateRecentPlayed  = "AGGREGATE_RECENT_PLAYED"
	envKeyAggregateHighlyRated   = "AGGREGATE_HIGHLY_RATED"
	envKeyAggregateUnwatched     = "AGGREGATE_UNWATCHED"
	envKeyHighlyRatedThreshold   = "HIGHLY_RATED_THRESHOLD"
	envKeyAggregateLimit         = "AGGREGATE_LIMIT"
)

type ApplicationConfig struct {
	ListenAddress      string
	StashGraphQLUrl    string
	StashApiKey        string
	FavoriteTag        string
	LogLevel           string
	DisableLogColor    bool
	IsRedactDisabled   bool
	ForceHTTPS         bool
	HeatmapHeightPx    int
	ExcludeSortName    string
	ConfigPath         string
	GenerateSummaryIds bool
	AutoSectionsPerformers bool
	MinScenesPerPerformer  int
	MaxPerformerSections   int
	AutoSectionsTags       bool
	TopNTags               int
	AutoSectionsAggregates bool
	AggregateRecentAdded   bool
	AggregateRecentPlayed  bool
	AggregateHighlyRated   bool
	AggregateUnwatched     bool
	HighlyRatedThreshold   int
	AggregateLimit         int
}

var applicationConfig ApplicationConfig

func Init() {
	pflag.String(envKeyListenAddress, ":9666", "Local address for Stash-VR to listen on")
	_ = viper.BindPFlag(envKeyListenAddress, pflag.Lookup(envKeyListenAddress))

	pflag.String(envKeyStashGraphQLUrl, "http://localhost:9999/graphql", "Url to Stash graphql")
	_ = viper.BindPFlag(envKeyStashGraphQLUrl, pflag.Lookup(envKeyStashGraphQLUrl))

	pflag.String(envKeyStashApiKey, "", "Stash API key")
	_ = viper.BindPFlag(envKeyStashApiKey, pflag.Lookup(envKeyStashApiKey))

	pflag.String(envKeyFavoriteTag, "FAVORITE", "Name of tag in Stash to hold scenes marked as favorites")
	_ = viper.BindPFlag(envKeyFavoriteTag, pflag.Lookup(envKeyFavoriteTag))

	pflag.String(envKeyLogLevel, "info", "Set log level - trace, debug, warn, info or error")
	_ = viper.BindPFlag(envKeyLogLevel, pflag.Lookup(envKeyLogLevel))

	pflag.Bool(envKeyDisableLogColor, false, "Disable colors in log output")
	_ = viper.BindPFlag(envKeyDisableLogColor, pflag.Lookup(envKeyDisableLogColor))

	pflag.Bool(envKeyDisableRedact, false, "Disable redacting sensitive information from logs")
	_ = viper.BindPFlag(envKeyDisableRedact, pflag.Lookup(envKeyDisableRedact))

	pflag.Bool(envKeyForceHTTPS, false, "Force Stash-VR to use HTTPS")
	_ = viper.BindPFlag(envKeyForceHTTPS, pflag.Lookup(envKeyForceHTTPS))

	pflag.Int(envKeyHeatmapHeightPx, 0, "Height of heatmaps")
	_ = viper.BindPFlag(envKeyHeatmapHeightPx, pflag.Lookup(envKeyHeatmapHeightPx))

	pflag.String(envKeyExcludeSortName, "hidden", "Exclude tags with this sort name")
	_ = viper.BindPFlag(envKeyExcludeSortName, pflag.Lookup(envKeyExcludeSortName))

	pflag.String(envKeyUserConfigPath, "", "Path to store user config (may contain filter names in plain text)")
	_ = viper.BindPFlag(envKeyUserConfigPath, pflag.Lookup(envKeyUserConfigPath))

	pflag.String(envKeyGenerateSummaryIds, "", "Generate summary ids for categorized tags")
	_ = viper.BindPFlag(envKeyGenerateSummaryIds, pflag.Lookup(envKeyGenerateSummaryIds))

	pflag.Bool(envKeyAutoSectionsPerformers, false, "Generate one HereSphere section per performer above MIN_SCENES_PER_PERFORMER")
	_ = viper.BindPFlag(envKeyAutoSectionsPerformers, pflag.Lookup(envKeyAutoSectionsPerformers))

	pflag.Int(envKeyMinScenesPerPerformer, 5, "Minimum scene count for a performer to get an auto-section")
	_ = viper.BindPFlag(envKeyMinScenesPerPerformer, pflag.Lookup(envKeyMinScenesPerPerformer))

	pflag.Int(envKeyMaxPerformerSections, 50, "Cap on per-performer auto-sections (ranked by scene_count desc)")
	_ = viper.BindPFlag(envKeyMaxPerformerSections, pflag.Lookup(envKeyMaxPerformerSections))

	pflag.Bool(envKeyAutoSectionsTags, false, "Generate one HereSphere section per tag in the top TOP_N_TAGS by scene_count")
	_ = viper.BindPFlag(envKeyAutoSectionsTags, pflag.Lookup(envKeyAutoSectionsTags))

	pflag.Int(envKeyTopNTags, 20, "Number of tag auto-sections")
	_ = viper.BindPFlag(envKeyTopNTags, pflag.Lookup(envKeyTopNTags))

	pflag.Bool(envKeyAutoSectionsAggregates, false, "Generate aggregate sections (Recently Added/Played/Highly Rated/Unwatched)")
	_ = viper.BindPFlag(envKeyAutoSectionsAggregates, pflag.Lookup(envKeyAutoSectionsAggregates))

	pflag.Bool(envKeyAggregateRecentAdded, true, "Sub-toggle for the Recently Added aggregate section")
	_ = viper.BindPFlag(envKeyAggregateRecentAdded, pflag.Lookup(envKeyAggregateRecentAdded))

	pflag.Bool(envKeyAggregateRecentPlayed, true, "Sub-toggle for the Recently Played aggregate section")
	_ = viper.BindPFlag(envKeyAggregateRecentPlayed, pflag.Lookup(envKeyAggregateRecentPlayed))

	pflag.Bool(envKeyAggregateHighlyRated, true, "Sub-toggle for the Highly Rated aggregate section")
	_ = viper.BindPFlag(envKeyAggregateHighlyRated, pflag.Lookup(envKeyAggregateHighlyRated))

	pflag.Bool(envKeyAggregateUnwatched, true, "Sub-toggle for the Unwatched aggregate section")
	_ = viper.BindPFlag(envKeyAggregateUnwatched, pflag.Lookup(envKeyAggregateUnwatched))

	pflag.Int(envKeyHighlyRatedThreshold, 80, "Stash 0-100 rating; scenes >= threshold qualify for Highly Rated")
	_ = viper.BindPFlag(envKeyHighlyRatedThreshold, pflag.Lookup(envKeyHighlyRatedThreshold))

	pflag.Int(envKeyAggregateLimit, 100, "Per-aggregate max scene count")
	_ = viper.BindPFlag(envKeyAggregateLimit, pflag.Lookup(envKeyAggregateLimit))

	pflag.BoolP("help", "h", false, "Display usage information")
	_ = viper.BindPFlag("help", pflag.Lookup("help"))

	pflag.Parse()

	if viper.GetBool("help") {
		pflag.Usage()
		os.Exit(1)
	}

	viper.AutomaticEnv()

	applicationConfig.ListenAddress = viper.GetString(envKeyListenAddress)
	applicationConfig.StashGraphQLUrl = viper.GetString(envKeyStashGraphQLUrl)
	applicationConfig.StashApiKey = viper.GetString(envKeyStashApiKey)
	applicationConfig.FavoriteTag = viper.GetString(envKeyFavoriteTag)
	applicationConfig.LogLevel = strings.ToLower(viper.GetString(envKeyLogLevel))
	applicationConfig.DisableLogColor = viper.GetBool(envKeyDisableLogColor)
	applicationConfig.IsRedactDisabled = viper.GetBool(envKeyDisableRedact)
	applicationConfig.ForceHTTPS = viper.GetBool(envKeyForceHTTPS)
	applicationConfig.HeatmapHeightPx = viper.GetInt(envKeyHeatmapHeightPx)
	applicationConfig.ExcludeSortName = viper.GetString(envKeyExcludeSortName)
	applicationConfig.ConfigPath = viper.GetString(envKeyUserConfigPath)
	applicationConfig.GenerateSummaryIds = viper.GetBool(envKeyGenerateSummaryIds)
	applicationConfig.AutoSectionsPerformers = viper.GetBool(envKeyAutoSectionsPerformers)
	applicationConfig.MinScenesPerPerformer = viper.GetInt(envKeyMinScenesPerPerformer)
	applicationConfig.MaxPerformerSections = viper.GetInt(envKeyMaxPerformerSections)
	applicationConfig.AutoSectionsTags = viper.GetBool(envKeyAutoSectionsTags)
	applicationConfig.TopNTags = viper.GetInt(envKeyTopNTags)
	applicationConfig.AutoSectionsAggregates = viper.GetBool(envKeyAutoSectionsAggregates)
	applicationConfig.AggregateRecentAdded = viper.GetBool(envKeyAggregateRecentAdded)
	applicationConfig.AggregateRecentPlayed = viper.GetBool(envKeyAggregateRecentPlayed)
	applicationConfig.AggregateHighlyRated = viper.GetBool(envKeyAggregateHighlyRated)
	applicationConfig.AggregateUnwatched = viper.GetBool(envKeyAggregateUnwatched)
	applicationConfig.HighlyRatedThreshold = viper.GetInt(envKeyHighlyRatedThreshold)
	applicationConfig.AggregateLimit = viper.GetInt(envKeyAggregateLimit)

}

func Application() ApplicationConfig {
	return applicationConfig
}

func (a ApplicationConfig) Redacted() ApplicationConfig {
	a.StashGraphQLUrl = Redacted(a.StashGraphQLUrl)
	a.StashApiKey = Redacted(a.StashApiKey)
	a.ConfigPath = Redacted(a.ConfigPath)
	return a
}
