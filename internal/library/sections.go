package library

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"slices"
	"stash-vr/internal/config"
	"stash-vr/internal/stash"
	"stash-vr/internal/stash/filter"
	"stash-vr/internal/stash/gql"
	"sync"
)

type Section struct {
	Name string
	Ids  []string
}

// filterEntry is the typed result of getFilters: each entry is either a
// saved Stash filter (SavedFilter set) or an auto-section record (AutoID set).
// Name and Disabled come from the corresponding UserConfig.Filter override.
type filterEntry struct {
	SavedFilter *gql.SavedFilterParts
	AutoID      string // empty if SavedFilter is set
	Name        string // user override; materializer resolves a fallback if empty
	Disabled    bool
}

func (libraryService *Service) GetSections(ctx context.Context) ([]Section, error) {
	res, err, _ := libraryService.single.Do("sections", func() (interface{}, error) {
		// Reconcile auto-section records into UserConfig.Filters before reading.
		// This adds new entries for entities that crossed the threshold and
		// hard-prunes records whose entities no longer qualify.
		reconcileAutoSections(ctx, libraryService.StashClient)

		filters, err := libraryService.getFilters(ctx)
		if err != nil {
			return nil, err
		}

		var sections []Section
		if len(filters) == 0 {
			log.Ctx(ctx).Info().Msg("No saved scene filters found, creating default section with ALL scenes")
			sections, err = libraryService.getDefaultSections(ctx)
		} else {
			sections, err = libraryService.getSectionsByFilters(ctx, filters)
		}
		if err != nil {
			return nil, err
		}

		libraryService.muVdCache.Lock()
		for k := range libraryService.vdCache {
			delete(libraryService.vdCache, k)
		}

		libraryService.Stats.Links = 0
		for _, v := range sections {
			libraryService.Stats.Links += len(v.Ids)
			for _, id := range v.Ids {
				libraryService.vdCache[id] = nil
			}
		}
		libraryService.Stats.Scenes = len(libraryService.vdCache)
		libraryService.muVdCache.Unlock()

		log.Ctx(ctx).Info().Int("sections", len(sections)).Int("links", libraryService.Stats.Links).
			Int("scenes", libraryService.Stats.Scenes).
			Msg("Index built")

		_ = libraryService.LoadTags(ctx)

		log.Ctx(ctx).Debug().Int("tags", len(libraryService.tagCache)).Msg("Cached tags")

		return sections, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]Section), nil
}

func (libraryService *Service) getDefaultSections(ctx context.Context) ([]Section, error) {
	resp, err := gql.FindAllSceneIds(ctx, libraryService.StashClient)
	if err != nil {
		return nil, fmt.Errorf("FindAllSceneIds: %w", err)
	}
	allScenesSection := Section{
		Name: "All",
		Ids:  make([]string, len(resp.FindScenes.Scenes)),
	}
	for i := range resp.FindScenes.Scenes {
		allScenesSection.Ids[i] = resp.FindScenes.Scenes[i].Id
	}
	return []Section{allScenesSection}, nil
}

func (libraryService *Service) getSectionsByFilters(ctx context.Context, entries []filterEntry) ([]Section, error) {
	sections := make([]Section, len(entries))

	wg := sync.WaitGroup{}
	wg.Add(len(entries))

	for i, e := range entries {
		go func(i int, e filterEntry) {
			defer wg.Done()
			if e.Disabled {
				return
			}
			if e.SavedFilter != nil {
				libraryService.buildSavedFilterSection(ctx, i, sections, *e.SavedFilter, e.Name)
				return
			}
			if e.AutoID != "" {
				libraryService.buildAutoSection(ctx, i, sections, config.Filter{ID: e.AutoID, Name: e.Name})
			}
		}(i, e)
	}
	wg.Wait()
	sections = slices.DeleteFunc(sections, func(s Section) bool {
		return len(s.Ids) == 0
	})
	return sections, nil
}

func (libraryService *Service) buildSavedFilterSection(ctx context.Context, idx int, out []Section, f gql.SavedFilterParts, nameOverride string) {
	flog := log.Ctx(ctx).With().Str("filterId", f.Id).Str("name", f.Name).Logger()

	sceneFilter, err := filter.SavedFilterToSceneFilter(ctx, f)
	if err != nil {
		flog.Warn().Err(err).Interface("savedFilter", f).Msg("Failed to convert filter, skipping")
		return
	}
	resp, err := gql.FindSceneIdsByFilter(ctx, libraryService.StashClient, &sceneFilter.SceneFilter, &sceneFilter.FilterOpts)
	if err != nil {
		flog.Err(err).Interface("savedFilter", f).Interface("sceneFilter", sceneFilter).Msg("Failed to find scenes by filter, skipping")
		return
	}
	if len(resp.FindScenes.Scenes) == 0 {
		flog.Debug().Msg("Filter skipped: 0 scenes")
		return
	}

	name := f.Name
	if nameOverride != "" {
		name = nameOverride
	}
	out[idx] = Section{Name: name, Ids: make([]string, len(resp.FindScenes.Scenes))}
	for j, v := range resp.FindScenes.Scenes {
		out[idx].Ids[j] = v.Id
	}
	flog.Debug().Int("scenes", len(out[idx].Ids)).Msg("Section built")
}

