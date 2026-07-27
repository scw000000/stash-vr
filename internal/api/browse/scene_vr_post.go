package browse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/library"
	"stash-vr/internal/prefix"
	"stash-vr/internal/stash/gql"
)

// sceneVRUpdateHandler backs the editable fields in the VR version of
// Stash's Scene Edit tab. The browser submits one field at a time so a
// headset keyboard never risks overwriting unrelated desktop edits.
func (h *httpHandler) sceneVRUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	field := strings.TrimSpace(r.PostForm.Get("field"))
	value := r.PostForm.Get("value")
	var updateErr error
	switch field {
	case "urls":
		updateErr = h.libraryService.UpdateSceneVRURLs(r.Context(), id, splitSceneURLs(value))
	case "customFields":
		fields := map[string]interface{}{}
		if strings.TrimSpace(value) != "" {
			if err := json.Unmarshal([]byte(value), &fields); err != nil {
				writeErr(w, http.StatusBadRequest, "custom fields must be JSON")
				return
			}
		}
		updateErr = h.libraryService.UpdateSceneVRCustomFields(r.Context(), id, fields)
	case "title", "code", "date", "director", "details", "coverImage":
		updateErr = h.libraryService.UpdateSceneVRField(r.Context(), id, field, value)
	default:
		writeErr(w, http.StatusBadRequest, "unsupported edit field")
		return
	}
	if updateErr != nil {
		log.Ctx(r.Context()).Err(updateErr).Str("id", id).Str("field", field).Msg("browse: update VR scene field")
		writeErr(w, http.StatusInternalServerError, "scene update failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func splitSceneURLs(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// sceneVRRelationsHandler provides the desktop editor's add/remove/set
// behavior for relations. IDs are selected in the VR panel, then the server
// reads the latest scene before applying the mutation to avoid clobbering a
// concurrent Stash desktop edit.
func (h *httpHandler) sceneVRRelationsHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	kind := strings.TrimSpace(r.PostForm.Get("kind"))
	action := strings.TrimSpace(r.PostForm.Get("action"))
	entityID := strings.TrimSpace(r.PostForm.Get("entityID"))
	if entityID == "" && !(kind == "studio" && action == "clear") {
		writeErr(w, http.StatusBadRequest, "missing entity")
		return
	}
	vd, err := h.libraryService.GetScene(r.Context(), id, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		writeErr(w, http.StatusNotFound, "scene not found")
		return
	}
	parts := vd.SceneParts
	var updateErr error
	switch kind {
	case "tag":
		ids := make([]string, 0, len(parts.Tags)+1)
		for _, tag := range parts.Tags {
			if tag == nil || strings.HasPrefix(tag.TagParts.Sort_name, prefix.SvrAncestor) {
				continue
			}
			if action == "remove" && tag.Id == entityID {
				continue
			}
			ids = append(ids, tag.Id)
		}
		if action == "add" && !containsString(ids, entityID) {
			ids = append(ids, entityID)
		}
		updateErr = h.libraryService.UpdateSceneVRTags(r.Context(), id, ids)
	case "performer":
		ids := make([]string, 0, len(parts.Performers)+1)
		for _, performer := range parts.Performers {
			if performer == nil || (action == "remove" && performer.Id == entityID) {
				continue
			}
			ids = append(ids, performer.Id)
		}
		if action == "add" && !containsString(ids, entityID) {
			ids = append(ids, entityID)
		}
		updateErr = h.libraryService.UpdateSceneVRPerformers(r.Context(), id, ids)
	case "studio":
		if action == "clear" {
			updateErr = h.libraryService.SetSceneVRStudio(r.Context(), id, nil)
		} else if action == "set" {
			updateErr = h.libraryService.SetSceneVRStudio(r.Context(), id, &entityID)
		} else {
			writeErr(w, http.StatusBadRequest, "unsupported studio action")
			return
		}
	case "gallery":
		ids := make([]string, 0, len(parts.Galleries)+1)
		for _, gallery := range parts.Galleries {
			if gallery == nil || (action == "remove" && gallery.Id == entityID) {
				continue
			}
			ids = append(ids, gallery.Id)
		}
		if action == "add" && !containsString(ids, entityID) {
			ids = append(ids, entityID)
		}
		updateErr = h.libraryService.UpdateSceneVRGalleries(r.Context(), id, ids)
	case "group":
		groups := make([]*gql.SceneGroupInput, 0, len(parts.Groups)+1)
		for _, group := range parts.Groups {
			if group == nil || group.Group == nil || (action == "remove" && group.Group.Id == entityID) {
				continue
			}
			groups = append(groups, &gql.SceneGroupInput{Group_id: group.Group.Id, Scene_index: group.Scene_index})
		}
		if action == "add" && !containsSceneGroup(groups, entityID) {
			groups = append(groups, &gql.SceneGroupInput{Group_id: entityID})
		}
		updateErr = h.libraryService.UpdateSceneVRGroups(r.Context(), id, groups)
	default:
		writeErr(w, http.StatusBadRequest, "unsupported relation")
		return
	}
	if updateErr != nil {
		log.Ctx(r.Context()).Err(updateErr).Str("id", id).Str("kind", kind).Msg("browse: update VR scene relation")
		writeErr(w, http.StatusInternalServerError, "scene relation update failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func containsSceneGroup(items []*gql.SceneGroupInput, candidate string) bool {
	for _, item := range items {
		if item != nil && item.Group_id == candidate {
			return true
		}
	}
	return false
}

func (h *httpHandler) sceneVREditOptionsHandler(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var query *string
	if q != "" {
		query = &q
	}
	var out []EntityRef
	switch kind {
	case "group":
		response, err := gql.FindGroupsForSceneEditor(r.Context(), h.libraryService.StashClient, query)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "load groups failed")
			return
		}
		for _, group := range response.FindGroups.Groups {
			if group != nil {
				out = append(out, EntityRef{ID: group.Id, Name: group.Name})
			}
		}
	case "gallery":
		response, err := gql.FindGalleriesForSceneEditor(r.Context(), h.libraryService.StashClient, query)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "load galleries failed")
			return
		}
		for _, gallery := range response.FindGalleries.Galleries {
			if gallery != nil {
				name := ""
				if gallery.Title != nil {
					name = *gallery.Title
				}
				out = append(out, EntityRef{ID: gallery.Id, Name: name})
			}
		}
	default:
		h.filterOptionsHandler(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// sceneVRStashBoxesHandler exposes only the display-safe Stash Box fields
// needed by the in-headset submission chooser; it never forwards API keys.
func (h *httpHandler) sceneVRStashBoxesHandler(w http.ResponseWriter, r *http.Request) {
	boxes, err := h.libraryService.SceneStashBoxesVR(r.Context())
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("browse: load VR Stash Boxes")
		writeErr(w, http.StatusInternalServerError, "load Stash Boxes failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(boxes)
}

func (h *httpHandler) sceneVRFileActionHandler(w http.ResponseWriter, r *http.Request) {
	sceneID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "fileID")
	action := chi.URLParam(r, "action")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	if _, err := h.findSceneFile(r, sceneID, fileID); err != nil {
		writeErr(w, http.StatusNotFound, "file not found on scene")
		return
	}
	var err error
	switch action {
	case "primary":
		err = h.libraryService.SetPrimaryFile(r.Context(), sceneID, fileID)
	case "reveal":
		err = h.libraryService.RevealFile(r.Context(), fileID)
	case "delete":
		err = h.libraryService.DeleteFiles(r.Context(), []string{fileID})
	case "reassign":
		targetSceneID := strings.TrimSpace(r.PostForm.Get("targetSceneID"))
		if targetSceneID == "" || targetSceneID == sceneID {
			writeErr(w, http.StatusBadRequest, "choose another target scene")
			return
		}
		err = h.libraryService.AssignFile(r.Context(), targetSceneID, fileID)
	case "split":
		newID, splitErr := h.libraryService.SplitFileToScene(r.Context(), fileID, strings.TrimSpace(r.PostForm.Get("title")))
		if splitErr != nil {
			err = splitErr
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"sceneID": newID})
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, "unsupported file action")
		return
	}
	if err != nil {
		log.Ctx(r.Context()).Err(err).Str("sceneID", sceneID).Str("fileID", fileID).Str("action", action).Msg("browse: VR file action")
		writeErr(w, http.StatusInternalServerError, "file action failed")
		return
	}
	h.refreshSceneCache(r, sceneID)
	h.writeState(w, r, sceneID)
}

