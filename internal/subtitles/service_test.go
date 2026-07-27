package subtitles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"stash-vr/internal/stash"
)

func TestRelatedFilesOnlyReturnsVideoSidecars(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	video := filepath.Join(dir, "scene.one.mp4")
	mustWriteTestFile(t, video, "video")
	mustWriteTestFile(t, filepath.Join(dir, "scene.one.srt"), "translated")
	mustWriteTestFile(t, filepath.Join(dir, "scene.one.jp.srt"), "original")
	mustWriteTestFile(t, filepath.Join(dir, "scene.one.en.srt"), "english")
	mustWriteTestFile(t, filepath.Join(dir, "scene.srt"), "different video")
	mustWriteTestFile(t, filepath.Join(dir, "scene.one.vtt"), "wrong format")

	files := RelatedFiles([]string{video})
	if len(files) != 3 {
		t.Fatalf("RelatedFiles() returned %d files, want 3: %#v", len(files), files)
	}
	wantNames := []string{"scene.one.en.srt", "scene.one.jp.srt", "scene.one.srt"}
	for index, want := range wantNames {
		if files[index].Name != want {
			t.Errorf("files[%d].Name = %q, want %q", index, files[index].Name, want)
		}
		if files[index].Key == "" {
			t.Errorf("files[%d].Key is empty", index)
		}
	}

	file, err := OpenFile([]string{video}, files[0].Key)
	if err != nil {
		t.Fatalf("OpenFile(): %v", err)
	}
	_ = file.Close()
}

func TestDeleteFileRejectsUnrelatedPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	video := filepath.Join(dir, "scene.mp4")
	subtitle := filepath.Join(dir, "scene.jp.srt")
	unrelated := filepath.Join(dir, "other.srt")
	mustWriteTestFile(t, video, "video")
	mustWriteTestFile(t, subtitle, "subtitle")
	mustWriteTestFile(t, unrelated, "keep")

	service := New(context.Background(), Config{})
	defer service.Close()

	files := RelatedFiles([]string{video})
	if len(files) != 1 {
		t.Fatalf("RelatedFiles() returned %d files, want 1", len(files))
	}
	if err := service.DeleteFile("42", []string{video}, files[0].Key); err != nil {
		t.Fatalf("DeleteFile(related): %v", err)
	}
	if _, err := os.Stat(subtitle); !os.IsNotExist(err) {
		t.Fatalf("related subtitle still exists after delete: %v", err)
	}
	if err := service.DeleteFile("42", []string{video}, encodeFileKey(0, "other.srt")); err == nil {
		t.Fatal("DeleteFile(unrelated) succeeded, want error")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated subtitle was changed: %v", err)
	}
}

func TestNormalizeOptionsRejectsJapaneseOnlyServiceForMandarin(t *testing.T) {
	t.Parallel()

	_, err := normalizeOptions(Options{
		SourceLanguage:       "zh",
		Mode:                 "transcribe_only",
		TranscriptionService: "reazonspeech",
	})
	if err == nil {
		t.Fatal("normalizeOptions() succeeded, want Japanese-only validation error")
	}
}

func TestResolveConfiguredPathFallsBackToExecutableDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	launchDirectory := filepath.Join(root, "launch")
	executableDirectory := filepath.Join(root, "dev", "stash-vr")
	generatorDirectory := filepath.Join(root, "dev", "python_tools", "CaptionGenerator")
	for _, dir := range []string{launchDirectory, executableDirectory, generatorDirectory} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create test directory: %v", err)
		}
	}
	mustWriteTestFile(t, filepath.Join(generatorDirectory, "video_caption_generator.py"), "# fake")
	mustWriteTestFile(t, filepath.Join(executableDirectory, ".env"), "TEST=true")

	bases := []string{launchDirectory, executableDirectory}
	if got := resolveConfiguredPathFrom("../python_tools/CaptionGenerator", "video_caption_generator.py", bases); got != generatorDirectory {
		t.Fatalf("generator path = %q, want %q", got, generatorDirectory)
	}
	if got := resolveConfiguredPathFrom(".env", "", bases); got != filepath.Join(executableDirectory, ".env") {
		t.Fatalf("dotenv path = %q, want executable-relative .env", got)
	}

	missing := filepath.Join(root, "Downloads", "CaptionGenerator")
	if got := preferAvailablePath(missing, "video_caption_generator.py", generatorDirectory); got != generatorDirectory {
		t.Fatalf("workspace fallback = %q, want %q", got, generatorDirectory)
	}
}

