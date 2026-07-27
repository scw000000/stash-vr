package subtitles

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/rs/zerolog/log"
	"stash-vr/internal/stash/gql"
)

//go:embed caption_job.py
var captionJobScript []byte

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"

	stashPluginID       = "stash_vr_caption"
	stashPluginTaskName = "Generate subtitle"
)

type Config struct {
	PythonExecutable string
	GeneratorPath    string
	EnvFile          string
	StashClient      graphql.Client
}

type RuntimeStatus struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

type Options struct {
	SourceLanguage       string `json:"sourceLanguage"`
	Mode                 string `json:"mode"`
	TranscriptionService string `json:"transcriptionService"`
	TranscriptionModel   string `json:"transcriptionModel"`
	TranslationService   string `json:"translationService"`
	TranslationModel     string `json:"translationModel"`
}

type File struct {
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	SourceBasename string    `json:"sourceBasename"`
	Size           int64     `json:"size"`
	ModifiedAt     time.Time `json:"modifiedAt"`
}

type Job struct {
	ID         string     `json:"id"`
	SceneID    string     `json:"sceneId"`
	Status     string     `json:"status"`
	Progress   float64    `json:"progress"`
	Stage      string     `json:"stage"`
	Error      string     `json:"error,omitempty"`
	Options    Options    `json:"options"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

type State struct {
	Runtime RuntimeStatus `json:"runtime"`
	Files   []File        `json:"files"`
	Job     *Job          `json:"job,omitempty"`
	Err     string        `json:"err,omitempty"`
}

type jobRecord struct {
	job    Job
	cancel context.CancelFunc
}

type Service struct {
	cfg         Config
	stashClient graphql.Client
	ctx         context.Context
	cancel      context.CancelFunc

	startMu sync.Mutex
	mu      sync.RWMutex
	jobs    map[string]*jobRecord

	pluginMu    sync.Mutex
	pluginReady bool
}

func New(parent context.Context, cfg Config) *Service {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	cfg.PythonExecutable = strings.TrimSpace(cfg.PythonExecutable)
	if cfg.PythonExecutable == "" {
		cfg.PythonExecutable = "python"
	}
	if executable, err := exec.LookPath(cfg.PythonExecutable); err == nil {
		cfg.PythonExecutable = executable
	}
	generatorSetting := strings.TrimSpace(cfg.GeneratorPath)
	cfg.GeneratorPath = resolveConfiguredPath(generatorSetting, "video_caption_generator.py")
	if filepath.Clean(generatorSetting) == filepath.Clean("../python_tools/CaptionGenerator") {
		cfg.GeneratorPath = preferAvailablePath(
			cfg.GeneratorPath,
			"video_caption_generator.py",
			defaultDevWorkspacePaths("python_tools", "CaptionGenerator")...,
		)
	}
	envSetting := strings.TrimSpace(cfg.EnvFile)
	cfg.EnvFile = resolveConfiguredPath(envSetting, "")
	if filepath.Clean(envSetting) == filepath.Clean(".env") {
		cfg.EnvFile = preferAvailablePath(
			cfg.EnvFile,
			"",
			defaultDevWorkspacePaths("stash-vr", ".env")...,
		)
	}
	return &Service{
		cfg:         cfg,
		stashClient: cfg.StashClient,
		ctx:         ctx,
		cancel:      cancel,
		jobs:        make(map[string]*jobRecord),
	}
}

func resolveConfiguredPath(value, requiredChild string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}

	bases := make([]string, 0, 2)
	if workingDirectory, err := os.Getwd(); err == nil {
		bases = append(bases, workingDirectory)
	}
	if executable, err := os.Executable(); err == nil {
		executableDirectory := filepath.Dir(executable)
		duplicate := false
		for _, base := range bases {
			if strings.EqualFold(filepath.Clean(base), filepath.Clean(executableDirectory)) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			bases = append(bases, executableDirectory)
		}
	}
	return resolveConfiguredPathFrom(value, requiredChild, bases)
}

func resolveConfiguredPathFrom(value, requiredChild string, bases []string) string {
	var fallback string
	for _, base := range bases {
		candidate, err := filepath.Abs(filepath.Join(base, value))
		if err != nil {
			continue
		}
		if fallback == "" {
			fallback = candidate
		}
		target := candidate
		if requiredChild != "" {
			target = filepath.Join(candidate, requiredChild)
		}
		if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	if fallback != "" {
		return fallback
	}
	return filepath.Clean(value)
}

func preferAvailablePath(primary, requiredChild string, alternatives ...string) string {
	for _, candidate := range append([]string{primary}, alternatives...) {
		target := candidate
		if requiredChild != "" {
			target = filepath.Join(candidate, requiredChild)
		}
		if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
			return filepath.Clean(candidate)
		}
	}
	return primary
}

func defaultDevWorkspacePaths(parts ...string) []string {
	volumes := make([]string, 0, 2)
	addVolume := func(path string) {
		volume := filepath.VolumeName(path)
		if volume == "" {
			return
		}
		for _, existing := range volumes {
			if strings.EqualFold(existing, volume) {
				return
			}
		}
		volumes = append(volumes, volume)
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		addVolume(workingDirectory)
	}
	if executable, err := os.Executable(); err == nil {
		addVolume(executable)
	}

	result := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		pathParts := append([]string{volume + string(os.PathSeparator), "dev"}, parts...)
		result = append(result, filepath.Join(pathParts...))
	}
	return result
}

func (s *Service) Close() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.jobs {
		if record.cancel != nil && isActive(record.job.Status) {
			record.cancel()
		}
	}
}

func (s *Service) RuntimeStatus() RuntimeStatus {
	if _, err := exec.LookPath(s.cfg.PythonExecutable); err != nil {
		return RuntimeStatus{Message: "Python executable is unavailable"}
	}
	if s.cfg.GeneratorPath == "" {
		return RuntimeStatus{Message: "CaptionGenerator path is not configured"}
	}
	scriptPath := filepath.Join(s.cfg.GeneratorPath, "video_caption_generator.py")
	info, err := os.Stat(scriptPath)
	if err != nil || !info.Mode().IsRegular() {
		return RuntimeStatus{Message: "CaptionGenerator source is unavailable"}
	}
	if s.cfg.EnvFile != "" {
		if info, err := os.Stat(s.cfg.EnvFile); err != nil || !info.Mode().IsRegular() {
			return RuntimeStatus{
				Available: true,
				Message:   "Ready; API-backed options require keys in the process environment",
			}
		}
	}
	return RuntimeStatus{Available: true, Message: "Ready"}
}

// Prepare installs the Stash task bridge without starting a subtitle job.
func (s *Service) Prepare(ctx context.Context) error {
	if s.stashClient == nil {
		return nil
	}
	return s.ensureStashPlugin(ctx)
}

func (s *Service) State(sceneID string, videoPaths []string) State {
	return State{
		Runtime: s.RuntimeStatus(),
		Files:   RelatedFiles(videoPaths),
		Job:     s.Job(sceneID),
	}
}

func (s *Service) Job(sceneID string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record := s.jobs[sceneID]
	if record == nil {
		return nil
	}
	snapshot := record.job
	return &snapshot
}

func (s *Service) Start(ctx context.Context, sceneID, videoPath string, options Options) (*Job, error) {
	sceneID = strings.TrimSpace(sceneID)
	videoPath = strings.TrimSpace(videoPath)
	if sceneID == "" || videoPath == "" {
		return nil, errors.New("scene and video path are required")
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		return nil, fmt.Errorf("video file is unavailable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("video path is not a regular file")
	}
	runtime := s.RuntimeStatus()
	if !runtime.Available {
		return nil, errors.New(runtime.Message)
	}
	options, err = normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.mu.RLock()
	if existing := s.jobs[sceneID]; existing != nil && isActive(existing.job.Status) {
		snapshot := existing.job
		s.mu.RUnlock()
		return &snapshot, fmt.Errorf("a subtitle job is already active for this scene")
	}
	s.mu.RUnlock()

	if s.stashClient != nil {
		return s.startStashJob(ctx, sceneID, videoPath, options)
	}

	jobID, err := newJobID()
	if err != nil {
		return nil, fmt.Errorf("create job id: %w", err)
	}
	jobCtx, cancel := context.WithCancel(s.ctx)
	record := &jobRecord{
		job: Job{
			ID:        jobID,
			SceneID:   sceneID,
			Status:    StatusQueued,
			Progress:  0,
			Stage:     "Queued",
			Options:   options,
			StartedAt: time.Now().UTC(),
		},
		cancel: cancel,
	}
	s.mu.Lock()
	s.jobs[sceneID] = record
	s.mu.Unlock()
	snapshot := record.job
	go func() {
		defer cancel()
		s.runJob(jobCtx, sceneID, jobID, videoPath, options)
	}()
	return &snapshot, nil
}

func (s *Service) startStashJob(ctx context.Context, sceneID, videoPath string, options Options) (*Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.ensureStashPlugin(ctx); err != nil {
		return nil, err
	}

	args := map[string]interface{}{
		"sceneID":              sceneID,
		"video":                videoPath,
		"generatorPath":        s.cfg.GeneratorPath,
		"envFile":              s.cfg.EnvFile,
		"sourceLanguage":       options.SourceLanguage,
		"mode":                 options.Mode,
		"transcriptionService": options.TranscriptionService,
		"transcriptionModel":   options.TranscriptionModel,
		"translationService":   options.TranslationService,
		"translationModel":     options.TranslationModel,
	}
	description := fmt.Sprintf("Generate subtitles · %s · scene %s", filepath.Base(videoPath), sceneID)
	response, err := gql.RunSubtitlePluginTask(
		ctx,
		s.stashClient,
		stashPluginID,
		description,
		stashPluginTaskName,
		&args,
	)
	if err != nil {
		return nil, fmt.Errorf("start Stash subtitle task: %w", err)
	}
	jobID := ""
	if response != nil {
		jobID = strings.TrimSpace(response.RunPluginTask)
	}
	if jobID == "" {
		return nil, errors.New("Stash did not return a subtitle task ID")
	}

	record := &jobRecord{job: Job{
		ID:        jobID,
		SceneID:   sceneID,
		Status:    StatusQueued,
		Progress:  0,
		Stage:     "Queued in Stash",
		Options:   options,
		StartedAt: time.Now().UTC(),
	}}
	s.mu.Lock()
	s.jobs[sceneID] = record
	s.mu.Unlock()

	snapshot := record.job
	go s.pollStashJob(sceneID, jobID)
	return &snapshot, nil
}

func (s *Service) pollStashJob(sceneID, jobID string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		terminal, err := s.syncStashJob(sceneID, jobID)
		if err != nil {
			log.Warn().Err(err).Str("sceneId", sceneID).Str("stashJob", jobID).
				Msg("subtitle: failed to poll Stash task")
		}
		if terminal {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) syncStashJob(sceneID, jobID string) (bool, error) {
	response, err := gql.FindSubtitleJob(s.ctx, s.stashClient, jobID)
	if err != nil {
		return false, err
	}
	if response == nil || response.FindJob == nil {
		return false, nil
	}
	stashJob := response.FindJob
	terminal := false
	s.updateJob(sceneID, jobID, func(job *Job) {
		switch stashJob.Status {
		case gql.JobStatusReady:
			job.Status = StatusQueued
			job.Stage = "Queued in Stash"
		case gql.JobStatusRunning:
			job.Status = StatusRunning
			job.Stage = "Generating subtitles in Stash"
		case gql.JobStatusStopping:
			job.Status = StatusRunning
			job.Stage = "Stopping in Stash"
		case gql.JobStatusFinished:
			job.Status = StatusSucceeded
			job.Progress = 100
			job.Stage = "Subtitle generation complete"
			terminal = true
		case gql.JobStatusCancelled:
			job.Status = StatusCancelled
			job.Stage = "Cancelled in Stash"
			terminal = true
		case gql.JobStatusFailed:
			job.Status = StatusFailed
			job.Stage = "Subtitle generation failed"
			terminal = true
		}
		if stashJob.Progress != nil && !terminal {
			job.Progress = max(0, min(100, *stashJob.Progress*100))
		}
		if stashJob.StartTime != nil {
			job.StartedAt = stashJob.StartTime.UTC()
		} else if !stashJob.AddTime.IsZero() {
			job.StartedAt = stashJob.AddTime.UTC()
		}
		if stashJob.EndTime != nil {
			finished := stashJob.EndTime.UTC()
			job.FinishedAt = &finished
		} else if terminal {
			finished := time.Now().UTC()
			job.FinishedAt = &finished
		}
		if stashJob.Error != nil {
			job.Error = strings.TrimSpace(*stashJob.Error)
		}
	})
	return terminal, nil
}

func (s *Service) ensureStashPlugin(ctx context.Context) error {
	s.pluginMu.Lock()
	defer s.pluginMu.Unlock()
	if s.pluginReady {
		return nil
	}

	configuration, err := gql.SubtitlePluginConfiguration(ctx, s.stashClient)
	if err != nil {
		return fmt.Errorf("read Stash plugin configuration: %w", err)
	}
	if configuration == nil || configuration.Configuration == nil || configuration.Configuration.General == nil {
		return errors.New("Stash plugin configuration is unavailable")
	}
	pluginsPath := resolveConfiguredPath(configuration.Configuration.General.PluginsPath, "")
	if pluginsPath == "" {
		return errors.New("Stash plugins path is not configured")
	}

	yamlPath := filepath.Join(pluginsPath, stashPluginID+".yml")
	scriptPath := filepath.Join(pluginsPath, stashPluginID, "caption_job.py")
	yaml := []byte(s.stashPluginYAML())
	yamlChanged, err := writePluginFile(yamlPath, yaml)
	if err != nil {
		return fmt.Errorf("install Stash subtitle plugin config: %w", err)
	}
	scriptChanged, err := writePluginFile(scriptPath, captionJobScript)
	if err != nil {
		return fmt.Errorf("install Stash subtitle plugin runner: %w", err)
	}

	loaded := subtitlePluginLoaded(configuration.Plugins)
	if yamlChanged || scriptChanged || !loaded {
		response, reloadErr := gql.ReloadSubtitlePlugins(ctx, s.stashClient)
		if reloadErr != nil {
			return fmt.Errorf("reload Stash plugins: %w", reloadErr)
		}
		if response == nil || !response.ReloadPlugins {
			return errors.New("Stash did not reload the subtitle plugin")
		}
		configuration, err = gql.SubtitlePluginConfiguration(ctx, s.stashClient)
		if err != nil {
			return fmt.Errorf("verify Stash subtitle plugin: %w", err)
		}
		loaded = subtitlePluginLoaded(configuration.Plugins)
	}
	if !loaded {
		return errors.New("Stash did not load the subtitle generator plugin")
	}
	s.pluginReady = true
	return nil
}

func subtitlePluginLoaded(plugins []*gql.SubtitlePluginConfigurationPluginsPlugin) bool {
	for _, plugin := range plugins {
		if plugin == nil || plugin.Id != stashPluginID || !plugin.Enabled {
			continue
		}
		for _, task := range plugin.Tasks {
			if task != nil && task.Name == stashPluginTaskName {
				return true
			}
		}
	}
	return false
}

func (s *Service) stashPluginYAML() string {
	return fmt.Sprintf(`name: Stash VR Caption Generator
description: Generates one video subtitle job requested by Stash-VR.
version: 1.1.0
exec:
  - %s
  - "{pluginDir}/stash_vr_caption/caption_job.py"
  - "--stash-plugin"
interface: raw
errLog: info
tasks:
  - name: %s
    description: Generate subtitles for a single Stash scene video.
`, strconv.Quote(s.cfg.PythonExecutable), strconv.Quote(stashPluginTaskName))
}

func writePluginFile(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

func newJobID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func isActive(status string) bool {
	return status == StatusQueued || status == StatusRunning
}

func normalizeOptions(options Options) (Options, error) {
	if options.SourceLanguage == "" {
		options.SourceLanguage = "ja"
	}
	if options.Mode == "" {
		options.Mode = "transcribe_translate"
	}
	if options.TranscriptionService == "" {
		options.TranscriptionService = "assemblyai"
	}
	if options.TranscriptionModel == "" {
		options.TranscriptionModel = "models/gemini-3.1-pro-preview"
	}
	if options.TranslationService == "" {
		options.TranslationService = "openai"
	}
	if options.TranslationModel == "" {
		options.TranslationModel = defaultTranslationModel(options.TranslationService)
	}

	if !oneOf(options.SourceLanguage, "ja", "zh") {
		return options, errors.New("unsupported source language")
	}
	if !oneOf(options.Mode, "transcribe_translate", "transcribe_only", "translate_only") {
		return options, errors.New("unsupported subtitle mode")
	}
	if options.Mode != "translate_only" &&
		!oneOf(options.TranscriptionService, "openai", "gemini", "assemblyai", "granite", "qwen3", "kotoba", "reazonspeech") {
		return options, errors.New("unsupported transcription service")
	}
	if options.SourceLanguage != "ja" &&
		oneOf(options.TranscriptionService, "granite", "kotoba", "reazonspeech") {
		return options, errors.New("the selected transcription service supports Japanese only")
	}
	if options.TranscriptionService == "gemini" &&
		!oneOf(options.TranscriptionModel,
			"models/gemini-3.5-flash",
			"models/gemini-3.1-pro-preview",
			"models/gemini-2.5-flash") {
		return options, errors.New("unsupported Gemini transcription model")
	}
	if options.Mode != "transcribe_only" &&
		!oneOf(options.TranslationService, "openai", "gemini", "local_llm") {
		return options, errors.New("unsupported translation service")
	}
	if options.Mode != "transcribe_only" {
		switch options.TranslationService {
		case "openai":
			if !oneOf(options.TranslationModel, "gpt-4o-mini", "gpt-4o") {
				return options, errors.New("unsupported OpenAI translation model")
			}
		case "gemini":
			if !oneOf(options.TranslationModel,
				"models/gemini-3.5-flash",
				"models/gemini-3.1-pro-preview",
				"models/gemini-2.5-flash") {
				return options, errors.New("unsupported Gemini translation model")
			}
		case "local_llm":
			if !oneOf(options.TranslationModel,
				"qwen3:14b",
				"qwen3:8b",
				"qwen3:30b-a3b",
				"deepseek-r1:14b",
				"deepseek-r1:7b") {
				return options, errors.New("unsupported local translation model")
			}
		}
	}
	return options, nil
}

func defaultTranslationModel(service string) string {
	switch service {
	case "gemini":
		return "models/gemini-3.1-pro-preview"
	case "local_llm":
		return "qwen3:14b"
	default:
		return "gpt-4o-mini"
	}
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func (s *Service) runJob(ctx context.Context, sceneID, jobID, videoPath string, options Options) {
	s.updateJob(sceneID, jobID, func(job *Job) {
		job.Status = StatusRunning
		job.Progress = 1
		job.Stage = "Starting CaptionGenerator"
	})

	runner, err := os.CreateTemp("", "stash-vr-caption-*.py")
	if err != nil {
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}
	runnerPath := runner.Name()
	defer os.Remove(runnerPath)
	if _, err := runner.Write(captionJobScript); err != nil {
		_ = runner.Close()
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}
	if err := runner.Close(); err != nil {
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}

	args := []string{
		runnerPath,
		"--generator-path", s.cfg.GeneratorPath,
		"--video", videoPath,
		"--source-language", options.SourceLanguage,
		"--mode", options.Mode,
		"--transcription-service", options.TranscriptionService,
		"--transcription-model", options.TranscriptionModel,
		"--translation-service", options.TranslationService,
		"--translation-model", options.TranslationModel,
	}
	if s.cfg.EnvFile != "" {
		args = append(args, "--env-file", s.cfg.EnvFile)
	}

	cmd := exec.CommandContext(ctx, s.cfg.PythonExecutable, args...)
	cmd.Dir = s.cfg.GeneratorPath
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONUNBUFFERED=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}
	if err := cmd.Start(); err != nil {
		s.finishJob(sceneID, jobID, StatusFailed, err)
		return
	}

	var stderrMu sync.Mutex
	stderrLines := make([]string, 0, 12)
	var scanWG sync.WaitGroup
	scanWG.Add(2)
	go func() {
		defer scanWG.Done()
		s.scanProgress(sceneID, jobID, stdout)
	}()
	go func() {
		defer scanWG.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			log.Debug().Str("sceneId", sceneID).Str("captionJob", jobID).Msg(line)
			stderrMu.Lock()
			if len(stderrLines) == cap(stderrLines) {
				copy(stderrLines, stderrLines[1:])
				stderrLines = stderrLines[:len(stderrLines)-1]
			}
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()
		}
	}()

	waitErr := cmd.Wait()
	scanWG.Wait()
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			s.finishJob(sceneID, jobID, StatusCancelled, errors.New("subtitle job was cancelled"))
			return
		}
		stderrMu.Lock()
		detail := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		if detail != "" {
			waitErr = fmt.Errorf("%w: %s", waitErr, detail)
		}
		s.finishJob(sceneID, jobID, StatusFailed, waitErr)
		return
	}
	s.finishJob(sceneID, jobID, StatusSucceeded, nil)
}

type progressEvent struct {
	Type     string  `json:"type"`
	Progress float64 `json:"progress"`
	Stage    string  `json:"stage"`
	Message  string  `json:"message"`
}

func (s *Service) scanProgress(sceneID, jobID string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event progressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			log.Debug().Str("sceneId", sceneID).Str("captionJob", jobID).Msg(line)
			continue
		}
		switch event.Type {
		case "progress":
			s.updateJob(sceneID, jobID, func(job *Job) {
				if event.Progress >= 0 {
					job.Progress = max(0, min(100, event.Progress))
				}
				if strings.TrimSpace(event.Stage) != "" {
					job.Stage = strings.TrimSpace(event.Stage)
				}
			})
		case "log":
			if event.Message != "" {
				log.Debug().Str("sceneId", sceneID).Str("captionJob", jobID).Msg(event.Message)
			}
		}
	}
}

func (s *Service) updateJob(sceneID, jobID string, update func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.jobs[sceneID]
	if record == nil || record.job.ID != jobID {
		return
	}
	update(&record.job)
}

func (s *Service) finishJob(sceneID, jobID, status string, jobErr error) {
	now := time.Now().UTC()
	s.updateJob(sceneID, jobID, func(job *Job) {
		job.Status = status
		job.FinishedAt = &now
		if status == StatusSucceeded {
			job.Progress = 100
			job.Stage = "Subtitle generation complete"
			job.Error = ""
		} else {
			if status == StatusCancelled {
				job.Stage = "Cancelled"
			} else {
				job.Stage = "Subtitle generation failed"
			}
			if jobErr != nil {
				job.Error = jobErr.Error()
			}
		}
	})
}

func RelatedFiles(videoPaths []string) []File {
	files := make([]File, 0)
	for index, videoPath := range videoPaths {
		videoPath = strings.TrimSpace(videoPath)
		if videoPath == "" {
			continue
		}
		dir := filepath.Dir(videoPath)
		videoBase := filepath.Base(videoPath)
		stem := strings.TrimSuffix(videoBase, filepath.Ext(videoBase))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !isRelatedSRT(stem, entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			files = append(files, File{
				Key:            encodeFileKey(index, entry.Name()),
				Name:           entry.Name(),
				SourceBasename: videoBase,
				Size:           info.Size(),
				ModifiedAt:     info.ModTime().UTC(),
			})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].SourceBasename != files[j].SourceBasename {
			return strings.ToLower(files[i].SourceBasename) < strings.ToLower(files[j].SourceBasename)
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files
}

func isRelatedSRT(videoStem, name string) bool {
	lowerStem := strings.ToLower(videoStem)
	lowerName := strings.ToLower(name)
	if lowerName == lowerStem+".srt" {
		return true
	}
	return strings.HasPrefix(lowerName, lowerStem+".") && strings.HasSuffix(lowerName, ".srt")
}

func encodeFileKey(index int, name string) string {
	value := strconv.Itoa(index) + "\x00" + name
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeFileKey(key string) (int, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return 0, "", errors.New("invalid subtitle key")
	}
	parts := strings.SplitN(string(decoded), "\x00", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("invalid subtitle key")
	}
	index, err := strconv.Atoi(parts[0])
	if err != nil || index < 0 {
		return 0, "", errors.New("invalid subtitle key")
	}
	name := parts[1]
	if name == "" || filepath.Base(name) != name {
		return 0, "", errors.New("invalid subtitle filename")
	}
	return index, name, nil
}

func resolveFile(videoPaths []string, key string) (string, error) {
	index, name, err := decodeFileKey(key)
	if err != nil {
		return "", err
	}
	if index >= len(videoPaths) {
		return "", errors.New("subtitle source is unavailable")
	}
	videoPath := strings.TrimSpace(videoPaths[index])
	videoStem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	if !isRelatedSRT(videoStem, name) {
		return "", errors.New("subtitle is unrelated to this video")
	}
	path := filepath.Join(filepath.Dir(videoPath), name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("subtitle is not a regular file")
	}
	return path, nil
}

func OpenFile(videoPaths []string, key string) (*os.File, error) {
	path, err := resolveFile(videoPaths, key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Service) DeleteFile(sceneID string, videoPaths []string, key string) error {
	if job := s.Job(sceneID); job != nil && isActive(job.Status) {
		return errors.New("wait for the active subtitle job before deleting sidecar files")
	}
	path, err := resolveFile(videoPaths, key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete subtitle: %w", err)
	}
	return nil
}
