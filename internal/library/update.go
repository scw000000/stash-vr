package library

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"stash-vr/internal/config"
	"stash-vr/internal/stash"
	"stash-vr/internal/stash/gql"
	"stash-vr/internal/util"
)

func (libraryService *Service) UpdateRating(ctx context.Context, id string, rating5 *float32) error {
	var newRating100 *int
	if rating5 != nil {
		converted := int(*rating5 * 20)
		newRating100 = &converted
	}

	_, err := gql.SceneUpdateRating100(ctx, libraryService.StashClient, id, newRating100)
	if err != nil {
		return fmt.Errorf("SceneUpdateRating100: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateFavorite(ctx context.Context, id string, isFavoriteRequested bool) error {
	favoriteTagName := config.Application().FavoriteTag

	if favoriteTagName == "" {
		log.Ctx(ctx).Info().Msg("Sync favorite requested but FAVORITE_TAG is empty, ignoring request")
		return nil
	}

	favoriteTagId, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, favoriteTagName)
	if err != nil {
		return err
	}

	response, err := gql.FindSceneTags(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("FindSceneTags: %w", err)
	}

	newTagIds := make([]string, 0, len(response.FindScene.Tags)+1)

	var hasFavoriteTag bool
	for _, t := range response.FindScene.Tags {
		if t.Id == favoriteTagId {
			hasFavoriteTag = true
			if !isFavoriteRequested {
				continue
			}
		}
		newTagIds = append(newTagIds, t.Id)
	}
	if !hasFavoriteTag && isFavoriteRequested {
		newTagIds = append(newTagIds, favoriteTagId)
	}

	if _, err := gql.SceneUpdateTags(ctx, libraryService.StashClient, id, newTagIds); err != nil {
		return fmt.Errorf("SceneUpdateTags: %w", err)
	}

	return nil
}

func (libraryService *Service) UpdateTags(ctx context.Context, id string, tags []string) error {
	tagIds := make([]string, len(tags))
	for i, tag := range tags {
		tagId, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, tag)
		if err != nil {
			return err
		}
		tagIds[i] = tagId
	}
	if _, err := gql.SceneUpdateTags(ctx, libraryService.StashClient, id, tagIds); err != nil {
		return fmt.Errorf("SceneUpdateTags: %w", err)
	}
	return nil
}

type MarkerDto struct {
	PrimaryTagName string
	StartSecond    float64
	EndSecond      *float64
	Title          string
	MarkerId       string //hack: use the rating field for transport of marker id
}

func (libraryService *Service) UpdateMarkers(ctx context.Context, id string, incomingMarkers []MarkerDto) error {
	vd, err := libraryService.GetScene(ctx, id, false)
	if err != nil {
		return err
	}

	markersToDestroy := make([]string, 0)
	for _, existingMarker := range vd.SceneParts.Scene_markers {
		if !slices.ContainsFunc(incomingMarkers, func(m MarkerDto) bool {
			return m.MarkerId == existingMarker.Id
		}) {
			markersToDestroy = append(markersToDestroy, existingMarker.Id)
		}
	}

	markersToUpdate := make([]MarkerDto, 0)
	markersToCreate := make([]MarkerDto, 0)

	for _, incoming := range incomingMarkers {
		if incoming.MarkerId != "" && incoming.MarkerId != "0" && slices.ContainsFunc(vd.SceneParts.Scene_markers, func(existingMarker *gql.ScenePartsScene_markersSceneMarker) bool {
			return incoming.MarkerId == existingMarker.Id
		}) {
			markersToUpdate = append(markersToUpdate, incoming)
		} else {
			markersToCreate = append(markersToCreate, incoming)
		}
	}

	for _, m := range markersToUpdate {
		tagId, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, m.PrimaryTagName)
		if err != nil {
			return fmt.Errorf("failed to find or create primary tag for marker: %w", err)
		}
		_, err = gql.SceneMarkerUpdate(ctx, libraryService.StashClient, m.MarkerId, tagId, m.StartSecond, m.EndSecond, m.Title)
		if err != nil {
			return fmt.Errorf("SceneMarkerCreate: %w", err)
		}
	}
	for _, m := range markersToCreate {
		tagId, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, m.PrimaryTagName)
		if err != nil {
			return fmt.Errorf("failed to find or create primary tag for marker: %w", err)
		}
		_, err = gql.SceneMarkerCreate(ctx, libraryService.StashClient, id, tagId, m.StartSecond, m.EndSecond, m.Title)
		if err != nil {
			return fmt.Errorf("SceneMarkerCreate: %w", err)
		}
	}

	_, err = gql.SceneMarkersDestroy(ctx, libraryService.StashClient, markersToDestroy)
	if err != nil {
		return fmt.Errorf("SceneMarkersDestroy: %w", err)
	}

	return nil
}