func (h *httpHandler) findSceneFile(r *http.Request, sceneID, fileID string) (*gql.ScenePartsFilesVideoFile, error) {
	vd, err := h.libraryService.GetScene(r.Context(), sceneID, true)
	if err != nil || vd == nil || vd.SceneParts == nil {
		return nil, fmt.Errorf("scene not found")
	}
	for _, file := range vd.SceneParts.Files {
		if file != nil && file.Id == fileID {
			return file, nil
		}
	}
	return nil, fmt.Errorf("file not on scene")
}

func (h *httpHandler) sceneVRMarkerHandler(w http.ResponseWriter, r *http.Request) {
	sceneID := chi.URLParam(r, "id")
	markerID := chi.URLParam(r, "markerID")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	if r.Method == http.MethodDelete || r.PostForm.Get("action") == "delete" || strings.HasSuffix(r.URL.Path, "/delete") {
		if markerID == "" {
			writeErr(w, http.StatusBadRequest, "marker id required")
			return
		}
		if err := h.libraryService.DeleteMarkerVR(r.Context(), markerID); err != nil {
			writeErr(w, http.StatusInternalServerError, "delete marker failed")
			return
		}
	} else {
		seconds, err := strconv.ParseFloat(r.PostForm.Get("seconds"), 64)
		if err != nil || seconds < 0 {
			writeErr(w, http.StatusBadRequest, "invalid marker start")
			return
		}
		var endSeconds *float64
		if rawEnd := strings.TrimSpace(r.PostForm.Get("endSeconds")); rawEnd != "" {
			end, parseErr := strconv.ParseFloat(rawEnd, 64)
			if parseErr != nil || end < seconds {
				writeErr(w, http.StatusBadRequest, "invalid marker end")
				return
			}
			endSeconds = &end
		}
		primaryTag := strings.TrimSpace(r.PostForm.Get("primaryTag"))
		if primaryTag == "" {
			writeErr(w, http.StatusBadRequest, "marker primary tag is required")
			return
		}
		tags := splitSceneURLs(r.PostForm.Get("tags"))
		if err := h.libraryService.SaveMarkerVR(r.Context(), sceneID, markerID,
			strings.TrimSpace(r.PostForm.Get("title")), primaryTag, seconds, endSeconds, tags); err != nil {
			log.Ctx(r.Context()).Err(err).Str("id", sceneID).Msg("browse: save VR marker")
			writeErr(w, http.StatusInternalServerError, "save marker failed")
			return
		}
	}
	h.refreshSceneCache(r, sceneID)
	h.writeState(w, r, sceneID)
}

