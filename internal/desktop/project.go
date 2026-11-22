//go:build gtk

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dvd-audio-studio/internal/core"
)

const (
	dvdVideo               = "dvd-video"
	invalidDVDVideoProfile = "Invalid DVD Video profile: use one group, DVD-5/DVD-9 and PAL/NTSC"
	tooManyDVDVideoClips   = "DVD Video supports at most 99 clips"
)

func newProject(lang string) core.Project {
	return core.Project{Name: translate(lang, "Untitled project", "Tetikasa tsy misy anarana"), SampleRate: 44100, Bits: 16, Channels: 2, Capacity: "dvd5", ISO: true, Groups: []core.Group{{Name: groupName(lang, 1), Tracks: []string{}}}}
}

func groupName(lang string, number int) string {
	return fmt.Sprintf(translate(lang, "Group %d", "Vondrona %d"), number)
}

func translate(lang, en, mg string) string {
	if lang == "fr" {
		if text, ok := french[en]; ok {
			return text
		}
	}
	if lang == "plt" {
		return mg
	}
	return en
}

func errorText(lang string, err error) string {
	message := err.Error()
	if lang == "fr" {
		if text, ok := french[message]; ok {
			return text
		}
		if group, ok := strings.CutSuffix(message, ": at most 99 tracks per group"); ok {
			return group + ": " + french["At most 99 tracks per group"]
		}
	}
	if lang == "plt" {
		messages := map[string]string{
			"Write speed must be between 1x and 52x":                          "Tsy maintsy eo anelanelan'ny 1x sy 52x ny hafainganam-panoratana",
			"Select a completed image and recorder first":                     "Misafidiana sary kapila vita sy mpanoratra kapila aloha",
			"project JSON exceeds 2 MiB":                                      "Mihoatra ny 2 MiB ny JSON an'ny tetikasa",
			"refusing to overwrite a non-regular file":                        "Tsy azo soloina ny rakitra tsy mahazatra",
			"cannot export over a source track":                               "Tsy azo soloina amin'ny fanondranana ny raki-peo loharano",
			"expected a single project JSON object":                           "Zavatra JSON iray ho an'ny tetikasa ihany no andrasana",
			"use 1-9 groups":                                                  "Mampiasao vondrona 1-9",
			"project name must contain 1-200 bytes":                           "Tsy maintsy misy oktety 1-200 ny anaran'ny tetikasa",
			"invalid PCM profile or media":                                    "Tsy mety ny safidy PCM na kapila",
			"empty track path":                                                "Foana ny lalan'ny raki-peo",
			"unknown project format":                                          "Tsy fantatra ny karazana tetikasa",
			"invalid Video CD profile: use one group, CD74/CD80 and PAL/NTSC": "Tsy mety ny safidy Video CD: vondrona iray, CD74/CD80 ary PAL/NTSC no ampiasao",
			"Video CD supports at most 98 tracks":                             "Raki-peo 98 fara-fahabetsany ho an'ny Video CD",
			invalidDVDVideoProfile:                                            "Tsy mety ny safidy DVD Video: vondrona iray, DVD-5/DVD-9 ary PAL/NTSC no ampiasao",
			tooManyDVDVideoClips:                                              "Horonantsary 99 fara-fahabetsany ho an'ny DVD Video",
		}
		if translated, ok := messages[message]; ok {
			return translated
		}
		if group, ok := strings.CutSuffix(message, ": at most 99 tracks per group"); ok {
			return group + ": raki-peo 99 fara-fahabetsany isaky ny vondrona"
		}
	}
	return translate(lang, "Error: ", "Hadisoana: ") + message
}

func fixedGroup(format string) bool { return format == "vcd" || format == dvdVideo }

func hasTracks(p core.Project) bool {
	for _, group := range p.Groups {
		if len(group.Tracks) > 0 {
			return true
		}
	}
	return false
}

