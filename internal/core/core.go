package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dvd-audio-studio/internal/i18n"
	"dvd-audio-studio/internal/runtimeenv"
)

type AudioFile struct {
	audioDuration float64
	HasVideo      bool    `json:"video,omitempty"`
	Path          string  `json:"path"`
	Duration      float64 `json:"duration"`
	Channels      int     `json:"channels"`
	SampleRate    int     `json:"sampleRate"`
	Error         string  `json:"error,omitempty"`
}
type Group struct {
	Name   string   `json:"name"`
	Tracks []string `json:"tracks"`
}
type Project struct {
	Format        string  `json:"format,omitempty"`
	VideoStandard string  `json:"videoStandard,omitempty"`
	Name          string  `json:"name"`
	SampleRate    int     `json:"sampleRate"`
	Bits          int     `json:"bits"`
	Channels      int     `json:"channels"`
	Capacity      string  `json:"capacity"`
	ISO           bool    `json:"iso"`
	Groups        []Group `json:"groups"`
}
type Estimate struct {
	Duration       float64 `json:"duration"`
	PCMBytes       int64   `json:"pcmBytes"`
	EstimatedBytes int64   `json:"estimatedBytes"`
	CapacityBytes  int64   `json:"capacityBytes"`
	WorkspaceBytes int64   `json:"workspaceBytes"`
	VideoBitrate   int64   `json:"videoBitrate,omitempty"`
	VideoWidth     int     `json:"videoWidth,omitempty"`
	Fits           bool    `json:"fits"`
}
type Tool struct {
	bundleRoot string
	Name       string `json:"name"`
	Path       string `json:"path"`
	Ready      bool   `json:"ready"`
	Detail     string `json:"detail"`
}
type Job struct {
	Format string   `json:"format,omitempty"`
	Kind   string   `json:"kind,omitempty"`
	ID     string   `json:"id"`
	State  string   `json:"state"`
	Log    []string `json:"log"`
	Error  string   `json:"error,omitempty"`
	Output string   `json:"output,omitempty"`
}
type cachedAudio struct {
	size     int64
	modified time.Time
	file     AudioFile
}
type Engine struct {
	root, output, app string
	tools             []Tool
	mu                sync.Mutex
	cacheMu           sync.Mutex
	cache             map[string]cachedAudio
	job               *Job
	cancel            context.CancelFunc
	done              chan struct{}
	closed            bool
	lock              *os.File
	images            map[string]vcdImage
	// Tests can supply optical metadata without ever touching a physical drive.
	recorderList func(context.Context) ([]Recorder, error)
}

// Config paths are resolved once; all source and output operations use canonical paths.
type Config struct {
	Root   string
	Output string
	App    string
}

var (
	ErrBusy         = errors.New("another authoring job is running")
	ErrDependencies = errors.New("required tools are unavailable")
	ErrNoJob        = errors.New("no job is running")
	ErrClosed       = errors.New("engine is closed")
)

func New(c Config) (*Engine, error) {
	canonical := func(path string) (string, error) {
		if path == "" {
			return "", errors.New("directory path must not be empty")
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		abs, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", path)
		}
		return abs, nil
	}
	root, err := canonical(c.Root)
	if err != nil {
		return nil, fmt.Errorf("library root: %w", err)
	}
	if c.App == "" {
		c.App, err = ApplicationDir()
		if err != nil {
			return nil, err
		}
	}
	app, err := canonical(c.App)
	if err != nil {
		return nil, fmt.Errorf("application directory: %w", err)
	}
	if c.Output == "" {
		return nil, errors.New("output directory must not be empty")
	}
	if err := os.MkdirAll(c.Output, 0700); err != nil {
		return nil, fmt.Errorf("output directory: %w", err)
	}
	output, err := canonical(c.Output)
	if err != nil {
		return nil, fmt.Errorf("output directory: %w", err)
	}
	if within(output, root) {
		return nil, errors.New("output directory must not contain the audio library")
	}
	return &Engine{root: root, output: output, app: app, tools: discoverTools(app), cache: make(map[string]cachedAudio)}, nil
}

