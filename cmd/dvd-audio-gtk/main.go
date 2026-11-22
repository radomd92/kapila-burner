//go:build gtk

package main

import (
	"flag"
	"log"
	"os"
	"runtime"

	"dvd-audio-studio/internal/core"
	"dvd-audio-studio/internal/desktop"
	"dvd-audio-studio/internal/runtimeenv"
)

func main() {
	runtime.LockOSThread()
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	defaultRoot, defaultOutput, err := runtimeenv.DefaultPaths(cwd)
	if err != nil {
		log.Fatal(err)
	}
	root := flag.String("root", defaultRoot, "Recursive audio library directory")
	output := flag.String("output", defaultOutput, "Authoring output directory")
	lang := flag.String("lang", "plt", "Interface language: plt, en or fr")
	project := flag.String("project", "", "Open a project JSON file")
	smoke := flag.Bool("smoke-test", false, "Exercise widgets and exit without authoring")
	flag.Parse()
	if *lang != "en" && *lang != "plt" && *lang != "fr" {
		log.Fatal("-lang must be plt, en or fr")
	}
	app, err := core.ApplicationDir()
	if err != nil {
		log.Fatal(err)
	}
	if err := runtimeenv.PrepareDefaultLibrary(*root, defaultRoot); err != nil {
		log.Fatal(err)
	}
	cleanup, err := runtimeenv.Configure(true)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	engine, err := core.New(core.Config{Root: *root, Output: *output, App: app})
	if err != nil {
		cleanup()
		log.Fatal(err)
	}
	err = desktop.Run(engine, desktop.Options{Language: *lang, Project: *project, Smoke: *smoke})
	engine.Close()
	if err != nil {
		cleanup()
		log.Fatal(err)
	}
}
