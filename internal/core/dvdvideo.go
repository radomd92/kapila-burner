package core

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dvd-audio-studio/internal/runtimeenv"
)

const (
	dvdVideoAudioRate = 192_000
	dvdVideoMinRate   = 900_000
	dvdVideoMaxRate   = 8_000_000
	dvdVideoReserve   = 16 * 1024 * 1024
)

func discoverDVDVideoISO(tools []Tool) Tool {
	t := Tool{Name: "dvd-video-mkisofs", Detail: "Not found; see README.md"}
	candidates := []Tool{}
	if override := os.Getenv("DVD_VIDEO_MKISOFS"); override != "" {
		candidates = append(candidates, Tool{Path: override})
	} else {
		for _, tool := range tools {
			if tool.Name == "mkisofs" && tool.Path != "" {
				candidates = append(candidates, tool)
			}
		}
		for _, name := range []string{"mkisofs", "genisoimage"} {
			if path, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, Tool{Path: path})
			}
		}
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate.Path)
		if err != nil || seen[path] {
			continue
		}
		seen[path] = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, path, "--help")
		cmd.Env = runtimeenv.CommandEnv(candidate.bundleRoot)
		configureProcess(cmd)
		out, _ := cmd.CombinedOutput() // Some mkisofs variants return nonzero for help.
		cancel()
		if t.Path == "" {
			t.Path, t.Detail = path, "Requires -dvd-video support"
		}
		if strings.Contains(string(out), "-dvd-video") {
			t.Path, t.bundleRoot, t.Ready, t.Detail = path, candidate.bundleRoot, true, "Ready"
			break
		}
	}
	return t
}

func dvdVideoWidth(rate int64) int {
	if rate < 2_800_000 {
		return 352 // Half-D1 makes low-bitrate MPEG-2 less blocky.
	}
	return 720
}

func planDVDVideo(duration float64, clips int, capacity int64) Estimate {
	// Allow for encoder/multiplexing variation, filesystem metadata and rounding
	// each independent clip to video frames and DVD packs.
	seconds := duration + float64(2*clips)
	payload := float64(capacity-dvdVideoReserve) / 1.04
	rate := min(int64(dvdVideoMaxRate), int64(math.Floor((payload*8/seconds-dvdVideoAudioRate)/1000))*1000)
	fits := rate >= dvdVideoMinRate
	rate = max(rate, dvdVideoMinRate)
	bytes := int64(math.Ceil(seconds*float64(rate+dvdVideoAudioRate)/8*1.04)) + dvdVideoReserve
	return Estimate{Duration: duration, EstimatedBytes: bytes, CapacityBytes: capacity,
		WorkspaceBytes: bytes*3 + 128*1024*1024, Fits: fits, VideoBitrate: rate, VideoWidth: dvdVideoWidth(rate)}
}

func (s *Engine) estimateDVDVideo(ctx context.Context, p Project) (Estimate, error) {
	var capacity int64
	switch p.Capacity {
	case "dvd5":
		capacity = 4_700_000_000
	case "dvd9":
		capacity = 8_540_000_000
	default:
		return Estimate{}, errorf(ctx, "DVD-Video capacity must be dvd5 or dvd9")
	}
	if p.VideoStandard != "" && p.VideoStandard != "pal" && p.VideoStandard != "ntsc" {
		return Estimate{}, errorf(ctx, "DVD-Video standard must be pal or ntsc")
	}
	if len(p.Groups) != 1 || len(p.Groups[0].Tracks) < 1 || len(p.Groups[0].Tracks) > 99 {
		return Estimate{}, errorf(ctx, "DVD-Video requires one group with 1-99 video clips")
	}
	var duration float64
	for _, path := range p.Groups[0].Tracks {
		f, err := s.dvdVideoClip(ctx, path)
		if err != nil {
			return Estimate{}, err
		}
		duration += f.Duration
	}
	return planDVDVideo(duration, len(p.Groups[0].Tracks), capacity), nil
}