func (libraryService *Service) ClearAndCreateMarkers(ctx context.Context, id string, markers []MarkerDto) error {
	resp, err := gql.FindSceneMarkers(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("FindSceneMarkers: %w", err)
	}
	currentMarkers := make([]MarkerDto, len(resp.FindSceneMarkers.Scene_markers))
	for i, m := range resp.FindSceneMarkers.Scene_markers {
		currentMarkers[i] = MarkerDto{
			PrimaryTagName: m.Primary_tag.Name,
			StartSecond:    m.Seconds * 1000,
			Title:          m.Title,
		}
		if m.End_seconds != nil {
			currentMarkers[i].EndSecond = util.Ptr(*m.End_seconds * 1000)
		}
	}
	if util.UnorderedEqual(currentMarkers, markers) {
		return nil
	}
	markersToDestroy := make([]string, len(resp.FindSceneMarkers.Scene_markers))
	for i, sm := range resp.FindSceneMarkers.Scene_markers {
		markersToDestroy[i] = sm.Id
	}
	_, err = gql.SceneMarkersDestroy(ctx, libraryService.StashClient, markersToDestroy)
	if err != nil {
		return fmt.Errorf("SceneMarkersDestroy: %w", err)
	}

	for _, m := range markers {
		tagId, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, m.PrimaryTagName)
		if err != nil {
			return fmt.Errorf("failed to find or create primary tag for marker: %w", err)
		}
		_, err = gql.SceneMarkerCreate(ctx, libraryService.StashClient, id, tagId, m.StartSecond, m.EndSecond, m.Title)
		if err != nil {
			return fmt.Errorf("SceneMarkerCreate: %w", err)
		}
	}
	return nil
}

func (libraryService *Service) Delete(ctx context.Context, id string) error {
	if _, err := gql.SceneDestroy(ctx, libraryService.StashClient, id); err != nil {
		return fmt.Errorf("SceneDestroy: %w", err)
	}
	return nil
}

func (libraryService *Service) IncrementO(ctx context.Context, id string) error {
	_, err := gql.SceneIncrementO(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("SceneIncrementO: %w", err)
	}
	return nil
}

func (libraryService *Service) DecrementO(ctx context.Context, id string) error {
	_, err := gql.SceneDecrementO(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("SceneDecrementO: %w", err)
	}
	return nil
}

func (libraryService *Service) IncrementPlayCount(ctx context.Context, id string) error {
	_, err := gql.SceneIncrementPlayCount(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("SceneIncrementPlayCount: %w", err)
	}
	return nil
}

func (libraryService *Service) DecrementPlayCount(ctx context.Context, id string) error {
	_, err := gql.SceneDecrementPlayCount(ctx, libraryService.StashClient, id)
	if err != nil {
		return fmt.Errorf("SceneDecrementPlayCount: %w", err)
	}
	return nil
}

func (libraryService *Service) SetOrganized(ctx context.Context, id string, newState bool) error {
	_, err := gql.SceneUpdateOrganized(ctx, libraryService.StashClient, id, &newState)
	if err != nil {
		return fmt.Errorf("SceneUpdateOrganized: %w", err)
	}
	return nil
}

