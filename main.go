package main

import (
	"bytes"
	"context"
	"dvd-audio-studio/internal/core"
	"dvd-audio-studio/internal/i18n"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed web/index.html
var web embed.FS

type webAdapter struct{ engine *core.Engine }

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "web" {
		args = args[1:]
	}
	flags := flag.NewFlagSet("web", flag.ExitOnError)
	root := flags.String("root", "..", "Audio library root directory")
	output := flags.String("output", "./output", "Authoring output directory")
	listen := flags.String("listen", "127.0.0.1:8787", "Loopback listen address")
	_ = flags.Parse(args)
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || (host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback())) {
		log.Fatal("-listen must use a loopback address")
	}
	app, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	engine, err := core.New(core.Config{Root: *root, Output: *output, App: app})
	if err != nil {
		log.Fatal(err)
	}
	defer engine.Close()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: (&webAdapter{engine: engine}).handler(ln.Addr().String()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-stop.Done()
		engine.Close()
		ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("DVD-Audio Studio: http://%s\nLibrary: %s\nOutput: %s", ln.Addr(), engine.Root(), engine.Output())
	if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	respond(w, status, map[string]string{"error": err.Error()})
}
func decodeProject(w http.ResponseWriter, r *http.Request) (core.Project, error) {
	var p core.Project
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, errors.New(i18n.Text(i18n.Language(r.Context()), "Invalid project JSON: %s", err))
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return p, errors.New(i18n.Text(i18n.Language(r.Context()), "Expected a single project JSON object"))
	}
	return p, nil
}
func (s *webAdapter) handler(address string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := web.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /api/library", func(w http.ResponseWriter, r *http.Request) {
		files, err := s.engine.Library(r.Context())
		if err != nil {
			fail(w, 500, err)
			return
		}
		respond(w, 200, map[string]any{"root": s.engine.Root(), "files": files})
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"tools": s.engine.Tools(r.Context()), "job": s.engine.Snapshot()})
	})
	mux.HandleFunc("POST /api/estimate", func(w http.ResponseWriter, r *http.Request) {
		p, err := decodeProject(w, r)
		if err != nil {
			fail(w, 400, err)
			return
		}
		e, err := s.engine.Estimate(r.Context(), p)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respond(w, 200, e)
	})
	mux.HandleFunc("POST /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		p, err := decodeProject(w, r)
		if err != nil {
			fail(w, 400, err)
			return
		}
		job, err := s.engine.Start(r.Context(), p)
		if err != nil {
			status := 400
			if errors.Is(err, core.ErrBusy) {
				status = 409
			}
			if errors.Is(err, core.ErrDependencies) {
				status = 503
			}
			fail(w, status, err)
			return
		}
		respond(w, 202, job)
	})
	mux.HandleFunc("POST /api/cancel", func(w http.ResponseWriter, r *http.Request) {
		if err := s.engine.Cancel(); err != nil {
			fail(w, 409, errors.New(i18n.Text(i18n.Language(r.Context()), "No job is running")))
			return
		}
		respond(w, 202, map[string]string{"status": "cancellation requested"})
	})
	mux.HandleFunc("GET /api/download", func(w http.ResponseWriter, r *http.Request) {
		job := s.engine.Snapshot()
		if job == nil || job.ID != r.URL.Query().Get("id") || job.State != "completed" || job.Error != "" || (job.Kind != "" && job.Kind != "author") {
			http.NotFound(w, r)
			return
		}
		path := job.Output
		file := strings.TrimPrefix(r.URL.Query().Get("file"), ".")
		name := "dvd-audio.iso"
		switch filepath.Ext(path) {
		case ".iso":
			if file != "" && file != "iso" {
				http.NotFound(w, r)
				return
			}
		case ".cue":
			if filepath.Base(path) != "disc.cue" || (file != "" && file != "cue" && file != "bin") {
				http.NotFound(w, r)
				return
			}
			name = "disc.cue"
			if file == "bin" {
				name = "disc.bin"
			}
			path = filepath.Join(filepath.Dir(path), name)
		default:
			http.NotFound(w, r)
			return
		}
		// Only direct artifacts of this job may be served, never symlink targets elsewhere.
		dir := filepath.Dir(job.Output)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || filepath.Dir(dir) != s.engine.Output() || filepath.Dir(resolved) != dir {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(resolved)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		var content io.ReadSeeker = f
		if name == "disc.cue" {
			data, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
			if err != nil || len(data) > 2<<20 {
				http.NotFound(w, r)
				return
			}
			// vcdimager may embed its absolute -b path; downloaded pairs must be portable.
			cue := strings.ReplaceAll(string(data), `FILE "`+filepath.Join(dir, "disc.bin")+`" BINARY`, `FILE "disc.bin" BINARY`)
			first, rest, _ := strings.Cut(cue, "\n")
			if strings.TrimSuffix(first, "\r") != `FILE "disc.bin" BINARY` || strings.Contains(rest, "FILE ") {
				http.NotFound(w, r)
				return
			}
			content = bytes.NewReader([]byte(cue))
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		http.ServeContent(w, r, name, info.ModTime(), content)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		language := r.URL.Query().Get("lang")
		if !r.URL.Query().Has("lang") {
			language = r.Header.Get("Accept-Language")
		}
		if strings.TrimSpace(language) == "" {
			language = "plt"
		}
		r = r.WithContext(core.WithLanguage(r.Context(), language))
		// Reject DNS rebinding and cross-origin browser requests, including localhost aliases.
		if r.Host != address {
			fail(w, 403, errors.New(i18n.Text(i18n.Language(r.Context()), "Use the exact loopback URL shown by the server")))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || u.Host != r.Host {
				fail(w, 403, errors.New(i18n.Text(i18n.Language(r.Context()), "Cross-origin requests are not allowed")))
				return
			}
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			fail(w, 403, errors.New(i18n.Text(i18n.Language(r.Context()), "Cross-site requests are not allowed")))
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		mux.ServeHTTP(w, r)
	})
}
