# Kapila Burner

[Malagasy (Plateau)](README.md) | **English** | [Français](README.fr.md)

[![CI](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml/badge.svg)](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

A local web or native GTK workspace for authoring uncompressed LPCM DVD-Audio
disc trees/ISOs and true Video CD 2.0 BIN/CUE images. DVD-Audio is not DVD-Video
or an MP3 data disc and requires a compatible DVD-Audio player. VCD uses lossy
MPEG-1 video and MP2 audio for compatible VCD players. Authoring is image-first;
only GTK offers an explicitly confirmed physical VCD burn, never automatic burning.

## Download

Latest successful default-branch build (rolling **continuous prerelease**):

| Platform | Archive |
| --- | --- |
| Linux x86-64 | [kapila-burner-linux-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-linux-amd64.tar.gz) |
| Windows x86-64 | [kapila-burner-windows-amd64.zip](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-windows-amd64.zip) |
| macOS Intel x86-64 | [kapila-burner-macos-amd64.tar.gz](https://github.com/radomd92/kapila-burner/releases/download/continuous/kapila-burner-macos-amd64.tar.gz) |

[SHA256SUMS](https://github.com/radomd92/kapila-burner/releases/download/continuous/SHA256SUMS)
| [Release page](https://github.com/radomd92/kapila-burner/releases/tag/continuous)
| [CI workflow and build artifacts](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)

These links become available after the **first successful CI run on the default
branch** publishes the release; they are not a claim that all hosted builds
already pass. Each later successful default-branch run refreshes it. Pull-request
builds provide Actions artifacts retained for **14 days**, not public releases.
Version tags matching `v*` publish separate versioned releases.

Every archive contains both native GTK **`kapila-burner`** and portable web
**`kapila-burner-web`** executables (`.exe` on Windows). Extract before running.
The web executable needs no GTK, browser extensions, or frontend build step;
authoring tools are separate for both interfaces.

| Platform | Native GTK runtime and limits |
| --- | --- |
| Linux | Ubuntu 22.04 or newer, or a compatible distribution with system GTK 3; on Ubuntu install `libgtk-3-0`. A graphical session is required. |
| Windows | GTK DLLs and runtime data are bundled. Keep the **entire extracted folder** and use `launch-gtk.cmd`, not an isolated `.exe`. |
| macOS | Intel macOS 15 build; requires Homebrew GTK 3 under `/usr/local` (`brew install gtk+3`). Not self-contained, not an `.app` bundle, **unsigned and not notarized**. |

No ARM executables are published. Physical VCD burning is **Linux-only** and
requires `cdrdao` and explicit confirmation. Cross-platform image authoring also
requires separately installed compatible tools; see [Dependencies](#dependencies).
FFmpeg, FFprobe, VCDImager, `dvda-author`, genuine `mkisofs`, and `cdrdao` are
**not bundled**. Availability of these tools on your OS is a separate prerequisite.

To check archive integrity, download `SHA256SUMS` beside the archives and use
`sha256sum --check --ignore-missing SHA256SUMS` on Linux, or
`shasum -a 256 kapila-burner-macos-amd64.tar.gz` on macOS and compare the value.
On PowerShell use `Get-FileHash .\kapila-burner-windows-amd64.zip -Algorithm SHA256`.
Checksums detect changed downloads; they are **not cryptographic signatures**.

## Run

Run either adapter from the application directory, including when using a
prebuilt executable. Tool discovery uses the **current working directory**,
not the executable's location; the default output is its `output/` directory.

### Downloaded Executables

From the extracted application folder on Linux or macOS:

```sh
./kapila-burner -root /path/to/media
# English or French instead of the default Plateau Malagasy:
./kapila-burner -root /path/to/media -lang en
./kapila-burner -root /path/to/media -lang fr
# Or run the browser interface:
./kapila-burner-web -root /path/to/media
```

On Windows, run `launch-gtk.cmd` from the extracted folder; pass
`-root C:\Music -lang en` as arguments if desired. For the web interface:

```powershell
.\kapila-burner-web.exe -root C:\Music
```

The web server prints its local URL, normally **http://127.0.0.1:8787**.
Do not expose it to a network. Keep its terminal running and use Ctrl-C to stop it.

### Build The Web Adapter

Building requires Go 1.22 or newer (CI uses Go 1.24). The web adapter and shared core
use only the standard library and local Go packages; the HTML/CSS/JavaScript
UI is embedded. No npm, Node.js, frontend build step, or GTK development
headers are needed for the default web build and tests. The web build and
plain `go test ./...` need no C compiler; `make test` enables Go's race
detector, which requires CGO and a C compiler on Linux.

```sh
git clone https://github.com/radomd92/kapila-burner.git
cd kapila-burner
make web
./dvd-audio-studio -root /path/to/media
```

Open **http://127.0.0.1:8787** in your browser. Keep the terminal running;
Ctrl-C stops the server and cancels an active job.

Alternatively, use `go run . -root /path/to/media`. Available web
flags are `-root` (default `..`), `-output` (default `./output`), and `-listen`
(default `127.0.0.1:8787`, loopback addresses only). Use the exact URL printed
by the server rather than substituting a localhost alias.

### Native GTK

The GTK adapter calls the core directly: it needs **no HTTP server or browser**.
Local source builds retain the developer executable name `dvd-audio-gtk`;
release archives use `kapila-burner`. With native GTK 3 available, run a local build:

```sh
./dvd-audio-gtk -root /path/to/media -lang en
# Or start in Plateau Malagasy:
./dvd-audio-gtk -root /path/to/media -lang plt
# Open a project exported by either interface:
./dvd-audio-gtk -root /path/to/media -lang en -project /path/to/project.json
```

GTK also accepts `-output` (default `output/` under the current directory).
Its default library root is the current directory's parent. The native,
Nero/K3b-style workspace starts on a welcome screen with **DVD Audio** and
**Video CD** choices and an option to open an existing JSON project.

The top **Project** menu provides **New DVD-Audio** (`Ctrl+N`), **New Video CD**
(`Ctrl+Shift+N`), **Open** (`Ctrl+O`), **Save** (`Ctrl+S`), **Save As**
(`Ctrl+Shift+S`), **Close Project** (`Ctrl+W`), and **Quit** (`Ctrl+Q`). Save
remembers the opened/chosen path, including `-project`; saves are atomic, not
automatic. A title-bar `*` marks unsaved edits. New, Open, Close, and Quit protect
dirty projects with **Save / Discard / Cancel**; Close returns to the welcome screen.

A toolbar sits above a split media browser and ordered compilation, showing
track filenames, durations, source paths, and source-format details. Folder
navigation/filtering, group and track ordering controls, a capacity meter,
recorder/speed selectors, and a collapsible **Details & Job Log** keep the
workflow in one window. Useful shortcuts: `Ctrl+Enter` adds selected sources,
`Alt+Up` / `Alt+Down` reorders tracks, `Ctrl+B` creates an image, `F5` rescans,
and `Ctrl+L` toggles the log. Project editing is locked during jobs; language
switching and log viewing remain available. GTK Open/Save and web Import/Export
exchange the same project JSON, not existing disc images.

To build the optional adapter, typical Debian/Ubuntu build prerequisites are:

```sh
sudo apt install libgtk-3-dev pkg-config build-essential
```

Development headers are needed to build, not to run a downloaded executable.
A host-dependent no-sudo development overlay is documented at
[tools/gtk-dev/README.md](tools/gtk-dev/README.md). From the app directory:

```sh
make gtk
```

The target sources `tools/gtk-dev/env.sh` if `pkg-config` cannot find
`gtk+-3.0`. It builds gotk3 v0.6.4 with tags `gtk gtk_3_20`: `gtk` enables
the optional `//go:build gtk` adapter, while `gtk_3_20` selects compatible
binding APIs and avoids an upstream gotk3 3.22 callback/import bug. This does
not downgrade the toolkit: the local Linux build uses native GTK 3.24.41.
A cold gotk3 build can take several minutes; cached builds are much faster.

### Language

Both interfaces offer Plateau Malagasy (`plt`), English (`en`), and French (`fr`),
with live language switching. A fresh web UI starts in Plateau Malagasy and
remembers the selection in browser `localStorage`; an explicitly saved English
selection is still honored. GTK defaults to `-lang plt`; use `-lang en` or
`-lang fr` to choose another startup language, or use its live selector.
Existing project/group names, file
names, technical format identifiers, and raw external-tool diagnostics are
preserved rather than translated. Current job logs and application-generated
job errors retain the language selected when that job started, even after a
live interface switch.

## Architecture

- `internal/core` is the shared, UI-free engine for library scanning, tool
  discovery, estimates, validation, DVD/VCD authoring, guarded VCD burning,
  cancellation, and job snapshots.
  It imports neither HTTP nor GTK.
- Root `main.go` and embedded `web/index.html` form the loopback HTTP/web adapter.
- `internal/desktop` and `cmd/dvd-audio-gtk` form the optional native GTK adapter,
  making direct engine calls rather than requests to the web adapter.

Code inside this application module can use the same API without either UI.
For example, within a function returning `error` (imports: `context`,
`fmt`, `time`, and `dvd-audio-studio/internal/core`):

```go
engine, err := core.New(core.Config{
    Root: "/path/to/media",
    Output: "./output",
    App: "/path/to/kapila-burner",
})
if err != nil {
    return err
}
defer engine.Close() // Cancels active work and waits for final job.json.
ctx := core.WithLanguage(context.Background(), "en") // Or "plt" / "fr".
project := core.Project{
    Name: "Example", SampleRate: 44100, Bits: 16, Channels: 2,
    Capacity: "dvd5", ISO: true,
    Groups: []core.Group{{Name: "Group 1", Tracks: []string{"album/track.flac"}}},
}
estimate, err := engine.Estimate(ctx, project)
if err != nil {
    return err
}
fmt.Println(estimate.Fits)
job, err := engine.Start(ctx, project)
if err != nil {
    return err
}
fmt.Println(job.ID)
for {
    snapshot := engine.Snapshot()
    if snapshot.State != "running" {
        fmt.Println(snapshot.State, snapshot.Output, snapshot.Error)
        return nil
    }
    time.Sleep(250 * time.Millisecond)
}
```

`Start` runs asynchronously and captures the caller's language for the job.
`Snapshot` returns a copy, not mutable internal state. Use `engine.Cancel()`
to request cancellation; keep the engine alive until the job finishes.

## Dependencies

- `ffprobe` inspects source duration and format; `ffmpeg` converts the first
  audio stream to PCM WAV for DVD-Audio or transcodes video/audio for VCD.
  Both normally come from `PATH`.
- `dvda-author` authors DVD-Audio navigation and audio objects.
- `mkisofs` packages the disc tree and must support genuine `-dvd-audio`.
  A `genisoimage` substitute without that option is not sufficient. This tool
  is unnecessary when ISO creation is disabled.
- `vcdimager` creates Video CD 2.0 BIN/CUE images; DVD tools are not needed for VCD.
- `cdrdao` is needed only for Linux GTK physical VCD burning or optional drive simulation,
  not image authoring or web downloads.

Install the tools required for your chosen disc type separately on each machine.
Check the UI's dependency status; do not run the application with sudo.

The optional development `tools/bin/dvda-author` and `tools/bin/mkisofs` launchers use an
extracted **x86_64/amd64 Ubuntu Impish package**, not a system installation.
They are platform-specific, not portable binaries for every Linux machine.
See [tools/README.md](tools/README.md) for the SourceForge provenance link,
package name, SHA-256, shared-library requirements, launcher workaround, and
verified backend smoke commands. Use these launchers, not the package's
hard-coded launcher under `tools/opt/`. No sudo is needed to run the app or
use these extracted tools.

Explicit executable overrides take precedence:

| Environment variable | Tool |
| --- | --- |
| `FFMPEG` | `ffmpeg` |
| `FFPROBE` | `ffprobe` |
| `DVDA_AUTHOR` | `dvda-author` |
| `MKISOFS` | `mkisofs` with `-dvd-audio` support |
| `VCDIMAGER` | `vcdimager` |
| `CDRDAO` | `cdrdao` |

Use absolute executable paths for overrides. Without overrides, only
`dvda-author` and `mkisofs` try `./tools/bin/` before `PATH`; the other tools
use `PATH`. `dvda-author-dev` is a final fallback for `dvda-author`.
Dependency status and resolved paths appear in the UI. Restart the chosen
adapter after changing tools or overrides.
The large extracted `tools/opt/` payload is ignored by Git; the launchers
and tools README remain eligible for version control.

## Make A DVD-Audio Disc

1. Let the recursive library scan finish. Filter by an album or folder name
   to find your input files.
   The filter matches relative paths; it does not change the library root or
   limit the recursive scan. Hidden directories, the application directory,
   and the configured output directory are skipped.
2. In the web UI, select tracks, or use **Select visible**, then
   **Add selected tracks** to append them to the active group. Selection can remain checked across
   filter changes, so inspect the selected count before adding. Create up to
   nine groups with up to 99 tracks per group. Use the up/down controls to
   check and adjust track order; duplicates are allowed. Group order and
   track order are preserved, with empty groups omitted during authoring.
   In GTK, multi-select library rows and choose **Add selected to group**;
   use its group selector and **Up**/**Down** controls for sequencing.
3. Start with the practical defaults: **16-bit, 44.1 kHz, stereo**, DVD-5,
   and ISO enabled. Mono is an optional space-saving choice for speech, but
   it downmixes and loses stereo information. The UI also offers 48, 96, and
   192 kHz and 24-bit PCM; higher settings consume more space.
4. Review the disc and workspace estimates, then choose **Build DVD-Audio** in
   the web UI or **Create Image** in GTK.
   One job runs at a time per engine and shared output directory (see below).
   The app converts to WAV, authors `AUDIO_TS`, checks for required nonempty
   navigation/audio files, and optionally creates an
   ISO using `mkisofs -dvd-audio`. Oversized projects and final outputs are
   rejected. The session log shows commands and progress; cancellation
   retains partial output.
5. Download the completed ISO in the web UI, or use the output path shown by
   either adapter. In a separate burning application choose **burn/write disc
   image**, not a data-disc operation
   that merely copies the `.iso` file onto a disc. DVD ISO burning remains
   external: the application's GTK burn feature is for VCD only.

Project and group names are organizational labels in the JSON manifest,
not on-disc text or player-visible titles. There are no visual menus, MLP
compression, multichannel output, or DRM features.

## Video CD

Choose **Project > New Video CD** in GTK, or **Video CD** as the web project
type. Select PAL/NTSC and CD74/CD80, add sources, order the single group's
**1-98 sequence tracks**, review capacity/workspace estimates, then create the
image. Sources are preserved, including audio-only inputs, which receive a
static black picture. Video inputs are scaled/padded to 4:3; silent video gets
silent audio. Output is true **VCD 2.0**, not an Audio CD, MP3 data CD, or DVD:

| Profile | Video | Audio |
| --- | --- | --- |
| PAL | MPEG-1, 352x288, 25 fps, 1150 kbps | MP2, 44.1 kHz stereo, 224 kbps |
| NTSC | MPEG-1, 352x240, 30000/1001 (~29.97) fps, 1150 kbps | MP2, 44.1 kHz stereo, 224 kbps |

VCD transcoding is **lossy**, even from lossless sources. Nominal capacity is
about **74/80 minutes**, not the many hours possible on an MP3 data CD. The
core budgets 333,000/360,000 raw 2352-byte sectors, reserving control-track and
lead-out space plus per-track padding/gaps; usable duration is lower. Trust
the conservative estimate and final image-size check, not nominal runtime alone.

The web UI authors and downloads images but exposes **no hardware burn API**.
Download **both** `disc.cue` and `disc.bin` into the same directory with those
names. The downloaded CUE references relative `disc.bin`, making the pair
portable. Use K3b's image-burning workflow for externally generated/retained
BIN/CUE images, not a data-disc operation that copies the files.

### GTK VCD Burning

Create the image first, then choose **Burn VCD Image** / **Burn**. Select an
actual Linux optical writer, use a blank **CD-R or CD-RW, never a DVD**, review
the image, device, and speed, and explicitly confirm before writing. The core
accepts speeds **1x-52x**; GTK defaults to **8x**, with no automatic/zero speed.
Optional **Dummy mode** is real-drive simulation via
`cdrdao`, if the writer supports it, not an offline mock; it also requires
explicit confirmation. Recorder discovery reads Linux kernel metadata without
opening the drive or querying inserted media.

Only a completed VCD authored in the **same GTK session** is eligible. The
engine keeps an in-memory trust registry and verifies CUE/BIN SHA-256 hashes
before burning; `image.json` alone cannot restore eligibility. After restarting,
or for web-generated images, reauthor the project in GTK or burn its BIN/CUE
externally in K3b. There is no existing-image import feature.

Cancellation may spoil the CD; images and logs are retained. Check job/tool
diagnostics for missing permissions, unsuitable/nonblank media, or unsupported
drive/simulation errors. Fix user device access or supply a suitable blank CD
rather than running the app with sudo. Successful image creation does not
guarantee physical playback compatibility.

## DVD Capacity And Long Tracks

Large libraries may require several discs. Select subsets using the live
capacity estimate. The first full-library scan can take over a minute; subsequent scans reuse
cached probe results for unchanged files while that adapter remains running.

MP3 and other compressed inputs expand to uncompressed PCM. Source file size
does not predict disc usage, and decoding or upsampling does not restore
quality lost to compression.

```text
PCM bytes = duration in seconds * sample rate * channels * (bits / 8)
Disc estimate = PCM bytes * 1.08 + 1 MiB per track + 16 MiB
```

At stereo 16-bit/44.1 kHz, PCM uses 176,400 bytes/second, about 635 MB/hour.
Budget conservatively for **about seven hours on DVD-5**, or **about fourteen
hours in mono** at the same rate/depth. These are planning figures, not
guarantees: track count, authoring overhead, and the selected profile matter.
The app uses 4,700,000,000 bytes for DVD-5 and 8,547,991,552 bytes for DVD-9.
Use the live estimate and final size checks rather than filling to a claimed
maximum runtime.

Each decoded track must fit a classic WAV's roughly **4 GiB limit**. The app
refuses tracks whose estimated PCM payload exceeds `0xffffffff - 4096` bytes;
it does not automatically split them or use RF64. Split long recordings into
separate tracks **before adding them to the project**. For example, create
one-hour stereo PCM16/44.1 WAV segments in a new folder under the library:

```sh
mkdir -p /path/to/media/split-tracks
ffmpeg -nostdin -i /path/to/media/long-recording.mp3 \
  -map 0:a:0 -vn -sn -dn -map_metadata -1 \
  -ac 2 -ar 44100 -c:a pcm_s16le \
  -f segment -segment_time 3600 -reset_timestamps 1 -segment_format wav \
  /path/to/media/split-tracks/track-%03d.wav
```

Replace the example input path with your recording. This writes new files
without modifying the source. Rescan, filter `split-tracks`, and add the
numbered WAVs in order. One-hour segments at this profile are well below the
WAV limit; if you later choose a much larger output profile, recheck the
per-track estimate.

## Output And Persistence

Each authoring job gets a dedicated `output/disc-*/` directory. DVD-Audio contains:

```text
project.json       project settings, groups, and ordered source paths
pcm/               g01-t01.wav, etc., decoded PCM intermediates
disc/AUDIO_TS/     authored navigation, backups, and AOB audio objects
dvd-audio.iso      image, only when ISO creation is enabled and succeeds
job.json           final job status, error/output path, and retained log
```

VCD uses the same `project.json` and `job.json`, plus:

```text
track-*.mpg        transcoded VCD MPEG sequence intermediates
disc.cue           cue sheet referencing the BIN
disc.bin           raw VCD image, paired with disc.cue (not an ISO)
image.json         image paths, SHA-256 hashes, and capacity metadata
```

GTK burn jobs use separate `output/burn-*/` directories containing `burn.json`
(request), `source.json` (image metadata), and final `job.json`; source images
remain in their authoring directory.

Files may be absent or incomplete after a failure or interruption. Successful,
failed, and cancelled outputs are retained; there is no automatic cleanup.
Inspect and remove unneeded job directories manually when no job is using
them. `job.json` is written when the job finishes, not continuously, and its
log is bounded to the last 600 lines rather than an unlimited transcript.

For DVD-Audio, allow roughly **three times the decoded audio size**, plus
overhead and spare space, when creating an ISO: PCM intermediates, the authored tree, and the
image coexist. The UI includes these plus a 256 MiB reserve in its workspace
estimate, and the core checks available output-filesystem space before
starting. Other jobs, retained outputs, and tool scratch space can still use
disk space, so leave additional headroom. VCD estimates account for MPEG
intermediates, the BIN image, and a 256 MiB reserve instead of PCM/DVD copies.

The web UI auto-saves the current project in this browser's `localStorage`.
Browser storage is origin-specific and can be cleared or unavailable; export
important projects explicitly. GTK does not auto-save: use **Save / Save As**
to persist edits and **Open** or `-project` to reopen them.
Both adapters use the same project JSON format, preserving names, settings,
groups, and ordered paths. This is a portable manifest, not an audio backup:
sources must still exist at the same paths relative to the chosen library root.
A started job also writes its own `project.json`, not a continuously saved
copy of later UI edits.

Job state is **memory-only**, tracking only the latest job in each engine.
GTK and web processes have separate engines: they do not share live status,
job logs, cancellation controls, or browser storage. A job-scoped Linux
`flock` on Linux/macOS (and platform-specific locking on Windows) on the same
output directory's `.authoring.lock` prevents simultaneous
authoring/burning there, including between GTK and web; scanning and estimation
can coexist. Use a common `-output` directory when you want that mutual exclusion.
Different output directories can author independently; this is not a global
machine-wide lock, and concurrent work still competes for disk and CPU.
Burning also takes a per-device lock shared across this user's application
instances even with different output directories. These are **app-local,
per-user coordination locks**, not locks on K3b or other external software.
Do not use the same writer concurrently outside the app.

Starting another job replaces that engine's previous status entry (and the
web download entry). Restarting an adapter loses its job status and, for web,
the download endpoint for old jobs; it does not delete their files. Older
outputs remain on disk even when absent from the UI. Retrieve retained ISOs,
BIN/CUE pairs, and manifests directly from the output directory. Jobs do not
resume after restart. Closing either adapter cancels its active job and waits for final
job-state writing.

## Safety And Compatibility

The web server binds only to loopback and checks the exact request host and
browser origin to reject cross-origin requests. Do not expose it through a
remote proxy, tunnel, or port forward; it is a local tool, not an authenticated
multi-user service. Do not run it with sudo. Source files are read, not
overwritten, and generated files go to dedicated job directories. Keep
`-output` dedicated to generated content, separate from source audio.

Path checks and loopback restrictions are not a media-parser sandbox.
FFmpeg/FFprobe and the authoring tools process local inputs with your user
permissions; malicious or untrusted media remains a risk. Use trusted tools,
keep them updated, and avoid untrusted inputs or isolate processing yourself.

The backend smoke test recorded in [tools/README.md](tools/README.md) verified
authoring and ISO generation from a one-second stereo 24-bit/96-kHz WAV,
including an `AUDIO_TS` tree and a UDF 1.5 image. It did **not** test burning
or hardware playback. Successful authoring and file checks are not a hardware
compatibility guarantee; test a small disc on the intended DVD-Audio player
before committing a large collection.

## Checks

Run from the application directory:

```sh
make test
# Optional native adapter checks, using system GTK dev files or the overlay:
make test-gtk
make smoke-gtk
# Real VCD authoring, as enabled in Linux CI (no physical burn):
VCD_INTEGRATION=1 make test
# Optional local DVD-Audio + VCD integration, with all tools provisioned:
DVDA_INTEGRATION=1 VCD_INTEGRATION=1 go test -race ./...
```

`make test` runs `go test -race ./...` and `go vet ./...` without GTK build
tags or C/GTK headers. Engine tests cover concurrent callers, copied job
snapshots, per-caller localization and job-start language isolation, job-scoped
same-output/per-writer locking, cancellation, DVD/VCD estimates, format limits,
source containment, image integrity, and burn validation with mock-only writers;
web tests cover HTTP guards and portable BIN/CUE downloads. `make test-gtk` tests
native project handling with tags `gtk gtk_3_20`. `make smoke-gtk` builds the real GTK
application and exercises widgets in all three languages (`en`, `plt`, `fr`) with
`G_DEBUG=fatal-criticals`; it requires an accessible `DISPLAY` and is not a
headless test. Smoke mode blocks authoring and burning, including real-drive
simulation; it checks welcome/menus, project save/dirty protection, profiles,
action eligibility, and language switching. Linux CI supplies a virtual display
with `xvfb-run -a`; it does not require a physical monitor or optical drive.

The opt-in core integration tests generate short synthetic
tracks and run real conversion, multi-group authoring, and ISO creation at
stereo 16-bit/44.1 kHz, mono 16-bit/48 kHz, and stereo 24-bit/96 kHz. They
check saved manifests, navigation/audio objects, and ISO/UDF descriptors. They
also create real PAL/NTSC VCD images from synthetic audio and video, checking
BIN/CUE structure and MPEG-1/MP2 profiles.
These tests require the authoring dependencies and do not burn media or test
hardware playback. For additional manual smoke commands, see
[tools/README.md](tools/README.md).

## CI And Contributing

The [CI workflow](https://github.com/radomd92/kapila-burner/actions/workflows/ci.yml)
builds native GTK and web archives on Ubuntu 22.04, Windows Server 2022, and
Intel macOS 15. Linux runs race tests, vet, GTK tests, and the three-language
widget smoke checks, with `VCD_INTEGRATION=1` for real VCD image authoring.
Hosted CI does **not** provision the DVD-Audio toolchain or enable
`DVDA_INTEGRATION`; that remains an optional local/manual check with external
tools installed. No CI job burns physical media.

Use `make web`, `make gtk`, `make test`, `make test-gtk`, and `make smoke-gtk`
for local development; `make package` builds the release layout on the target
OS with its GTK development dependencies. Linux has been compiled locally;
Windows and Darwin web cross-compilation is not verification of their native
GTK packages. Hosted results remain pending until visible in Actions; consult
the badge and run logs rather than assuming all three builds pass.

For contributions, branch from `main`, make a focused change, run the relevant
checks, and open a pull request against `main`. Include reproduction steps for
bugs and distinguish image-authoring checks from physical-player tests.