func (s *Engine) Root() string   { return s.root }
func (s *Engine) Output() string { return s.output }

// WithLanguage selects a caller's language without changing engine-wide state.
func WithLanguage(ctx context.Context, language string) context.Context {
	return i18n.WithLanguage(ctx, language)
}

func text(ctx context.Context, english string, args ...any) string {
	return i18n.Text(i18n.Language(ctx), english, args...)
}

func errorf(ctx context.Context, english string, args ...any) error {
	return errors.New(text(ctx, english, args...))
}

type localizedError struct {
	cause   error
	message string
}

func (e *localizedError) Error() string { return e.message }
func (e *localizedError) Unwrap() error { return e.cause }
func classified(ctx context.Context, cause error, english string, args ...any) error {
	return &localizedError{cause: cause, message: text(ctx, english, args...)}
}

func (s *Engine) Tools(ctx context.Context) []Tool {
	tools := append([]Tool(nil), s.tools...)
	for i := range tools {
		if detail, ok := strings.CutPrefix(tools[i].Detail, "Tool check failed: "); ok {
			tools[i].Detail = text(ctx, "Tool check failed: %s", detail)
		} else {
			tools[i].Detail = text(ctx, tools[i].Detail)
		}
	}
	return tools
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	copy := *job
	copy.Log = append([]string{}, job.Log...)
	return &copy
}

func (s *Engine) Snapshot() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneJob(s.job)
}

func (s *Engine) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return ErrNoJob
	}
	s.cancel()
	return nil
}

