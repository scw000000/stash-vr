package browse

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash/gql"
)

// SceneMetaResponse is the JSON returned by GET /browse/scene/{id}/meta.
// Used by the M4c seamless scene swap to refresh playback-panel state
// (title, duration, caption tracks, scene markers) for the newly-loaded
// scene without exiting the WebXR session.
type SceneMetaResponse struct {
	Title        string        `json:"title"`
	DurationSec  float64       `json:"durationSec"`
	Captions     []CaptionRef  `json:"captions"`
	SceneMarkers []SceneMarker `json:"sceneMarkers"`

	// Detail fields feed the Stash-style scene panel in the in-VR browser.
	// Keep this response as the single source of metadata for both the
	// seamless scene swap and the detail panel.
	Description  string                 `json:"description"`
	Code         string                 `json:"code"`
	Director     string                 `json:"director"`
	URLs         []string               `json:"urls"`
	CustomFields map[string]interface{} `json:"customFields"`
	StashIDs     []StashIDRef           `json:"stashIDs"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
	Date         string                 `json:"date"`
	Rating1to5   int                    `json:"rating1to5"`
	PlayCount    int                    `json:"playCount"`
	OCounter     int                    `json:"oCounter"`
	IsFavorite   bool                   `json:"isFavorite"`
	Performers   []EntityRef            `json:"performers"`
	Studio       *EntityRef             `json:"studio"`
	Tags         []EntityRef            `json:"tags"`
	Galleries    []EntityRef            `json:"galleries"`
	Groups       []SceneGroupRef        `json:"groups"`
	Organized    bool                   `json:"organized"`
	ResumeTime   *float64               `json:"resumeTime,omitempty"`
	PlayDuration *float64               `json:"playDuration,omitempty"`
	LastPlayedAt *time.Time             `json:"lastPlayedAt,omitempty"`
	PlayHistory  []time.Time            `json:"playHistory"`
	OHistory     []time.Time            `json:"oHistory"`
	Files        []SceneFile            `json:"files"`
}

// SceneFile carries the media details shown by the File Info VR tab.
type SceneFile struct {
	ID           string            `json:"id"`
	Basename     string            `json:"basename"`
	Path         string            `json:"path"`
	Duration     float64           `json:"duration"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Size         int64             `json:"size"`
	BitRate      int               `json:"bitRate"`
	Format       string            `json:"format"`
	FrameRate    float64           `json:"frameRate"`
	AudioCodec   string            `json:"audioCodec"`
	VideoCodec   string            `json:"videoCodec"`
	ModifiedAt   string            `json:"modifiedAt"`
	Fingerprints []FileFingerprint `json:"fingerprints"`
}

// FileFingerprint is the desktop File Info panel's checksum data.
type FileFingerprint struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type StashIDRef struct {
	Endpoint string `json:"endpoint"`
	StashID  string `json:"stashID"`
}