func (s *Engine) dvdVideoClip(ctx context.Context, path string) (AudioFile, error) {
	f := s.probe(ctx, path)
	if f.Error != "" {
		if f.Error == text(ctx, "ffprobe is unavailable") {
			return f, classified(ctx, ErrDependencies, "%s is unavailable or incompatible; check tool status", "ffprobe")
		}
		return f, errorf(ctx, "%s: %s", path, f.Error)
	}
	if !f.HasVideo {
		return f, errorf(ctx, "DVD-Video requires a video stream: %s", path)
	}
	if f.Duration <= 0 || math.IsNaN(f.Duration) || math.IsInf(f.Duration, 0) || f.Duration > 24*3600 {
		return f, errorf(ctx, "Unknown or invalid audio duration or format")
	}
	return f, nil
}

func (s *Engine) checkDVDVideoEncoders(ctx context.Context) error {
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(check, s.toolPath("ffmpeg"), "-hide_banner", "-encoders")
	cmd.Env = runtimeenv.CommandEnv(s.toolBundleRoot(cmd.Path))
	configureProcess(cmd)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var mpeg2, ac3 bool
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mpeg2 = mpeg2 || fields[1] == "mpeg2video"
			ac3 = ac3 || fields[1] == "ac3"
		}
	}
	if err != nil || !mpeg2 || !ac3 {
		return classified(ctx, ErrDependencies, "DVD-Video requires FFmpeg with MPEG-2 video and AC-3 encoders")
	}
	return nil
}