func TestJobRunnerProcessesOnlySelectedVideo(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable")
	}

	dir := t.TempDir()
	generatorDir := filepath.Join(dir, "CaptionGenerator")
	if err := os.Mkdir(generatorDir, 0o700); err != nil {
		t.Fatalf("create fake CaptionGenerator: %v", err)
	}
	fakeGenerator := `
from pathlib import Path

load_dotenv = None
GEMINI_AVAILABLE = True
ASSEMBLYAI_AVAILABLE = True
GRANITE_AVAILABLE = True
QWEN3_ASR_AVAILABLE = True
KOTOBA_AVAILABLE = True
REAZONSPEECH_AVAILABLE = True

async def transcribe_video(video_path, service, client, model, language, pbar):
    pbar.n = 60
    return [{"start": 0.0, "end": 1.0, "text": Path(video_path).stem}]

def clean_transcription_segments(segments):
    return segments

async def generate_srt_content(**kwargs):
    return "1\n00:00:00,000 --> 00:00:01,000\nselected\n"

def parse_srt_file(path):
    return []
`
	mustWriteTestFile(t, filepath.Join(generatorDir, "video_caption_generator.py"), fakeGenerator)
	selected := filepath.Join(dir, "selected.mp4")
	other := filepath.Join(dir, "other.mp4")
	mustWriteTestFile(t, selected, "video")
	mustWriteTestFile(t, other, "video")

	service := New(context.Background(), Config{
		PythonExecutable: python,
		GeneratorPath:    generatorDir,
	})
	defer service.Close()
	job, err := service.Start(context.Background(), "scene-1", selected, Options{
		SourceLanguage:       "ja",
		Mode:                 "transcribe_only",
		TranscriptionService: "qwen3",
	})
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		job = service.Job("scene-1")
		if job != nil && !isActive(job.Status) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if job == nil || job.Status != StatusSucceeded {
		t.Fatalf("job = %#v, want succeeded", job)
	}
	if _, err := os.Stat(filepath.Join(dir, "selected.jp.srt")); err != nil {
		t.Fatalf("selected subtitle was not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other.jp.srt")); !os.IsNotExist(err) {
		t.Fatalf("unselected video received a subtitle: %v", err)
	}
}