func (h *httpHandler) sceneMetaHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vd, err := h.libraryService.GetScene(r.Context(), id, false)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("browse: meta scene not found")
		http.NotFound(w, r)
		return
	}
	sp := vd.SceneParts
	out := SceneMetaResponse{
		Title:        vd.Title(),
		URLs:         append([]string(nil), sp.Urls...),
		CustomFields: sp.Custom_fields,
		CreatedAt:    sp.Created_at.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    sp.Updated_at.Format("2006-01-02T15:04:05Z07:00"),
		Organized:    sp.Organized,
	}
	// Playback swaps consume this endpoint too; history is only needed by
	// the on-demand VR detail panel, so do not add an expensive activity read
	// to every scene switch.
	if r.URL.Query().Get("detail") == "1" {
		activity, activityErr := gql.FindSceneActivity(r.Context(), h.libraryService.StashClient, id)
		if activityErr != nil {
			log.Ctx(r.Context()).Warn().Err(activityErr).Str("id", id).Msg("browse: read scene activity")
		} else if activity != nil && activity.FindScene != nil {
			out.ResumeTime = activity.FindScene.Resume_time
			out.PlayDuration = activity.FindScene.Play_duration
			out.LastPlayedAt = activity.FindScene.Last_played_at
			out.PlayHistory = activity.FindScene.Play_history
			out.OHistory = activity.FindScene.O_history
		}
	}
	if sp.Details != nil {
		out.Description = *sp.Details
	}
	if sp.Code != nil {
		out.Code = *sp.Code
	}
	if sp.Director != nil {
		out.Director = *sp.Director
	}
	if sp.Date != nil {
		out.Date = *sp.Date
	}
	if sp.Rating100 != nil {
		out.Rating1to5 = *sp.Rating100 / 20
	}
	if sp.Play_count != nil {
		out.PlayCount = *sp.Play_count
	}
	if sp.O_counter != nil {
		out.OCounter = *sp.O_counter
	}
	for _, p := range sp.Performers {
		if p == nil {
			continue
		}
		out.Performers = append(out.Performers, EntityRef{ID: p.Id, Name: p.Name})
	}
	if sp.Studio != nil {
		out.Studio = &EntityRef{ID: sp.Studio.Id, Name: sp.Studio.Name}
	}
	out.Tags, out.IsFavorite = projectTags(sp)
	for _, file := range sp.Files {
		if file == nil {
			continue
		}
		out.Files = append(out.Files, SceneFile{
			ID: file.Id, Basename: file.Basename, Path: file.Path, Duration: file.Duration,
			Width: file.Width, Height: file.Height, Size: file.Size,
			BitRate: file.Bit_rate, Format: file.Format, FrameRate: file.Frame_rate,
			AudioCodec: file.Audio_codec, VideoCodec: file.Video_codec,
			ModifiedAt: file.Mod_time.Format(time.RFC3339),
		})
		for _, fingerprint := range file.Fingerprints {
			if fingerprint == nil {
				continue
			}
			fileIndex := len(out.Files) - 1
			out.Files[fileIndex].Fingerprints = append(out.Files[fileIndex].Fingerprints, FileFingerprint{
				Type: fingerprint.Type, Value: fingerprint.Value,
			})
		}
	}
	if len(out.Files) > 0 {
		out.DurationSec = out.Files[0].Duration
	}
	out.Captions = make([]CaptionRef, 0, len(sp.Captions))
	for _, c := range sp.Captions {
		if c == nil {
			continue
		}
		out.Captions = append(out.Captions, CaptionRef{
			LanguageCode: c.Language_code,
			CaptionType:  c.Caption_type,
		})
	}
	out.SceneMarkers = make([]SceneMarker, 0, len(sp.Scene_markers))
	for _, m := range sp.Scene_markers {
		if m == nil {
			continue
		}
		marker := SceneMarker{ID: m.Id, Seconds: m.Seconds, EndSeconds: m.End_seconds, Title: m.Title}
		if m.Primary_tag != nil {
			marker.PrimaryTag = &EntityRef{ID: m.Primary_tag.Id, Name: m.Primary_tag.Name}
		}
		for _, tag := range m.Tags {
			if tag != nil {
				marker.Tags = append(marker.Tags, EntityRef{ID: tag.Id, Name: tag.Name})
			}
		}
		out.SceneMarkers = append(out.SceneMarkers, marker)
	}
	for _, gallery := range sp.Galleries {
		if gallery != nil {
			name := ""
			if gallery.Title != nil {
				name = *gallery.Title
			}
			out.Galleries = append(out.Galleries, EntityRef{ID: gallery.Id, Name: name})
		}
	}
	for _, group := range sp.Groups {
		if group == nil || group.Group == nil {
			continue
		}
		out.Groups = append(out.Groups, SceneGroupRef{
			ID: group.Group.Id, Name: group.Group.Name, SceneIndex: group.Scene_index,
		})
	}
	for _, stashID := range sp.Stash_ids {
		if stashID != nil {
			out.StashIDs = append(out.StashIDs, StashIDRef{Endpoint: stashID.Endpoint, StashID: stashID.Stash_id})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: encode scene meta")
	}
}
