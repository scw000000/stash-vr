package browse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	apiinternal "stash-vr/internal/api/internal"
	"stash-vr/internal/prefix"
)

// projectionRequest is the JSON body shape of POST /browse/scene/{id}/projection.
// Either Auto is true (clears all projection tags), or Type/Degree/Stereo are
// set (writes the corresponding tags after dropping the existing ones).
type projectionRequest struct {
	Auto   bool   `json:"auto,omitempty"`
	Type   string `json:"type,omitempty"`   // "Normal" | "FishEye"
	Degree string `json:"degree,omitempty"` // "Cinema" | "180" | "200" | "360"
	Stereo string `json:"stereo,omitempty"` // "2D" | "SBS" | "TB"
}

// sceneProjectionHandler updates a scene's VR_* projection tags based on the
// in-VR picker selection. Drops the seven VR_* projection tags from the
// scene's current tag set, adds the tags returned by TagsForProjection for
// the new selection (or none on Auto), and persists via library.UpdateTags.
// Ancestor-injected tags are filtered out before computing the new set.
func (h *httpHandler) sceneProjectionHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req projectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var addTags []string
	if !req.Auto {
		proj, err := projectionFromRequest(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		addTags = apiinternal.TagsForProjection(proj)
	}

	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		log.Ctx(r.Context()).Warn().Err(err).Str("id", id).Msg("projection: scene not found")
		http.Error(w, "scene not found", http.StatusNotFound)
		return
	}

	projectionTags := map[string]bool{
		apiinternal.TagVR_DOME:    true,
		apiinternal.TagVR_SPHERE:  true,
		apiinternal.TagVR_FISHEYE: true,
		apiinternal.TagVR_MKX200:  true,
		apiinternal.TagVR_RF52:    true,
		apiinternal.TagVR_SBS:     true,
		apiinternal.TagVR_TB:      true,
	}
	newTags := make([]string, 0, len(vd.SceneParts.Tags)+len(addTags))
	for _, t := range vd.SceneParts.Tags {
		if t == nil {
			continue
		}
		// Skip ancestor-injected tags (they're persisted as ancestors of
		// other tags, not as direct tags on the scene).
		if strings.HasPrefix(t.TagParts.Sort_name, prefix.SvrAncestor) {
			continue
		}
		// Drop existing projection tags so we can replace cleanly.
		if projectionTags[t.TagParts.Name] {
			continue
		}
		newTags = append(newTags, t.TagParts.Name)
	}
	newTags = append(newTags, addTags...)

	if err := h.libraryService.UpdateTags(r.Context(), id, newTags); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("projection: UpdateTags failed")
		http.Error(w, "tag update failed", http.StatusInternalServerError)
		return
	}
	h.refreshSceneCache(r, id)
	w.WriteHeader(http.StatusNoContent)
}

// projectionFromRequest validates the three-field input and maps it to a
// Projection. Returns an error on invalid combinations (Normal+200,
// FishEye+Cinema, FishEye+360, or any unknown value).
func projectionFromRequest(req *projectionRequest) (apiinternal.Projection, error) {
	// Cinema means "no VR" — empty Projection regardless of Type/Stereo.
	if req.Degree == "Cinema" {
		return apiinternal.Projection{}, nil
	}

	proj := apiinternal.Projection{}
	switch req.Type {
	case "Normal":
		proj.Geometry = "equirectangular"
		switch req.Degree {
		case "180":
			proj.FOV = 180
		case "360":
			proj.FOV = 360
		default:
			return apiinternal.Projection{}, fmt.Errorf("invalid degree %q for type Normal", req.Degree)
		}
	case "FishEye":
		proj.Geometry = "fisheye"
		switch req.Degree {
		case "180":
			proj.FOV = 180
		case "200":
			proj.FOV = 200
		default:
			return apiinternal.Projection{}, fmt.Errorf("invalid degree %q for type FishEye", req.Degree)
		}
	default:
		return apiinternal.Projection{}, fmt.Errorf("invalid type %q", req.Type)
	}

	switch req.Stereo {
	case "2D", "":
		// Mono — leave proj.Stereo empty.
	case "SBS":
		proj.Stereo = "sbs"
	case "TB":
		proj.Stereo = "tb"
	default:
		return apiinternal.Projection{}, fmt.Errorf("invalid stereo %q", req.Stereo)
	}

	return proj, nil
}