func cloneProject(p core.Project) core.Project {
	p.Groups = slices.Clone(p.Groups)
	for i := range p.Groups {
		p.Groups[i].Tracks = slices.Clone(p.Groups[i].Tracks)
	}
	return p
}

// Parsing checks structure without probing media; core validates sources and capacity.
func readProject(path string) (core.Project, error) {
	f, err := os.Open(path)
	if err != nil {
		return core.Project{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return core.Project{}, err
	}
	if st.Size() > 2<<20 {
		return core.Project{}, errors.New("project JSON exceeds 2 MiB")
	}
	return decodeProject(io.LimitReader(f, (2<<20)+1))
}

func exportProject(path, root string, p core.Project) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	path = filepath.Join(dir, filepath.Base(path))
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return errors.New("refusing to overwrite a non-regular file")
	}
	for _, g := range p.Groups {
		for _, track := range g.Tracks {
			source := track
			if !filepath.IsAbs(source) {
				source = filepath.Join(root, source)
			}
			if resolved, err := filepath.EvalSymlinks(source); err == nil && resolved == path {
				return errors.New("cannot export over a source track")
			}
		}
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	// Atomic replacement never truncates an existing source through a hard link.
	f, err := os.CreateTemp(dir, ".dvd-audio-project-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func decodeProject(r io.Reader) (core.Project, error) {
	var p core.Project
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return p, errors.New("expected a single project JSON object")
	}
	if p.Format != dvdVideo && (len(p.Groups) < 1 || len(p.Groups) > 9) {
		return p, errors.New("use 1-9 groups")
	}
	if len(strings.TrimSpace(p.Name)) == 0 || len(p.Name) > 200 {
		return p, errors.New("project name must contain 1-200 bytes")
	}
	switch p.Format {
	case "vcd":
		if len(p.Groups) != 1 || (p.Capacity != "cd74" && p.Capacity != "cd80") || (p.VideoStandard != "" && p.VideoStandard != "pal" && p.VideoStandard != "ntsc") {
			return p, errors.New("invalid Video CD profile: use one group, CD74/CD80 and PAL/NTSC")
		}
	case dvdVideo:
		if p.VideoStandard == "" {
			p.VideoStandard = "pal"
		}
		if len(p.Groups) != 1 || (p.Capacity != "dvd5" && p.Capacity != "dvd9") || (p.VideoStandard != "pal" && p.VideoStandard != "ntsc") {
			return p, errors.New(invalidDVDVideoProfile)
		}
		p.ISO = true
	case "", "dvd-audio":
		if !slices.Contains([]int{44100, 48000, 88200, 96000, 176400, 192000}, p.SampleRate) || (p.Bits != 16 && p.Bits != 24) || (p.Channels != 1 && p.Channels != 2) || (p.Capacity != "dvd5" && p.Capacity != "dvd9") {
			return p, errors.New("invalid PCM profile or media")
		}
	default:
		return p, errors.New("unknown project format")
	}
	for _, g := range p.Groups {
		if p.Format == "vcd" && len(g.Tracks) > 98 {
			return p, errors.New("Video CD supports at most 98 tracks")
		}
		if len(g.Tracks) > 99 {
			if p.Format == dvdVideo {
				return p, errors.New(tooManyDVDVideoClips)
			}
			return p, fmt.Errorf("%s: at most 99 tracks per group", g.Name)
		}
		for _, t := range g.Tracks {
			if t == "" {
				return p, errors.New("empty track path")
			}
		}
	}
	return p, nil
}

// Selected indices are applied in source order, not mouse-click order.
func appendTracks(g *core.Group, paths []string) bool {
	if len(g.Tracks)+len(paths) > 99 {
		return false
	}
	g.Tracks = append(g.Tracks, paths...)
	return true
}

func moveTrack(g *core.Group, index, delta int) int {
	next := index + delta
	if index < 0 || index >= len(g.Tracks) || next < 0 || next >= len(g.Tracks) {
		return index
	}
	g.Tracks[index], g.Tracks[next] = g.Tracks[next], g.Tracks[index]
	return next
}