func TestStashTaskIsInstalledStartedAndPolled(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable")
	}

	dir := t.TempDir()
	pluginsPath := filepath.Join(dir, "plugins")
	generatorDir := filepath.Join(dir, "CaptionGenerator")
	if err := os.Mkdir(generatorDir, 0o700); err != nil {
		t.Fatalf("create fake CaptionGenerator: %v", err)
	}
	mustWriteTestFile(t, filepath.Join(generatorDir, "video_caption_generator.py"), "# fake")
	video := filepath.Join(dir, "selected.mp4")
	mustWriteTestFile(t, video, "video")

	var mu sync.Mutex
	reloaded := false
	startedArgs := map[string]interface{}{}
	stashServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch request.OperationName {
		case "SubtitlePluginConfiguration":
			mu.Lock()
			pluginReloaded := reloaded
			mu.Unlock()
			plugins := []map[string]interface{}{}
			if pluginReloaded {
				plugins = append(plugins, map[string]interface{}{
					"id":      stashPluginID,
					"enabled": true,
					"tasks":   []map[string]interface{}{{"name": stashPluginTaskName}},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{
				"configuration": map[string]interface{}{
					"general": map[string]interface{}{"pluginsPath": pluginsPath},
				},
				"plugins": plugins,
			}})
		case "ReloadSubtitlePlugins":
			mu.Lock()
			reloaded = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{
				"reloadPlugins": true,
			}})
		case "RunSubtitlePluginTask":
			mu.Lock()
			if args, ok := request.Variables["args"].(map[string]interface{}); ok {
				startedArgs = args
			}
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{
				"runPluginTask": "stash-job-12",
			}})
		case "FindSubtitleJob":
			now := time.Now().UTC().Format(time.RFC3339Nano)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{
				"findJob": map[string]interface{}{
					"id":          "stash-job-12",
					"description": "Generate subtitles",
					"status":      "FINISHED",
					"progress":    1,
					"addTime":     now,
					"startTime":   now,
					"endTime":     now,
					"error":       nil,
				},
			}})
		default:
			http.Error(w, "unexpected operation "+request.OperationName, http.StatusBadRequest)
		}
	}))
	defer stashServer.Close()

	service := New(context.Background(), Config{
		PythonExecutable: python,
		GeneratorPath:    generatorDir,
		StashClient:      stash.NewClient(stashServer.URL, ""),
	})
	defer service.Close()

	job, err := service.Start(context.Background(), "scene-1", video, Options{
		SourceLanguage:       "ja",
		Mode:                 "transcribe_only",
		TranscriptionService: "qwen3",
	})
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if job.ID != "stash-job-12" {
		t.Fatalf("job ID = %q, want Stash job ID", job.ID)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job = service.Job("scene-1")
		if job != nil && !isActive(job.Status) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job == nil || job.Status != StatusSucceeded || job.Progress != 100 {
		t.Fatalf("job = %#v, want completed Stash task", job)
	}

	if _, err := os.Stat(filepath.Join(pluginsPath, stashPluginID+".yml")); err != nil {
		t.Fatalf("plugin config was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginsPath, stashPluginID, "caption_job.py")); err != nil {
		t.Fatalf("plugin runner was not installed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if startedArgs["video"] != video || startedArgs["sceneID"] != "scene-1" {
		t.Fatalf("Stash task args = %#v, want selected video and scene", startedArgs)
	}
}

func TestCaptionRunnerSpeaksStashRawPluginProtocol(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable")
	}

	runner := filepath.Join(t.TempDir(), "caption_job.py")
	if err := os.WriteFile(runner, captionJobScript, 0o600); err != nil {
		t.Fatalf("write caption runner: %v", err)
	}
	stashDirectory := t.TempDir()
	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName += ".exe"
	}
	ffmpegPath := filepath.Join(stashDirectory, ffmpegName)
	if err := os.WriteFile(ffmpegPath, []byte("fake ffmpeg"), 0o700); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	input := map[string]interface{}{
		"server_connection": map[string]interface{}{"Dir": stashDirectory},
		"args": map[string]interface{}{
			"generatorPath":        filepath.Join(t.TempDir(), "missing-generator"),
			"video":                filepath.Join(t.TempDir(), "missing-video.mp4"),
			"sourceLanguage":       "ja",
			"mode":                 "transcribe_only",
			"transcriptionService": "qwen3",
			"transcriptionModel":   "models/gemini-3.1-pro-preview",
			"translationService":   "openai",
			"translationModel":     "gpt-4o-mini",
		}}
	encodedInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("encode plugin input: %v", err)
	}

	cmd := exec.Command(python, runner, "--stash-plugin")
	cmd.Stdin = bytes.NewReader(encodedInput)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("plugin runner succeeded for invalid input, want a failed Stash task")
	} else if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("plugin runner could not execute: %v; stderr=%q", err, stderr.String())
	}

	var output map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("plugin stdout is not a single JSON result: %v; stdout=%q", err, stdout.String())
	}
	if strings.TrimSpace(fmt.Sprint(output["error"])) == "" {
		t.Fatalf("plugin output = %#v, want a structured error", output)
	}
	if !bytes.Contains(stderr.Bytes(), []byte{1, 'p', 2}) {
		t.Fatalf("plugin stderr did not contain a Stash progress prefix: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Using FFmpeg:") {
		t.Fatalf("plugin did not discover FFmpeg from the Stash directory: %q", stderr.String())
	}
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
