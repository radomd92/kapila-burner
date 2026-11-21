//go:build gtk

package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dvd-audio-studio/internal/core"
)

func TestLocalizedDefaults(t *testing.T) {
	for _, lang := range []string{"en", "plt"} {
		p := newProject(lang)
		if p.SampleRate != 44100 || p.Bits != 16 || p.Channels != 2 || p.Capacity != "dvd5" || !p.ISO {
			t.Fatalf("%s: incorrect profile: %+v", lang, p)
		}
		if p.Name != translate(lang, "Untitled project", "Tetikasa tsy misy anarana") || p.Groups[0].Name != groupName(lang, 1) {
			t.Fatalf("%s: names not localized", lang)
		}
		if hasTracks(p) {
			t.Fatal("empty project has tracks")
		}
		p.Groups = append(p.Groups, core.Group{Name: groupName("plt", 2), Tracks: []string{"story.wav"}})
		if !hasTracks(p) {
			t.Fatal("missed tracks in another group")
		}
	}
	if groupName("plt", 2) != "Vondrona 2" || groupName("en", 2) != "Group 2" {
		t.Fatal("incorrect group names")
	}
}

func TestLocalizedProjectErrors(t *testing.T) {
	for _, message := range []string{
		"project JSON exceeds 2 MiB", "refusing to overwrite a non-regular file", "cannot export over a source track",
		"expected a single project JSON object", "use 1-9 groups", "project name must contain 1-200 bytes",
		"invalid PCM profile or media", "empty track path",
	} {
		err := errors.New(message)
		if got := errorText("plt", err); strings.Contains(got, message) {
			t.Fatalf("untranslated validation: %s", got)
		}
		if got := errorText("en", err); !strings.Contains(got, message) {
			t.Fatalf("lost English diagnostic: %s", got)
		}
	}
	if got := errorText("plt", errors.New("My group: at most 99 tracks per group")); got != "My group: raki-peo 99 fara-fahabetsany isaky ny vondrona" {
		t.Fatal(got)
	}
	if got := errorText("plt", errors.New("invalid character '<' looking for beginning of value")); got != "Hadisoana: invalid character '<' looking for beginning of value" {
		t.Fatal(got)
	}
}

