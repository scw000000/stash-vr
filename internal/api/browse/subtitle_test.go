package browse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stash-vr/internal/library"
	"stash-vr/internal/stash"
	"stash-vr/internal/subtitles"
)

func TestSubtitleRoutesListServeAndDeleteSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "scene.mp4")
	subtitlePath := filepath.Join(dir, "scene.jp.srt")
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatalf("write video: %v", err)
	}
	subtitleContent := "1\n00:00:00,000 --> 00:00:01,000\nhello\n"
	if err := os.WriteFile(subtitlePath, []byte(subtitleContent), 0o600); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	stashServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"findScenes": map[string]interface{}{
					"scenes": []map[string]interface{}{{
						"id":    "1",
						"title": "Test scene",
						"files": []map[string]interface{}{{
							"id":       "file-1",
							"basename": "scene.mp4",
							"path":     videoPath,
							"duration": 1,
						}},
					}},
				},
			},
		})
	}))
	defer stashServer.Close()

	libraryService := library.NewService(stash.NewClient(stashServer.URL, ""))
	subtitleService := subtitles.New(context.Background(), subtitles.Config{})
	defer subtitleService.Close()
	handler := Router(libraryService, subtitleService)

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/scene/1/subtitles", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET subtitles status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var state subtitles.State
	if err := json.NewDecoder(listResponse.Body).Decode(&state); err != nil {
		t.Fatalf("decode subtitle state: %v", err)
	}
	if len(state.Files) != 1 || state.Files[0].Name != "scene.jp.srt" {
		t.Fatalf("subtitle files = %#v, want scene.jp.srt", state.Files)
	}

	captionResponse := httptest.NewRecorder()
	captionURL := "/scene/1/caption?file=" + url.QueryEscape(state.Files[0].Key)
	handler.ServeHTTP(captionResponse, httptest.NewRequest(http.MethodGet, captionURL, nil))
	if captionResponse.Code != http.StatusOK || captionResponse.Body.String() != subtitleContent {
		t.Fatalf("GET caption status = %d, body = %q", captionResponse.Code, captionResponse.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	form := url.Values{"key": {state.Files[0].Key}}
	request := httptest.NewRequest(http.MethodPost, "/scene/1/subtitles/delete", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(deleteResponse, request)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("POST subtitle delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, err := os.Stat(subtitlePath); !os.IsNotExist(err) {
		t.Fatalf("subtitle still exists after route deletion: %v", err)
	}
}