func (h *httpHandler) sceneVRHistoryHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	kind := chi.URLParam(r, "kind")
	action := chi.URLParam(r, "action")
	at, err := parseOptionalVRTime(r.PostForm.Get("time"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid timestamp")
		return
	}
	switch kind {
	case "play":
		switch action {
		case "add":
			err = h.libraryService.AddPlayAt(r.Context(), id, at)
		case "remove":
			err = h.libraryService.DeletePlayAt(r.Context(), id, at)
		case "clear":
			err = h.libraryService.ResetPlayHistory(r.Context(), id)
		case "reset-resume":
			err = h.libraryService.ResetSceneActivity(r.Context(), id, true, false)
		case "reset-duration":
			err = h.libraryService.ResetSceneActivity(r.Context(), id, false, true)
		}
	case "o":
		switch action {
		case "add":
			err = h.libraryService.AddOAt(r.Context(), id, at)
		case "remove":
			err = h.libraryService.DeleteOAt(r.Context(), id, at)
		case "clear":
			err = h.libraryService.ResetOHistory(r.Context(), id)
		}
	default:
		writeErr(w, http.StatusBadRequest, "unsupported history")
		return
	}
	if err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("kind", kind).Str("action", action).Msg("browse: VR history action")
		writeErr(w, http.StatusInternalServerError, "history action failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func parseOptionalVRTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("unsupported time")
}

func (h *httpHandler) sceneVRDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.libraryService.Delete(r.Context(), id); err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Msg("browse: delete VR scene")
		writeErr(w, http.StatusInternalServerError, "delete scene failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}

// sceneVROperationHandler mirrors the desktop scene Operations menu. Jobs are
// deliberately scoped to the selected scene so invoking them from a headset
// cannot unexpectedly process the full library.
func (h *httpHandler) sceneVROperationHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	operation := chi.URLParam(r, "operation")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "bad form")
		return
	}
	var err error
	switch operation {
	case "rescan":
		vd, fetchErr := h.libraryService.GetScene(r.Context(), id, true)
		if fetchErr != nil || vd == nil || vd.SceneParts == nil || len(vd.SceneParts.Files) == 0 || vd.SceneParts.Files[0] == nil {
			writeErr(w, http.StatusNotFound, "scene file not found")
			return
		}
		err = h.libraryService.RescanScene(r.Context(), vd.SceneParts.Files[0].Path)
	case "screenshot-current":
		at, parseErr := strconv.ParseFloat(r.PostForm.Get("seconds"), 64)
		if parseErr != nil || at < 0 {
			writeErr(w, http.StatusBadRequest, "invalid video position")
			return
		}
		err = h.libraryService.GenerateScreenshot(r.Context(), id, &at)
	case "screenshot-default":
		err = h.libraryService.GenerateScreenshot(r.Context(), id, nil)
	case "generate":
		err = h.libraryService.GenerateScene(r.Context(), id, strings.TrimSpace(r.PostForm.Get("kind")))
	case "merge":
		destination := strings.TrimSpace(r.PostForm.Get("destinationID"))
		if destination == "" || destination == id {
			writeErr(w, http.StatusBadRequest, "choose another merge destination")
			return
		}
		var destinationID string
		destinationID, err = h.libraryService.MergeScene(r.Context(), id, destination)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"sceneID": destinationID})
			return
		}
	case "submit-stash-box":
		endpoint := strings.TrimSpace(r.PostForm.Get("endpoint"))
		boxes, boxErr := h.libraryService.SceneStashBoxesVR(r.Context())
		if boxErr != nil {
			log.Ctx(r.Context()).Err(boxErr).Msg("browse: verify VR Stash Box")
			writeErr(w, http.StatusInternalServerError, "load Stash Boxes failed")
			return
		}
		if !hasStashBoxEndpoint(boxes, endpoint) {
			writeErr(w, http.StatusBadRequest, "unknown Stash Box")
			return
		}
		err = h.libraryService.SubmitSceneToStashBoxVR(r.Context(), id, endpoint)
	default:
		writeErr(w, http.StatusBadRequest, "unsupported scene operation")
		return
	}
	if err != nil {
		log.Ctx(r.Context()).Err(err).Str("id", id).Str("operation", operation).Msg("browse: VR scene operation")
		writeErr(w, http.StatusInternalServerError, "scene operation failed")
		return
	}
	h.refreshSceneCache(r, id)
	h.writeState(w, r, id)
}

func hasStashBoxEndpoint(boxes []library.StashBoxRef, endpoint string) bool {
	for _, box := range boxes {
		if box.Endpoint == endpoint {
			return true
		}
	}
	return false
}