// UpdateSceneVRField changes precisely one text field. SceneUpdateInput's
// generated JSON shape includes null-valued fields, so it cannot safely be
// used for partial Stash edits.
func (libraryService *Service) UpdateSceneVRField(ctx context.Context, sceneID, field, value string) error {
	var err error
	switch field {
	case "title":
		_, err = gql.SceneUpdateTitleVR(ctx, libraryService.StashClient, sceneID, &value)
	case "code":
		_, err = gql.SceneUpdateCodeVR(ctx, libraryService.StashClient, sceneID, &value)
	case "date":
		_, err = gql.SceneUpdateDateVR(ctx, libraryService.StashClient, sceneID, &value)
	case "director":
		_, err = gql.SceneUpdateDirectorVR(ctx, libraryService.StashClient, sceneID, &value)
	case "details":
		_, err = gql.SceneUpdateDetailsVR(ctx, libraryService.StashClient, sceneID, &value)
	case "coverImage":
		_, err = gql.SceneUpdateCoverImageVR(ctx, libraryService.StashClient, sceneID, &value)
	default:
		return fmt.Errorf("unsupported VR scene field %q", field)
	}
	if err != nil {
		return fmt.Errorf("update VR scene %s: %w", field, err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRURLs(ctx context.Context, sceneID string, urls []string) error {
	if _, err := gql.SceneUpdateURLsVR(ctx, libraryService.StashClient, sceneID, urls); err != nil {
		return fmt.Errorf("update VR scene urls: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRCustomFields(ctx context.Context, sceneID string, fields map[string]interface{}) error {
	if _, err := gql.SceneUpdateCustomFieldsVR(ctx, libraryService.StashClient, sceneID, &gql.CustomFieldsInput{Full: &fields}); err != nil {
		return fmt.Errorf("update VR scene custom fields: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRTags(ctx context.Context, sceneID string, ids []string) error {
	if _, err := gql.SceneUpdateTagsVR(ctx, libraryService.StashClient, sceneID, ids); err != nil {
		return fmt.Errorf("update VR scene tags: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRPerformers(ctx context.Context, sceneID string, ids []string) error {
	if _, err := gql.SceneUpdatePerformersVR(ctx, libraryService.StashClient, sceneID, ids); err != nil {
		return fmt.Errorf("update VR scene performers: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRGalleries(ctx context.Context, sceneID string, ids []string) error {
	if _, err := gql.SceneUpdateGalleryVR(ctx, libraryService.StashClient, sceneID, ids); err != nil {
		return fmt.Errorf("update VR scene galleries: %w", err)
	}
	return nil
}

func (libraryService *Service) UpdateSceneVRGroups(ctx context.Context, sceneID string, groups []*gql.SceneGroupInput) error {
	if _, err := gql.SceneUpdateGroupsVR(ctx, libraryService.StashClient, sceneID, groups); err != nil {
		return fmt.Errorf("update VR scene groups: %w", err)
	}
	return nil
}

func (libraryService *Service) SetSceneVRStudio(ctx context.Context, sceneID string, studioID *string) error {
	var err error
	if studioID == nil {
		_, err = gql.SceneClearStudioVR(ctx, libraryService.StashClient, sceneID)
	} else {
		_, err = gql.SceneSetStudioVR(ctx, libraryService.StashClient, sceneID, *studioID)
	}
	if err != nil {
		return fmt.Errorf("update VR scene studio: %w", err)
	}
	return nil
}

// StashBoxRef intentionally exposes only the name and endpoint required by
// the VR chooser. API keys stay inside Stash's configuration response.
type StashBoxRef struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

func (libraryService *Service) SceneStashBoxesVR(ctx context.Context) ([]StashBoxRef, error) {
	response, err := gql.FindSceneStashBoxesVR(ctx, libraryService.StashClient)
	if err != nil {
		return nil, fmt.Errorf("load Stash Boxes: %w", err)
	}
	if response == nil || response.Configuration == nil || response.Configuration.General == nil {
		return []StashBoxRef{}, nil
	}
	boxes := response.Configuration.General.StashBoxes
	out := make([]StashBoxRef, 0, len(boxes))
	for _, box := range boxes {
		if box == nil || box.Endpoint == "" {
			continue
		}
		out = append(out, StashBoxRef{Name: box.Name, Endpoint: box.Endpoint})
	}
	return out, nil
}

func (libraryService *Service) SubmitSceneToStashBoxVR(ctx context.Context, sceneID, endpoint string) error {
	if sceneID == "" || endpoint == "" {
		return fmt.Errorf("scene and Stash Box endpoint are required")
	}
	if _, err := gql.SceneSubmitStashBoxVR(ctx, libraryService.StashClient, &gql.StashBoxDraftSubmissionInput{
		Id:                 sceneID,
		Stash_box_endpoint: &endpoint,
	}); err != nil {
		return fmt.Errorf("submit scene draft to Stash Box: %w", err)
	}
	return nil
}

func (libraryService *Service) DeleteFiles(ctx context.Context, fileIDs []string) error {
	if len(fileIDs) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	if _, err := gql.SceneDeleteFilesVR(ctx, libraryService.StashClient, fileIDs); err != nil {
		return fmt.Errorf("SceneDeleteFilesVR: %w", err)
	}
	return nil
}

func (libraryService *Service) SetPrimaryFile(ctx context.Context, sceneID, fileID string) error {
	if _, err := gql.SceneSetPrimaryFileVR(ctx, libraryService.StashClient, sceneID, fileID); err != nil {
		return fmt.Errorf("SceneSetPrimaryFileVR: %w", err)
	}
	return nil
}

func (libraryService *Service) AssignFile(ctx context.Context, sceneID, fileID string) error {
	if _, err := gql.SceneAssignFileVR(ctx, libraryService.StashClient, &gql.AssignSceneFileInput{
		Scene_id: sceneID,
		File_id:  fileID,
	}); err != nil {
		return fmt.Errorf("SceneAssignFileVR: %w", err)
	}
	return nil
}

func (libraryService *Service) SplitFileToScene(ctx context.Context, fileID, title string) (string, error) {
	input := &gql.SceneCreateInput{File_ids: []string{fileID}}
	if title != "" {
		input.Title = &title
	}
	response, err := gql.SceneCreateFromFileVR(ctx, libraryService.StashClient, input)
	if err != nil {
		return "", fmt.Errorf("SceneCreateFromFileVR: %w", err)
	}
	if response == nil || response.SceneCreate == nil {
		return "", fmt.Errorf("SceneCreateFromFileVR returned no scene")
	}
	return response.SceneCreate.Id, nil
}

// SaveMarkerVR creates or edits a marker, resolving tag names through the
// same find-or-create behavior used by the existing HereSphere marker sync.
// The primary tag remains mandatory in Stash, while the remaining tags are
// optional metadata for the marker editor.
func (libraryService *Service) SaveMarkerVR(ctx context.Context, sceneID, markerID, title, primaryTag string, seconds float64, endSeconds *float64, tagNames []string) error {
	primaryTagID, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, primaryTag)
	if err != nil {
		return fmt.Errorf("find marker primary tag: %w", err)
	}
	tagIDs := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tagID, err := stash.FindOrCreateTag(ctx, libraryService.StashClient, name)
		if err != nil {
			return fmt.Errorf("find marker tag: %w", err)
		}
		tagIDs = append(tagIDs, tagID)
	}
	if markerID == "" {
		_, err = gql.SceneMarkerCreateVR(ctx, libraryService.StashClient, &gql.SceneMarkerCreateInput{
			Scene_id:       sceneID,
			Primary_tag_id: primaryTagID,
			Seconds:        seconds,
			End_seconds:    endSeconds,
			Tag_ids:        tagIDs,
			Title:          title,
		})
	} else {
		_, err = gql.SceneMarkerUpdateVR(ctx, libraryService.StashClient, &gql.SceneMarkerUpdateInput{
			Id:             markerID,
			Primary_tag_id: &primaryTagID,
			Seconds:        &seconds,
			End_seconds:    endSeconds,
			Tag_ids:        tagIDs,
			Title:          &title,
		})
	}
	if err != nil {
		return fmt.Errorf("save marker: %w", err)
	}
	return nil
}

func (libraryService *Service) DeleteMarkerVR(ctx context.Context, markerID string) error {
	if _, err := gql.SceneMarkerDeleteVR(ctx, libraryService.StashClient, markerID); err != nil {
		return fmt.Errorf("SceneMarkerDeleteVR: %w", err)
	}
	return nil
}

func (libraryService *Service) AddPlayAt(ctx context.Context, id string, at *time.Time) error {
	times := []time.Time(nil)
	if at != nil {
		times = []time.Time{*at}
	}
	if _, err := gql.SceneAddPlayVR(ctx, libraryService.StashClient, id, times); err != nil {
		return fmt.Errorf("SceneAddPlayVR: %w", err)
	}
	return nil
}

func (libraryService *Service) DeletePlayAt(ctx context.Context, id string, at *time.Time) error {
	times := []time.Time(nil)
	if at != nil {
		times = []time.Time{*at}
	}
	if _, err := gql.SceneDeletePlayVR(ctx, libraryService.StashClient, id, times); err != nil {
		return fmt.Errorf("SceneDeletePlayVR: %w", err)
	}
	return nil
}

func (libraryService *Service) ResetPlayHistory(ctx context.Context, id string) error {
	if _, err := gql.SceneResetPlayVR(ctx, libraryService.StashClient, id); err != nil {
		return fmt.Errorf("SceneResetPlayVR: %w", err)
	}
	return nil
}

func (libraryService *Service) AddOAt(ctx context.Context, id string, at *time.Time) error {
	times := []time.Time(nil)
	if at != nil {
		times = []time.Time{*at}
	}
	if _, err := gql.SceneAddOVR(ctx, libraryService.StashClient, id, times); err != nil {
		return fmt.Errorf("SceneAddOVR: %w", err)
	}
	return nil
}

func (libraryService *Service) DeleteOAt(ctx context.Context, id string, at *time.Time) error {
	times := []time.Time(nil)
	if at != nil {
		times = []time.Time{*at}
	}
	if _, err := gql.SceneDeleteOVR(ctx, libraryService.StashClient, id, times); err != nil {
		return fmt.Errorf("SceneDeleteOVR: %w", err)
	}
	return nil
}

func (libraryService *Service) ResetOHistory(ctx context.Context, id string) error {
	if _, err := gql.SceneResetOVR(ctx, libraryService.StashClient, id); err != nil {
		return fmt.Errorf("SceneResetOVR: %w", err)
	}
	return nil
}

func (libraryService *Service) ResetSceneActivity(ctx context.Context, id string, resetResume, resetDuration bool) error {
	if _, err := gql.SceneResetActivityVR(ctx, libraryService.StashClient, id, &resetResume, &resetDuration); err != nil {
		return fmt.Errorf("SceneResetActivityVR: %w", err)
	}
	return nil
}

func (libraryService *Service) GenerateScreenshot(ctx context.Context, id string, at *float64) error {
	if _, err := gql.SceneGenerateScreenshotVR(ctx, libraryService.StashClient, id, at); err != nil {
		return fmt.Errorf("SceneGenerateScreenshotVR: %w", err)
	}
	return nil
}

func (libraryService *Service) RescanScene(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("scene path is required")
	}
	rescan := true
	if _, err := gql.SceneRescanVR(ctx, libraryService.StashClient, &gql.ScanMetadataInput{
		Paths:  []string{path},
		Rescan: &rescan,
	}); err != nil {
		return fmt.Errorf("SceneRescanVR: %w", err)
	}
	return nil
}

// GenerateScene starts the same scoped Stash generation jobs exposed from
// the desktop scene Operations menu. Each job is restricted to one scene.
func (libraryService *Service) GenerateScene(ctx context.Context, id, kind string) error {
	input := &gql.GenerateMetadataInput{SceneIDs: []string{id}}
	on := true
	switch kind {
	case "covers":
		input.Covers = &on
	case "sprites":
		input.Sprites = &on
	case "previews":
		input.Previews = &on
	case "markers":
		input.Markers = &on
	case "transcodes":
		input.Transcodes = &on
	case "all":
		input.Covers = &on
		input.Sprites = &on
		input.Previews = &on
		input.Markers = &on
		input.Transcodes = &on
	default:
		return fmt.Errorf("unsupported generation kind %q", kind)
	}
	if _, err := gql.SceneGenerateVR(ctx, libraryService.StashClient, input); err != nil {
		return fmt.Errorf("SceneGenerateVR: %w", err)
	}
	return nil
}

func (libraryService *Service) MergeScene(ctx context.Context, sourceID, destinationID string) (string, error) {
	response, err := gql.SceneMergeVR(ctx, libraryService.StashClient, &gql.SceneMergeInput{
		Destination: destinationID,
		Source:      []string{sourceID},
	})
	if err != nil {
		return "", fmt.Errorf("SceneMergeVR: %w", err)
	}
	if response == nil || response.SceneMerge == nil {
		return "", fmt.Errorf("SceneMergeVR returned no scene")
	}
	return response.SceneMerge.Id, nil
}

func (libraryService *Service) RevealFile(ctx context.Context, fileID string) error {
	if _, err := gql.SceneRevealFileVR(ctx, libraryService.StashClient, fileID); err != nil {
		return fmt.Errorf("SceneRevealFileVR: %w", err)
	}
	return nil
}

func (libraryService *Service) AddPlayDuration(ctx context.Context, id string, duration time.Duration) error {
	seconds := duration.Seconds()
	_, err := gql.SceneAddPlayDurationSeconds(ctx, libraryService.StashClient, id, &seconds)
	if err != nil {
		return fmt.Errorf("SceneAddPlayDurationSeconds: %w", err)
	}
	return nil
}
