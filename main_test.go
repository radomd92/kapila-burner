package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dvd-audio-studio/internal/core"
)

func testAdapter(t *testing.T) *webAdapter {
	t.Helper()
	root := t.TempDir()
	engine, err := core.New(core.Config{Root: root, Output: filepath.Join(root, "output"), App: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)
	return &webAdapter{engine: engine}
}
func TestEmbeddedUI(t *testing.T) {
	page, err := web.ReadFile("web/index.html")
	if err != nil || len(page) == 0 {
		t.Fatalf("embedded UI: %v", err)
	}
}

func TestHTTPContractAndLanguage(t *testing.T) {
	const address = "127.0.0.1:8787"
	// Missing dependencies must remain HTTP 503, including a missing ffprobe.
	for _, variable := range []string{"FFMPEG", "FFPROBE", "DVDA_AUTHOR", "MKISOFS"} {
		t.Setenv(variable, filepath.Join(t.TempDir(), "missing"))
	}
	s := testAdapter(t)
	if err := os.WriteFile(filepath.Join(s.engine.Root(), "a.wav"), []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	h := s.handler(address)
	for _, tc := range []struct {
		path, body, language, want string
		status                     int
	}{
		{"/api/estimate", `{}`, "", "anaran'ny tetikasa", 400},
		{"/api/estimate?lang=fr", `{}`, "", "nom du projet", 400},
		{"/api/estimate?lang=plt", `{}`, "", "anaran'ny tetikasa", 400},
		{"/api/estimate?lang=en", `{}`, "plt", "Project name", 400},
		{"/api/estimate", `{}`, "mg-MG,en;q=0.5", "anaran'ny tetikasa", 400},
		{"/api/estimate?lang=plt", `{} {}`, "", "Zavatra JSON", 400},
		{"/api/estimate?lang=plt", `{"unknown":1}`, "", "Tsy manankery", 400},
		{"/api/cancel?lang=plt", ``, "", "Tsy misy asa", 409},
		{"/api/jobs?lang=plt", `{"name":"Disc","sampleRate":48000,"bits":16,"channels":2,"capacity":"dvd5","iso":false,"groups":[{"name":"Group","tracks":["a.wav"]}]}`, "", "Tsy azo ampiasaina", 503},
	} {
		t.Run(tc.path+tc.body+tc.language, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+address+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Accept-Language", tc.language)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://"+address+"/api/status?lang=plt", nil))
	var status struct {
		Tools []core.Tool `json:"tools"`
		Job   *core.Job   `json:"job"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.Job != nil {
		t.Fatalf("status schema: %s, %v", w.Body.String(), err)
	}
	for _, name := range []string{"ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager", "cdrdao"} {
		found := false
		for _, tool := range status.Tools {
			found = found || tool.Name == name
		}
		if !found {
			t.Errorf("status missing tool %q: %+v", name, status.Tools)
		}
	}
	if !strings.Contains(status.Tools[0].Detail, "Tsy nahomby") {
		t.Fatalf("tool language: %+v", status.Tools)
	}
}

func TestDownloadRequiresCurrentCompletedJob(t *testing.T) {
	const address = "127.0.0.1:8787"
	h := testAdapter(t).handler(address)
	for _, query := range []string{"", "id=old", "id=old&file=cue", "id=old&file=bin", "id=old&file=../../etc/passwd"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://"+address+"/api/download?"+query, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("download %q: status %d", query, w.Code)
		}
	}
}

func TestVCDDownloadIntegration(t *testing.T) {
	if os.Getenv("VCD_INTEGRATION") != "1" {
		t.Skip("set VCD_INTEGRATION=1 to run installed ffmpeg, ffprobe and vcdimager")
	}
	const address = "127.0.0.1:8787"
	s := testAdapter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=5", filepath.Join(s.engine.Root(), "a.wav"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate WAV: %v: %s", err, output)
	}
	p := core.Project{Name: "VCD", Format: "vcd", VideoStandard: "pal", Capacity: "cd80", Groups: []core.Group{{Name: "Group", Tracks: []string{"a.wav"}}}}
	started, err := s.engine.Start(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	h := s.handler(address)
	request := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://"+address+"/api/download?"+query, nil))
		return w
	}
	if w := request("id=" + started.ID + "&file=cue"); w.Code != 404 {
		t.Fatalf("unfinished download: %d", w.Code)
	}
	for {
		job := s.engine.Snapshot()
		if job.State == "completed" {
			break
		}
		if job.State == "failed" || job.State == "cancelled" || ctx.Err() != nil {
			t.Fatalf("authoring: %+v", job)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, file := range []string{"cue", "bin", ".cue", ".bin"} {
		w := request("id=" + started.ID + "&file=" + file)
		name := "disc." + strings.TrimPrefix(file, ".")
		if w.Code != 200 || w.Body.Len() == 0 || w.Header().Get("Content-Disposition") != `attachment; filename="`+name+`"` {
			t.Fatalf("%s download: %d, %v", file, w.Code, w.Header())
		}
		if name == "disc.cue" && !strings.Contains(w.Body.String(), `FILE "disc.bin" BINARY`) {
			t.Fatalf("CUE must reference downloaded relative disc.bin: %s", w.Body.String())
		}
	}
	for _, query := range []string{"id=old&file=cue", "id=" + started.ID + "&file=../../disc.bin", "id=" + started.ID + "&file=iso"} {
		if w := request(query); w.Code != 404 {
			t.Errorf("invalid download %q: %d", query, w.Code)
		}
	}
	if w := request("id=" + started.ID); w.Code != 200 || !strings.Contains(w.Body.String(), `FILE "disc.bin" BINARY`) {
		t.Fatalf("default CUE download: %d: %s", w.Code, w.Body.String())
	}
	bin := filepath.Join(filepath.Dir(s.engine.Snapshot().Output), "disc.bin")
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(s.engine.Root(), "a.wav"), bin); err != nil {
		t.Fatal(err)
	}
	if w := request("id=" + started.ID + "&file=bin"); w.Code != 404 {
		t.Fatalf("symlink escape download: %d", w.Code)
	}
	if _, err := s.engine.Start(ctx, p); err != nil {
		t.Fatal(err)
	}
	if w := request("id=" + started.ID + "&file=cue"); w.Code != 404 {
		t.Fatalf("non-current completed job download: %d", w.Code)
	}
}

func TestDecodeVCDProject(t *testing.T) {
	const body = `{"name":"VCD","format":"vcd","videoStandard":"ntsc","sampleRate":0,"bits":0,"channels":0,"capacity":"cd74","iso":false,"groups":[{"name":"Group","tracks":["a.wav"]}]}`
	r := httptest.NewRequest("POST", "/api/jobs", strings.NewReader(body))
	p, err := decodeProject(httptest.NewRecorder(), r)
	if err != nil || p.Format != "vcd" || p.VideoStandard != "ntsc" || p.Capacity != "cd74" || p.SampleRate != 0 {
		t.Fatalf("VCD decode: %+v, %v", p, err)
	}
	data, err := json.Marshal(p)
	if err != nil || !strings.Contains(string(data), `"format":"vcd"`) || !strings.Contains(string(data), `"videoStandard":"ntsc"`) {
		t.Fatalf("VCD round trip: %s, %v", data, err)
	}
}
func TestHandlerGuardsAndMethods(t *testing.T) {
	const address = "127.0.0.1:8787"
	h := testAdapter(t).handler(address)
	for _, tc := range []struct {
		name, method, path, host, origin, site string
		status                                 int
	}{
		{"allowed", "GET", "/api/status", address, "http://" + address, "same-origin", 200},
		{"no origin", "GET", "/api/status", address, "", "none", 200},
		{"host alias", "GET", "/api/status", "localhost:8787", "", "", 403},
		{"host attacker", "GET", "/api/status", "evil.test:8787", "", "", 403},
		{"origin alias", "POST", "/api/estimate", address, "http://localhost:8787", "", 403},
		{"origin port", "GET", "/api/status", address, "http://127.0.0.1:9999", "", 403},
		{"origin scheme", "GET", "/api/status", address, "https://" + address, "", 403},
		{"origin null", "GET", "/api/status", address, "null", "", 403},
		{"origin malformed", "GET", "/api/status", address, "%", "", 403},
		{"cross site", "GET", "/api/status", address, "", "cross-site", 403},
		{"same site", "GET", "/api/status", address, "", "same-site", 403},
		{"status method", "POST", "/api/status", address, "", "", 405},
		{"library method", "POST", "/api/library", address, "", "", 405},
		{"estimate method", "PUT", "/api/estimate", address, "", "", 405},
		{"jobs method", "PUT", "/api/jobs", address, "", "", 405},
		{"cancel method", "PUT", "/api/cancel", address, "", "", 405},
		// The catch-all GET / currently masks method errors on POST-only routes.
		{"estimate GET fallback", "GET", "/api/estimate", address, "", "", 404},
		{"jobs GET fallback", "GET", "/api/jobs", address, "", "", 404},
		{"cancel GET fallback", "GET", "/api/cancel", address, "", "", 404},
		{"download method", "POST", "/api/download", address, "", "", 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://"+address+tc.path, nil)
			r.Host = tc.host
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == http.StatusMethodNotAllowed && w.Header().Get("Allow") == "" {
				t.Error("method error missing Allow header")
			}
		})
	}
}