func (s *Engine) runDVDVideo(ctx context.Context, p Project, e Estimate, dir string) error {
	standard, height, fps, gop := "pal", 576, "25", "15"
	if p.VideoStandard == "ntsc" {
		standard, height, fps, gop = "ntsc", 480, "30000/1001", "18"
	}
	rate := e.VideoBitrate
	staging := filepath.Join(dir, "dvd-work")
	disc := filepath.Join(dir, "disc")
	partial := filepath.Join(dir, "dvd-video.partial.iso")
	output := filepath.Join(dir, "dvd-video.iso")
	for attempt := 1; attempt <= 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Mkdir(staging, 0700); err != nil {
			return err
		}
		width := dvdVideoWidth(rate)
		dar := "16/9"
		if width == 352 {
			dar = "4/3"
		}
		w, h := strconv.Itoa(width), strconv.Itoa(height)
		sar := "(" + dar + "*" + h + "/" + w + ")"
		filter := "scale=w='max(2,trunc(min(" + w + "," + h + "*dar/" + sar + ")/2)*2)':h='max(2,trunc(min(" + h + "," + w + "*" + sar + "/dar)/2)*2)',setsar=" + sar + ",pad=" + w + ":" + h + ":(ow-iw)/2:(oh-ih)/2,setdar=" + dar + ",fps=" + fps + ",format=yuv420p"
		s.appendLog(text(ctx, "DVD-Video attempt %d: %dx%d at %.2f Mb/s (two passes)", attempt, width, height, float64(rate)/1e6))
		// Generated ASCII names avoid encoding/XML escaping issues with source paths.
		var xml strings.Builder
		fmt.Fprintf(&xml, `<dvdauthor dest="../disc"><vmgm><fpc>jump title 1;</fpc><menus><video format="%s"/></menus></vmgm><titleset><titles><video format="%s" aspect="%s"/><pgc>`, standard, standard, strings.ReplaceAll(dar, "/", ":"))
		for i, path := range p.Groups[0].Tracks {
			f, err := s.dvdVideoClip(ctx, path)
			if err != nil {
				return err
			}
			full, err := s.source(ctx, path)
			if err != nil {
				return err
			}
			name := fmt.Sprintf("clip-%02d", i+1)
			duration := strconv.FormatFloat(f.Duration, 'f', 6, 64)
			ff := []string{"-nostdin", "-hide_banner", "-loglevel", "warning", "-y", "-i", full}
			video := []string{"-map", "0:V:0", "-sn", "-dn", "-map_metadata", "-1", "-vf", filter + ",tpad=stop_mode=clone:stop_duration=" + duration,
				"-c:v", "mpeg2video", "-b:v", strconv.FormatInt(rate, 10), "-maxrate", "9000k", "-minrate", "0", "-bufsize", "1835k", "-g", gop, "-bf", "2", "-t", duration, "-passlogfile", name}
			s.appendLog(text(ctx, "Group %d / track %d: %s", 1, i+1, path))
			pass1 := append(append([]string{}, ff...), video...)
			if err := s.command(ctx, staging, s.toolPath("ffmpeg"), append(pass1, "-pass", "1", "-an", "-f", "null", "-")...); err != nil {
				return err
			}
			audio := "0:a:0"
			if f.Channels == 0 {
				ff = append(ff, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
				audio = "1:a:0"
			}
			ff = append(ff, video...)
			ff = append(ff, "-pass", "2", "-map", audio, "-c:a", "ac3", "-b:a", "192k", "-ar", "48000", "-ac", "2", "-af", "apad", "-f", "dvd", "-packetsize", "2048", "-muxrate", "10080000", name+".mpg")
			if err := s.command(ctx, staging, s.toolPath("ffmpeg"), ff...); err != nil {
				return err
			}
			fmt.Fprintf(&xml, `<vob file="%s.mpg" chapters="0"/>`, name)
		}
		xml.WriteString(`</pgc></titles></titleset></dvdauthor>`)
		if err := os.WriteFile(filepath.Join(staging, "author.xml"), []byte(xml.String()), 0600); err != nil {
			return err
		}
		if err := s.command(ctx, staging, s.toolPath("dvdauthor"), "-x", "author.xml"); err != nil {
			return err
		}
		for _, name := range []string{"VIDEO_TS.IFO", "VIDEO_TS.BUP", "VTS_01_0.IFO", "VTS_01_1.VOB"} {
			if info, err := os.Stat(filepath.Join(disc, "VIDEO_TS", name)); err != nil || info.Size() == 0 {
				return errorf(ctx, "Authoring did not create a valid %s file", name)
			}
		}
		if err := os.MkdirAll(filepath.Join(disc, "AUDIO_TS"), 0700); err != nil {
			return err
		}
		if err := s.command(ctx, dir, s.toolPath("dvd-video-mkisofs"), "-dvd-video", "-udf", "-V", "DVD_VIDEO", "-o", partial, disc); err != nil {
			return err
		}
		info, err := os.Stat(partial)
		if err != nil {
			return err
		}
		if info.Size() > 0 && info.Size() <= e.CapacityBytes && info.Size()%2048 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.Rename(partial, output); err != nil {
				return err
			}
			s.mu.Lock()
			s.job.Output = output
			s.mu.Unlock()
			s.appendLog(text(ctx, "Completed: ") + output)
			return nil
		}
		if info.Size() <= e.CapacityBytes {
			return errorf(ctx, "DVD-Video ISO is empty or invalid")
		}
		if rate == dvdVideoMinRate {
			break
		}
		if attempt == 3 {
			return errorf(ctx, "DVD-Video ISO still exceeds capacity after re-encoding")
		}
		// Adjust by the measured overshoot, retaining a margin for the next mux.
		s.appendLog(text(ctx, "DVD-Video ISO exceeded capacity; retrying with a lower bitrate"))
		rate = max(dvdVideoMinRate, int64(float64(rate+dvdVideoAudioRate)*float64(e.CapacityBytes)*0.96/float64(info.Size())-dvdVideoAudioRate)/1000*1000)
		for _, artifact := range []string{partial, disc, staging} {
			if err := os.RemoveAll(artifact); err != nil {
				return err
			}
		}
	}
	return errorf(ctx, "DVD-Video cannot fit at the minimum supported quality")
}