// Close prevents new jobs, cancels the active process group and waits for job.json.
func (s *Engine) Close() {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (s *Engine) Start(ctx context.Context, p Project) (*Job, error) {
	if p.Format == "dvd-video" {
		p.ISO = true
	}
	// The caller retains ownership of all slices, even while the background job runs.
	p.Groups = slices.Clone(p.Groups)
	for i := range p.Groups {
		p.Groups[i].Tracks = slices.Clone(p.Groups[i].Tracks)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, classified(ctx, ErrClosed, "Engine is closed")
	}
	if s.cancel != nil {
		return nil, classified(ctx, ErrBusy, "Another authoring job is running")
	}
	// The native lock is job-scoped and excludes other engines in this process. Scanning
	// and estimation do not take the lock, so web and desktop can coexist.
	lock, err := openLock(filepath.Join(s.output, ".authoring.lock"))
	if err != nil {
		if errors.Is(err, ErrBusy) {
			return nil, classified(ctx, ErrBusy, "Another authoring job is running")
		}
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			_ = lock.Close()
		}
	}()
	e, err := s.Estimate(ctx, p)
	if err != nil {
		return nil, err
	}
	if !e.Fits {
		if p.Format == "vcd" {
			return nil, errorf(ctx, "VCD exceeds selected CD capacity")
		}
		if p.Format == "dvd-video" {
			return nil, errorf(ctx, "DVD-Video cannot fit at the minimum supported quality")
		}
		return nil, errorf(ctx, "Decoded PCM exceeds disc capacity")
	}
	dependencies := []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs"}
	if p.Format == "vcd" {
		dependencies = []string{"ffmpeg", "ffprobe", "vcdimager"}
	}
	if p.Format == "dvd-video" {
		dependencies = []string{"ffmpeg", "ffprobe", "dvdauthor", "dvd-video-mkisofs"}
	}
	for _, name := range dependencies {
		if name == "mkisofs" && !p.ISO {
			continue
		}
		if s.toolPath(name) == "" {
			return nil, classified(ctx, ErrDependencies, "%s is unavailable or incompatible; check tool status", name)
		}
	}
	if p.Format == "dvd-video" {
		if err := s.checkDVDVideoEncoders(ctx); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	available, err := workspaceAvailable(s.output)
	if err != nil {
		return nil, err
	}
	if uint64(e.WorkspaceBytes) > available {
		if p.Format == "vcd" {
			return nil, errorf(ctx, "Not enough free workspace for MPEG tracks and VCD image")
		}
		if p.Format == "dvd-video" {
			return nil, errorf(ctx, "Not enough free workspace for MPEG tracks, VIDEO_TS and the ISO")
		}
		return nil, errorf(ctx, "Not enough free workspace for PCM, disc structure and ISO")
	}
	dir, err := os.MkdirTemp(s.output, "disc-")
	if err != nil {
		return nil, err
	}
	jobCtx, cancel := context.WithCancel(WithLanguage(context.Background(), i18n.Language(ctx)))
	s.cancel, s.lock, s.done = cancel, lock, make(chan struct{})
	s.job = &Job{ID: filepath.Base(dir), State: "running", Log: []string{}, Format: p.Format, Kind: "author"}
	started = true
	go s.runJob(jobCtx, p, e, dir)
	return cloneJob(s.job), nil
}

// ApplicationDir returns the native bundle root, or cwd for local development.
func ApplicationDir() (string, error) { return runtimeenv.ApplicationDir() }

func discoverTools(app string) []Tool {
	return discoverToolsFor(app, runtimeenv.BundleRoot(), runtime.GOOS)
}

func discoverToolsFor(app, bundle, goos string) []Tool {
	var tools []Tool
	for _, name := range []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager", "dvdauthor", "cdrdao"} {
		if name == "cdrdao" && goos != "linux" {
			continue
		}
		path := os.Getenv(map[string]string{"ffmpeg": "FFMPEG", "ffprobe": "FFPROBE", "dvda-author": "DVDA_AUTHOR", "mkisofs": "MKISOFS", "vcdimager": "VCDIMAGER", "dvdauthor": "DVDAUTHOR", "cdrdao": "CDRDAO"}[name])
		toolRoot := ""
		if name != "cdrdao" {
			roots := []string{bundle}
			if goos == "windows" || goos == "darwin" {
				roots = append(roots, app)
			}
			for _, root := range roots {
				if root == "" {
					continue
				}
				localName := name
				if goos == "windows" {
					localName += ".exe"
				}
				local := filepath.Join(runtimeenv.ToolDir(root), localName)
				if info, err := os.Stat(local); err == nil && info.Mode().IsRegular() && (path == "" || path == local) {
					path, toolRoot = local, root
					break
				}
			}
		}
		if path == "" && goos == "linux" && (name == "dvda-author" || name == "mkisofs") {
			local := filepath.Join(app, "tools", "bin", name)
			if _, err := os.Stat(local); err == nil {
				// Tracked Linux launchers require an ignored payload. Custom
				// local binaries can be selected with DVDA_AUTHOR or MKISOFS.
				payload := filepath.Join(app, "tools", "opt", "dvda-author", "dvda")
				if name == "mkisofs" {
					payload = filepath.Join(app, "tools", "opt", "dvda-author", "bin", "mkisofs")
				}
				if info, err := os.Stat(payload); err == nil && info.Mode().IsRegular() {
					path = local
				}
			}
		}
		if path == "" {
			path, _ = exec.LookPath(name)
			if path == "" && name == "dvda-author" {
				path, _ = exec.LookPath("dvda-author-dev")
			}
		}
		t := Tool{Name: name, Path: path, bundleRoot: toolRoot}
		if path == "" {
			t.Detail = "Not found; see README.md"
			tools = append(tools, t)
			continue
		}
		abs, err := filepath.Abs(path)
		if err == nil {
			t.Path = abs
		}
		arg := "-version"
		if name == "dvda-author" || name == "dvdauthor" || name == "mkisofs" {
			arg = "--help"
		}
		if name == "vcdimager" || name == "cdrdao" {
			arg = "--version"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		checkArgs := []string{arg}
		if name == "dvda-author" && t.bundleRoot != "" {
			// Match native authoring's config-independent invocation.
			checkArgs = []string{"-W", "-h"}
		}
		if name == "cdrdao" {
			checkArgs = []string{"write", "--help"}
		}
		cmd := exec.CommandContext(ctx, t.Path, checkArgs...)
		cmd.Env = runtimeenv.CommandEnv(t.bundleRoot)
		configureProcess(cmd)
		output, runErr := cmd.CombinedOutput()
		cancel()
		t.Ready = runErr == nil
		if name == "dvdauthor" {
			// Upstream help prints usage but may exit 1 when no standard is set.
			var exit *exec.ExitError
			t.Ready = (runErr == nil || (errors.As(runErr, &exit) && exit.ExitCode() == 1)) &&
				strings.Contains(string(output), "syntax: dvdauthor") && strings.Contains(string(output), "-x XMLFILE")
			if !t.Ready {
				t.Detail = "Requires dvdauthor with XML authoring support"
			}
		}
		if name == "cdrdao" {
			// cdrdao 1.2.x returns status 1 for help. This command parses
			// usage only; unlike scanbus/drive-info it never opens a drive.
			var exit *exec.ExitError
			t.Ready = (runErr == nil || (errors.As(runErr, &exit) && exit.ExitCode() == 1)) &&
				strings.Contains(string(output), "Cdrdao version ") && strings.Contains(string(output), " write [options] toc-file") &&
				strings.Contains(string(output), "--device") && strings.Contains(string(output), "--simulate") && strings.Contains(string(output), "--speed")
		}
		if name == "mkisofs" {
			t.Ready = strings.Contains(string(output), "-dvd-audio")
			if !t.Ready {
				t.Detail = "Requires real -dvd-audio support; genisoimage is not sufficient"
			}
		}
		if runErr != nil && t.Detail == "" {
			t.Detail = "Tool check failed: " + runErr.Error()
		}
		if t.Ready {
			t.Detail = "Ready"
		}
		tools = append(tools, t)
	}
	return append(tools, discoverDVDVideoISO(tools))
}

func (s *Engine) toolPath(name string) string {
	for _, t := range s.tools {
		if t.Name == name && t.Ready {
			return t.Path
		}
	}
	return ""
}

func (s *Engine) toolBundleRoot(path string) string {
	for _, tool := range s.tools {
		if tool.Path == path {
			return tool.bundleRoot
		}
	}
	return ""
}

func (s *Engine) runtimePath(path string) bool {
	for _, root := range []string{s.app, runtimeenv.BundleRoot()} {
		if root == "" {
			continue
		}
		resources := runtimeenv.ResourceDir(root)
		if resources != root && (within(resources, path) || within(filepath.Join(root, "MacOS"), path)) {
			return true
		}
		for _, protected := range []string{runtimeenv.ToolDir(root), runtimeenv.LibraryDir(root), filepath.Join(resources, "tools"), filepath.Join(resources, "share"), filepath.Join(resources, "etc")} {
			if resolved, err := filepath.EvalSymlinks(protected); err == nil {
				protected = resolved
			}
			if within(protected, path) {
				return true
			}
		}
	}
	return false
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func (s *Engine) source(ctx context.Context, path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", errorf(ctx, "Source must be relative to the audio library: %q", path)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(s.root, filepath.FromSlash(path)))
	if err != nil {
		return "", errorf(ctx, "Cannot read source: %s", err)
	}
	if !within(s.root, full) || within(s.output, full) || s.runtimePath(full) {
		return "", errorf(ctx, "Source is outside the audio library: %s", path)
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", errorf(ctx, "Cannot read source: %s", err)
	}
	if !info.Mode().IsRegular() {
		return "", errorf(ctx, "Not a regular file: %s", path)
	}
	return full, nil
}
func (s *Engine) probe(ctx context.Context, path string) AudioFile {
	f := AudioFile{Path: path}
	if err := ctx.Err(); err != nil {
		f.Error = text(ctx, "Cannot read source: %s", err)
		return f
	}
	full, err := s.source(ctx, path)
	if err != nil {
		f.Error = err.Error()
		return f
	}
	info, err := os.Stat(full)
	if err != nil {
		f.Error = text(ctx, "Cannot read source: %s", err)
		return f
	}
	s.cacheMu.Lock()
	old, ok := s.cache[path]
	s.cacheMu.Unlock()
	if ok && old.size == info.Size() && old.modified.Equal(info.ModTime()) {
		return old.file
	}
	if s.toolPath("ffprobe") == "" {
		f.Error = text(ctx, "ffprobe is unavailable")
		return f
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.toolPath("ffprobe"), "-v", "error", "-show_entries", "format=duration:stream=codec_type,sample_rate,channels,duration:stream_disposition=attached_pic", "-of", "json", full)
	cmd.Env = runtimeenv.CommandEnv(s.toolBundleRoot(cmd.Path))
	configureProcess(cmd)
	b, err := cmd.Output()
	var data struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err == nil {
		err = json.Unmarshal(b, &data)
	}
	if err != nil {
		f.Error = text(ctx, "ffprobe failed: %s", err)
		return f
	}
	for _, stream := range data.Streams {
		if stream.CodecType == "video" && stream.Disposition.AttachedPic == 0 {
			f.HasVideo = true
		}
		if stream.CodecType == "audio" && f.Channels == 0 {
			f.Channels = stream.Channels
			f.SampleRate, _ = strconv.Atoi(stream.SampleRate)
			f.Duration, _ = strconv.ParseFloat(stream.Duration, 64)
		}
	}
	if f.Channels == 0 && !f.HasVideo {
		f.Error = text(ctx, "The file contains no audio")
		return f
	}
	if f.Duration <= 0 {
		f.Duration, _ = strconv.ParseFloat(data.Format.Duration, 64)
	}
	if f.HasVideo {
		f.audioDuration = f.Duration
		if duration, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil && duration > f.Duration {
			f.Duration = duration
		}
	}
	if f.Duration <= 0 || math.IsNaN(f.Duration) || math.IsInf(f.Duration, 0) || (!f.HasVideo && (f.SampleRate <= 0 || f.Channels <= 0)) {
		f.Error = text(ctx, "Unknown or invalid audio duration or format")
		return f
	}
	s.cacheMu.Lock()
	s.cache[path] = cachedAudio{info.Size(), info.ModTime(), f}
	s.cacheMu.Unlock()
	return f
}
func (s *Engine) Library(ctx context.Context) ([]AudioFile, error) {
	return s.scanLibrary(ctx, s.root, false)
}

// ScanFolder scans only the requested directory inside the configured library.
// Track paths remain relative to the library root, not to the selected folder.
func (s *Engine) ScanFolder(ctx context.Context, folder string) ([]AudioFile, error) {
	folder = filepath.Clean(folder)
	if !filepath.IsAbs(folder) {
		folder = filepath.Join(s.root, folder)
	}
	if !within(s.root, folder) {
		return nil, errorf(ctx, "Source is outside the audio library: %s", folder)
	}
	rel, _ := filepath.Rel(s.root, folder)
	dir := s.root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." {
			continue
		}
		dir = filepath.Join(dir, part)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.HasPrefix(part, ".") || within(s.output, dir) || dir == s.app {
			return nil, errorf(ctx, "Source is outside the audio library: %s", folder)
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || s.runtimePath(dir) {
			return nil, errorf(ctx, "Source is outside the audio library: %s", folder)
		}
	}
	return s.scanLibrary(ctx, folder, true)
}

func (s *Engine) scanLibrary(ctx context.Context, folder string, scoped bool) ([]AudioFile, error) {
	var paths []string
	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if s.runtimePath(path) || (path != s.root && (within(s.output, path) || path == s.app || strings.HasPrefix(d.Name(), "."))) {
				return filepath.SkipDir
			}
			return nil
		}
		if scoped && !d.Type().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".mp3", ".m4a", ".aac", ".flac", ".wav", ".ogg", ".opus", ".webm", ".mp4", ".aiff", ".aif", ".wma", ".mov", ".mkv", ".avi", ".mpeg", ".mpg", ".m2v", ".m1v", ".mts", ".m2ts", ".vob", ".wmv", ".ogv", ".ts":
		default:
			return nil
		}
		rel, _ := filepath.Rel(s.root, path)
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	files := make([]AudioFile, len(paths))
	indices := make(chan int)
	var workers sync.WaitGroup
	for n := 0; n < 4; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range indices {
				files[i] = s.probe(ctx, paths[i])
			}
		}()
	}
	for i := range paths {
		select {
		case indices <- i:
		case <-ctx.Done():
			close(indices)
			workers.Wait()
			return nil, ctx.Err()
		}
	}
	close(indices)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return files, nil
}
func (s *Engine) Estimate(ctx context.Context, p Project) (Estimate, error) {
	e := Estimate{}
	if err := ctx.Err(); err != nil {
		return e, err
	}
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 200 {
		return e, errorf(ctx, "Project name must contain 1-200 bytes")
	}
	if p.Format == "vcd" {
		return s.estimateVCD(ctx, p)
	}
	if p.Format == "dvd-video" {
		return s.estimateDVDVideo(ctx, p)
	}
	if p.Format != "" && p.Format != "dvd-audio" {
		return e, errorf(ctx, "Format must be dvd-audio, dvd-video or vcd")
	}
	if p.VideoStandard != "" {
		return e, errorf(ctx, "Video standard is only available for DVD-Video and VCD")
	}
	if p.Bits != 16 && p.Bits != 24 {
		return e, errorf(ctx, "Choose 16-bit or 24-bit PCM")
	}
	if p.Channels != 1 && p.Channels != 2 {
		return e, errorf(ctx, "Only mono or stereo PCM is supported")
	}
	switch p.SampleRate {
	case 44100, 48000, 88200, 96000, 176400, 192000:
	default:
		return e, errorf(ctx, "Unsupported DVD-Audio sample rate")
	}
	switch p.Capacity {
	case "dvd5":
		e.CapacityBytes = 4700000000
	case "dvd9":
		e.CapacityBytes = 8547991552
	default:
		return e, errorf(ctx, "Capacity must be dvd5 or dvd9")
	}
	if len(p.Groups) == 0 || len(p.Groups) > 9 {
		return e, errorf(ctx, "Use 1-9 groups")
	}
	count := 0
	for _, g := range p.Groups {
		if len(g.Tracks) > 99 {
			return e, errorf(ctx, "A group may contain at most 99 tracks")
		}
		for _, path := range g.Tracks {
			f := s.probe(ctx, path)
			if f.audioDuration > 0 {
				f.Duration = f.audioDuration
			}
			if f.Error == "" && f.Channels == 0 {
				return e, errorf(ctx, "The file contains no audio")
			}
			if f.Error != "" {
				if f.Error == text(ctx, "ffprobe is unavailable") {
					return e, classified(ctx, ErrDependencies, "%s is unavailable or incompatible; check tool status", "ffprobe")
				}
				return e, errorf(ctx, "%s: %s", path, f.Error)
			}
			pcm := math.Ceil(f.Duration * float64(p.SampleRate*p.Channels*p.Bits/8))
			if pcm > float64(0xffffffff-4096) {
				return e, errorf(ctx, "%s exceeds the 4 GiB WAV limit; split the track or choose a smaller PCM profile", path)
			}
			e.Duration += f.Duration
			e.PCMBytes += int64(pcm)
			count++
		}
	}
	if count == 0 {
		return e, errorf(ctx, "Add at least one track")
	}
	// Conservative sector-packing and navigation allowance, not source-file size.
	e.EstimatedBytes = int64(float64(e.PCMBytes)*1.08) + int64(count)*1024*1024 + 16*1024*1024
	e.WorkspaceBytes = e.PCMBytes + e.EstimatedBytes
	if p.ISO {
		e.WorkspaceBytes += e.EstimatedBytes
	}
	e.WorkspaceBytes += 256 * 1024 * 1024
	e.Fits = e.EstimatedBytes <= e.CapacityBytes
	return e, nil
}