func TestProjectEditing(t *testing.T) {
	p := newProject("en")
	g := &p.Groups[0]
	if !appendTracks(g, []string{"a.wav", "b.wav", "a.wav"}) {
		t.Fatal("append failed")
	}
	if got := moveTrack(g, 1, -1); got != 0 {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(g.Tracks, []string{"b.wav", "a.wav", "a.wav"}) {
		t.Fatal(g.Tracks)
	}
	if moveTrack(g, 0, -1) != 0 || moveTrack(g, -1, 1) != -1 {
		t.Fatal("boundary move")
	}
	copy := cloneProject(p)
	copy.Groups[0].Tracks[0] = "changed"
	if p.Groups[0].Tracks[0] != "b.wav" {
		t.Fatal("clone aliases tracks")
	}
	if appendTracks(g, make([]string, 99)) || len(g.Tracks) != 3 {
		t.Fatal("track limit not atomic")
	}
}

func TestProjectJSON(t *testing.T) {
	p := newProject("en")
	p.Groups[0].Tracks = []string{"<title>.wav", "a.wav", "a.wav"}
	b, _ := json.Marshal(p)
	got, err := decodeProject(strings.NewReader(string(b)))
	if err != nil || !reflect.DeepEqual(p, got) {
		t.Fatalf("round trip: %v %v", got, err)
	}
	for _, input := range []string{string(b) + " {}", `{"unknown":1}`, `null`, string(b[:len(b)-1]) + `,"other":true}`} {
		if _, err := decodeProject(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestExportSafety(t *testing.T) {
	dir := t.TempDir()
	p := newProject("en")
	source := filepath.Join(dir, "source.json")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	p.Groups[0].Tracks = []string{"source.json"}
	if err := exportProject(source, dir, p); err == nil {
		t.Fatal("overwrote source")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if err := exportProject(link, dir, p); err == nil {
		t.Fatal("followed symlink")
	}
	hard := filepath.Join(dir, "hard.json")
	if err := os.Link(source, hard); err != nil {
		t.Fatal(err)
	}
	if err := exportProject(hard, dir, p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(source)
	if string(data) != "original" {
		t.Fatal("modified hard-linked original")
	}
	got, err := readProject(hard)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("export: %v %v", got, err)
	}
}

func TestDiscFormats(t *testing.T) {
	for _, format := range []string{"", "dvd-audio", "vcd"} {
		p := newProject("en")
		p.Format = format
		if format == "vcd" {
			p.Capacity = "cd80"
			p.VideoStandard = "pal"
		}
		b, _ := json.Marshal(p)
		got, err := decodeProject(strings.NewReader(string(b)))
		if err != nil || !reflect.DeepEqual(p, got) {
			t.Fatalf("%s round trip: %+v %v", format, got, err)
		}
	}
	p := newProject("en")
	p.Format = "vcd"
	p.Capacity = "cd74"
	for _, mutate := range []func(*core.Project){
		func(p *core.Project) { p.Format = "svcd" },
		func(p *core.Project) { p.Capacity = "dvd5" },
		func(p *core.Project) { p.VideoStandard = "secam" },
		func(p *core.Project) { p.Groups = append(p.Groups, core.Group{}) },
		func(p *core.Project) { p.Groups[0].Tracks = make([]string, 99) },
	} {
		bad := cloneProject(p)
		mutate(&bad)
		b, _ := json.Marshal(bad)
		if _, err := decodeProject(strings.NewReader(string(b))); err == nil {
			t.Fatalf("accepted invalid VCD: %+v", bad)
		}
	}
	p.Groups[0].Tracks = make([]string, 98)
	for i := range p.Groups[0].Tracks {
		p.Groups[0].Tracks[i] = "duplicate.wav"
	}
	b, _ := json.Marshal(p)
	if _, err := decodeProject(strings.NewReader(string(b))); err != nil {
		t.Fatalf("rejected 98 tracks/default PAL: %v", err)
	}
}

func TestBurnEligibility(t *testing.T) {
	p := newProject("en")
	a := &app{active: true, project: p, authorProject: cloneProject(p), authorJobID: "author-image"}
	if a.burnReady() {
		t.Fatal("empty session can burn")
	}
	for _, job := range []core.Job{
		{State: "running", Format: "vcd", Kind: "author", Output: "image.cue"},
		{State: "failed", Format: "vcd", Kind: "author", Output: "image.cue"},
		{State: "completed", Format: "dvd-audio", Kind: "author", Output: "image.iso"},
		{State: "completed", Format: "vcd", Kind: "burn", Output: "image.cue"},
		{State: "completed", Format: "vcd", Kind: "author", Output: "image.bin"},
	} {
		job.ID = a.authorJobID
		a.lastJob = &job
		a.rememberAuthoredImage(&job)
		if a.burnReady() {
			t.Fatalf("unsafe burn eligibility: %+v", job)
		}
	}
	a.lastJob = &core.Job{ID: a.authorJobID, State: "completed", Format: "vcd", Kind: "author", Output: "image.cue"}
	a.rememberAuthoredImage(a.lastJob)
	if !a.burnReady() {
		t.Fatal("completed VCD author image not eligible")
	}
}

func TestBurnSourceLifecycle(t *testing.T) {
	p := newProject("en")
	p.Format = "vcd"
	p.Capacity = "cd80"
	p.Groups[0].Tracks = []string{"source.wav"}
	a := &app{active: true, project: p, authorProject: cloneProject(p), authorJobID: "original-author"}
	author := &core.Job{ID: "original-author", State: "completed", Format: "vcd", Kind: "author", Output: "original.cue", Log: []string{"authored"}}
	a.rememberAuthoredImage(author)
	if !a.burnReady() {
		t.Fatal("completed author did not supply source")
	}
	author.Log[0] = "changed"
	if a.burnSource.Log[0] != "authored" {
		t.Fatal("source snapshot aliases author log")
	}
	for _, state := range []string{"running", "completed", "failed", "cancelled", "completed"} {
		// Completed dummy and real burns have the same Job representation.
		a.lastJob = &core.Job{ID: "burn-job", State: state, Format: "vcd", Kind: "burn", Output: "original.cue"}
		a.rememberAuthoredImage(a.lastJob)
		if !a.burnReady() || a.burnSource.ID != "original-author" || a.burnSource.Output != "original.cue" {
			t.Fatalf("%s burn lost source", state)
		}
		request, err := makeBurnRequest(a.burnSource.ID, "/dev/synthetic", "8", false)
		if err != nil || request.JobID != "original-author" {
			t.Fatalf("retry used burn job ID: %+v %v", request, err)
		}
	}
	a.project.Groups[0].Tracks[0] = "different.wav"
	if a.burnReady() {
		t.Fatal("edited document can burn old image")
	}
	a.project = cloneProject(a.burnProject)
	if !a.burnReady() {
		t.Fatal("restoring authored document did not restore eligibility")
	}
	a.authorJobID = "new-author"
	a.rememberAuthoredImage(&core.Job{ID: "new-author", State: "failed", Kind: "author", Format: "vcd"})
	if !a.burnReady() {
		t.Fatal("failed reauthor lost prior valid source")
	}
	a.rememberAuthoredImage(&core.Job{ID: "new-author", State: "completed", Kind: "author", Format: "dvd-audio", Output: "new.iso"})
	if a.burnReady() || a.burnSource != nil {
		t.Fatal("successful DVD author kept VCD source")
	}
	a.authorJobID = ""
	a.rememberAuthoredImage(author)
	if a.burnReady() || a.burnSource != nil {
		t.Fatal("old snapshot associated with new document")
	}
}

func TestBurnRequest(t *testing.T) {
	for _, speed := range []string{"1", "2", "4", "8", "16", "24", "48", "52"} {
		request, err := makeBurnRequest("image-job", "/dev/synthetic-recorder", speed, true)
		if err != nil || request.Speed < 1 || request.Speed > 52 || request.JobID != "image-job" || request.Device != "/dev/synthetic-recorder" || !request.Dummy || request.Confirmed {
			t.Fatalf("invalid request or premature confirmation: %+v %v", request, err)
		}
	}
	for _, speed := range []string{"", "0", "-1", "53", "8x", "automatic"} {
		if _, err := makeBurnRequest("image-job", "/dev/synthetic-recorder", speed, false); err == nil {
			t.Fatalf("accepted invalid speed %q", speed)
		}
	}
	for _, missing := range [][2]string{{"", "/dev/synthetic-recorder"}, {"image-job", ""}, {"image-job", " "}} {
		if _, err := makeBurnRequest(missing[0], missing[1], "8", false); err == nil {
			t.Fatalf("accepted missing job/device: %v", missing)
		}
	}
}