func (libraryService *Service) buildAutoSection(ctx context.Context, idx int, out []Section, rec config.Filter) {
	flog := log.Ctx(ctx).With().Str("autoId", rec.ID).Logger()
	sec, err := materializeAutoSection(ctx, libraryService.StashClient, rec)
	if err != nil {
		flog.Warn().Err(err).Msg("auto-section materialize failed, skipping")
		return
	}
	if len(sec.Ids) == 0 {
		flog.Debug().Msg("auto-section skipped: 0 scenes")
		return
	}
	if sec.Name == "" {
		flog.Warn().Msg("auto-section produced empty name; skipping")
		return
	}
	out[idx] = sec
	flog.Debug().Int("scenes", len(sec.Ids)).Msg("auto-section built")
}

func (libraryService *Service) getFilters(ctx context.Context) ([]filterEntry, error) {
	savedFiltersResp, err := gql.FindSavedSceneFilters(ctx, libraryService.StashClient)
	if err != nil {
		return nil, fmt.Errorf("failed to find saved filters: %w", err)
	}

	userCfg := config.User(ctx)

	// Split user config: saved-filter overrides (numeric ids) drive ordering;
	// auto records always come from user config in their stored order.
	var savedOverrides []config.Filter
	autoRecords := make(map[string]config.Filter)
	autoOrder := make([]string, 0)
	for _, f := range userCfg.Filters {
		if isAutoID(f.ID) {
			autoRecords[f.ID] = f
			autoOrder = append(autoOrder, f.ID)
			continue
		}
		savedOverrides = append(savedOverrides, f)
	}

	// Resolve saved-filter ordering using existing logic.
	var savedSlice []gql.SavedFilterParts
	if len(savedOverrides) == 0 {
		savedSlice, err = libraryService.buildFiltersByFrontpage(ctx, savedFiltersResp)
		if err != nil {
			return nil, err
		}
	} else {
		savedSlice = buildFiltersByUserConfig(ctx, savedFiltersResp, savedOverrides)
	}
	out := make([]filterEntry, 0, len(savedSlice)+len(autoOrder))
	for i := range savedSlice {
		// Honor name override / disabled flag from UserConfig if present.
		var override config.Filter
		for _, ov := range savedOverrides {
			if ov.ID == savedSlice[i].Id {
				override = ov
				break
			}
		}
		out = append(out, filterEntry{
			SavedFilter: &savedSlice[i],
			Name:        override.Name,
			Disabled:    override.Disabled,
		})
	}

	// Append auto records in their UserConfig.Filters order. DefaultName
	// resolution happens in the materializer; the entry just carries IDs and
	// user override fields here.
	for _, id := range autoOrder {
		f := autoRecords[id]
		out = append(out, filterEntry{
			AutoID:   f.ID,
			Name:     f.Name,
			Disabled: f.Disabled,
		})
	}

	return out, nil
}

func (libraryService *Service) buildFiltersByFrontpage(ctx context.Context, savedFilters *gql.FindSavedSceneFiltersResponse) ([]gql.SavedFilterParts, error) {
	fpIds, err := stash.FindSavedFilterIdsByFrontPage(ctx, libraryService.StashClient)
	if err != nil {
		return nil, fmt.Errorf("failed to find frontpage filter IDs: %w", err)
	}

	var front []gql.SavedFilterParts

	for _, id := range fpIds {
		for _, f := range savedFilters.FindSavedFilters {
			if f.Id == id {
				front = append(front, f.SavedFilterParts)
				break
			}
		}
	}

	seen := make(map[string]struct{}, len(fpIds))
	for _, id := range fpIds {
		seen[id] = struct{}{}
	}
	var rest []gql.SavedFilterParts
	for _, f := range savedFilters.FindSavedFilters {
		if _, ok := seen[f.Id]; !ok {
			rest = append(rest, f.SavedFilterParts)
		}
	}
	out := append(front, rest...)

	log.Ctx(ctx).Debug().Int("count", len(out)).Msg("Filters built by frontpage")

	return out, nil
}

func buildFiltersByUserConfig(ctx context.Context, savedFilters *gql.FindSavedSceneFiltersResponse, cfgFilters []config.Filter) []gql.SavedFilterParts {
	stashFilters := savedFilters.FindSavedFilters
	stashFilterParts := make(map[string]gql.SavedFilterParts, len(stashFilters))
	for _, sf := range stashFilters {
		stashFilterParts[sf.Id] = sf.SavedFilterParts
	}

	out := make([]gql.SavedFilterParts, 0, len(stashFilters))
	seen := make(map[string]struct{}, len(stashFilters))

	// 1) Enabled cfgFilters in the given order.
	for _, cf := range cfgFilters {
		seen[cf.ID] = struct{}{}
		if cf.Disabled {
			continue
		}
		sf, ok := stashFilterParts[cf.ID]
		if !ok {
			continue
		}

		if cf.Name != "" {
			sf.Name = cf.Name
		}
		out = append(out, sf)
	}

	for _, s := range stashFilters {
		if _, done := seen[s.Id]; done {
			continue
		}
		out = append(out, s.SavedFilterParts)
	}

	log.Ctx(ctx).Debug().Int("count", len(out)).Msg("Filters built by user config")

	return out
}