func (s *Engine) appendLog(line string) {
	if len(line) > 4000 {
		line = line[:4000]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.job.Log = append(s.job.Log, line)
	if len(s.job.Log) > 600 {
		s.job.Log = append([]string(nil), s.job.Log[len(s.job.Log)-600:]...)
	}
}
func (s *Engine) command(ctx context.Context, dir, path string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := s.toolBundleRoot(path)
	if root != "" && path == s.toolPath("dvda-author") {
		// Native upstream binaries do not have the Linux scratch-dir wrapper.
		scratch, err := os.MkdirTemp(dir, ".dvda-author-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(scratch)
		for _, name := range []string{"audio", "lplex", "lplex-output"} {
			if err := os.MkdirAll(filepath.Join(scratch, name), 0700); err != nil {
				return err
			}
		}
		prefix := []string{"-W", "--bindir", filepath.Dir(path), "--datadir", filepath.Join(runtimeenv.ResourceDir(root), "tools", "share", "dvda-author"), "--workdir", dir, "-D", filepath.Join(scratch, "audio"), "--lplex-tempdir", filepath.Join(scratch, "lplex"), "--lplex-output", filepath.Join(scratch, "lplex-output")}
		args = append(prefix, args...)
	}
	s.appendLog("$ " + filepath.Base(path) + " " + strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = runtimeenv.CommandEnv(root)
	cmd.Dir = dir
	configureProcess(cmd)
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout = w
	cmd.Stderr = w
	if err = cmd.Start(); err != nil {
		_ = r.Close()
		_ = w.Close()
		return err
	}
	_ = w.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer r.Close()
		scan := bufio.NewScanner(r)
		scan.Buffer(make([]byte, 4096), 1<<20)
		for scan.Scan() {
			s.appendLog(scan.Text())
		}
		_, _ = io.Copy(io.Discard, r)
	}()
	err = cmd.Wait()
	// This pipe is read outside os/exec, so WaitDelay cannot close it for us.
	timer := time.NewTimer(cmd.WaitDelay)
	defer timer.Stop()
	select {
	case <-done:
	case <-ctx.Done():
		// A launcher may already have exited while its children still hold the pipe.
		_ = cmd.Cancel()
		_ = r.Close()
		<-done
	case <-timer.C:
		_ = cmd.Cancel()
		_ = r.Close()
		<-done
		if err == nil {
			err = exec.ErrWaitDelay
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return errorf(ctx, "Command failed: %s", err)
	}
	return nil
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func (s *Engine) finishJob(ctx context.Context, runErr error, dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		s.job.State = "cancelled"
		s.job.Error = text(ctx, "Job cancelled; partial output was retained")
		if s.job.Kind == "burn" {
			s.job.Error = text(ctx, "Burn cancelled; the disc may be unusable. Image files were retained.")
		}
	} else if runErr != nil {
		s.job.State = "failed"
		s.job.Error = runErr.Error()
	} else {
		s.job.State = "completed"
	}
	if err := writeJSON(filepath.Join(dir, "job.json"), s.job); err != nil {
		s.job.State = "failed"
		s.job.Error = text(ctx, "Cannot persist job status: %s", err)
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	_ = s.lock.Close()
	s.lock = nil
	close(s.done)
}

func (s *Engine) runJob(ctx context.Context, p Project, e Estimate, dir string) {
	var runErr error
	defer func() { s.finishJob(ctx, runErr, dir) }()
	if runErr = writeJSON(filepath.Join(dir, "project.json"), p); runErr != nil {
		return
	}
	if p.Format == "vcd" {
		runErr = s.runVCD(ctx, p, e, dir)
		return
	}
	if p.Format == "dvd-video" {
		runErr = s.runDVDVideo(ctx, p, e, dir)
		return
	}
	s.appendLog(text(ctx, "Preparing %.2f hours of audio; estimated disc size %.2f GB. Source files will not be modified.", e.Duration/3600, float64(e.EstimatedBytes)/1e9))
	pcmDir := filepath.Join(dir, "pcm")
	discDir := filepath.Join(dir, "disc")
	for _, path := range []string{pcmDir, discDir} {
		if runErr = os.MkdirAll(path, 0700); runErr != nil {
			return
		}
	}
	args := []string{"-W", "-P0", "-o", discDir}
	for gi, g := range p.Groups {
		if len(g.Tracks) == 0 {
			continue
		}
		args = append(args, "-g")
		for ti, path := range g.Tracks {
			var full string
			full, runErr = s.source(ctx, path)
			if runErr != nil {
				return
			}
			wav := filepath.Join(pcmDir, fmt.Sprintf("g%02d-t%02d.wav", gi+1, ti+1))
			codec := "pcm_s16le"
			if p.Bits == 24 {
				codec = "pcm_s24le"
			}
			s.appendLog(text(ctx, "Group %d / track %d: %s", gi+1, ti+1, path))
			runErr = s.command(ctx, dir, s.toolPath("ffmpeg"), "-nostdin", "-hide_banner", "-loglevel", "warning", "-y", "-i", full, "-map", "0:a:0", "-vn", "-sn", "-dn", "-map_metadata", "-1", "-ac", strconv.Itoa(p.Channels), "-ar", strconv.Itoa(p.SampleRate), "-c:a", codec, "-rf64", "never", wav)
			if runErr != nil {
				return
			}
			args = append(args, wav)
		}
	}
	s.appendLog(text(ctx, "Creating DVD-Audio navigation and AOB files..."))
	if runErr = s.command(ctx, dir, s.toolPath("dvda-author"), args...); runErr != nil {
		return
	}
	var aob bool
	runErr = filepath.WalkDir(filepath.Join(discDir, "AUDIO_TS"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.EqualFold(filepath.Ext(path), ".AOB") {
			info, err := d.Info()
			if err != nil {
				return err
			}
			aob = aob || info.Size() > 0
		}
		return nil
	})
	if runErr != nil {
		return
	}
	for _, name := range []string{"AUDIO_TS.IFO", "AUDIO_TS.BUP", "ATS_01_0.IFO"} {
		info, err := os.Stat(filepath.Join(discDir, "AUDIO_TS", name))
		if err != nil || info.Size() == 0 {
			runErr = errorf(ctx, "Authoring did not create a valid %s file", name)
			return
		}
	}
	if !aob {
		runErr = errorf(ctx, "Authoring produced no audio objects")
		return
	}
	var treeBytes int64
	runErr = filepath.WalkDir(discDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			treeBytes += info.Size()
		}
		return nil
	})
	if runErr != nil {
		return
	}
	if treeBytes > e.CapacityBytes {
		runErr = errorf(ctx, "Authored disc structure exceeds selected disc capacity")
		return
	}
	output := discDir
	if p.ISO {
		output = filepath.Join(dir, "dvd-audio.iso")
		if runErr = s.command(ctx, dir, s.toolPath("mkisofs"), "-dvd-audio", "-udf", "-V", "DVD_AUDIO", "-o", output, discDir); runErr != nil {
			return
		}
		info, err := os.Stat(output)
		if err != nil {
			runErr = err
			return
		}
		if info.Size() == 0 || info.Size() > e.CapacityBytes {
			runErr = errorf(ctx, "Final ISO is empty or exceeds selected disc capacity")
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	s.appendLog(text(ctx, "Completed: ") + output)
	s.appendLog(text(ctx, "Temporary PCM files were retained for inspection. Burn the ISO as a disc image, not as a file on a data disc."))
	s.mu.Lock()
	s.job.Output = output
	s.mu.Unlock()
}
