//go:build gtk

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"dvd-audio-studio/internal/core"
	"dvd-audio-studio/internal/desktop"
)

func main() {
	runtime.LockOSThread()
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	root := flag.String("root", filepath.Dir(cwd), "Recursive audio library directory")
	output := flag.String("output", filepath.Join(cwd, "output"), "Authoring output directory")
	lang := flag.String("lang", "plt", "Interface language: plt, en or fr")
	project := flag.String("project", "", "Open a project JSON file")
	smoke := flag.Bool("smoke-test", false, "Exercise widgets and exit without authoring")
	flag.Parse()
	if *lang != "en" && *lang != "plt" && *lang != "fr" {
		log.Fatal("-lang must be plt, en or fr")
	}
	engine, err := core.New(core.Config{Root: *root, Output: *output, App: cwd})
	if err != nil {
		log.Fatal(err)
	}
	err = desktop.Run(engine, desktop.Options{Language: *lang, Project: *project, Smoke: *smoke})
	engine.Close()
	if err != nil {
		log.Fatal(err)
	}
}
