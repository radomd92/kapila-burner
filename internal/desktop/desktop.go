//go:build gtk

// Package desktop owns the native GTK adapter. All widget access stays on the
// locked GTK thread; workers only exchange copied core values through IdleAdd.
package desktop

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"dvd-audio-studio/internal/core"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

type Options struct {
	Language, Project string
	Smoke             bool
}
type translation struct {
	en, mg string
	set    func(string)
}
type app struct {
	engine                                                       *core.Engine
	ctx                                                          context.Context
	stop                                                         context.CancelFunc
	wg                                                           sync.WaitGroup
	window                                                       *gtk.Window
	lang                                                         string
	translations                                                 []translation
	project                                                      core.Project
	group                                                        int
	loading, closed, busy, pending, polling, scanning, canceling bool
	revision, scanRevision, jobRevision                          uint64
	estimateCancel, scanCancel, startCancel                      context.CancelFunc
	edit                                                         *gtk.Box
	name, groupName, filter                                      *gtk.Entry
	groups, rate, bits, channels, media, language                *gtk.ComboBoxText
	iso                                                          *gtk.CheckButton
	library, tracks                                              *gtk.ListBox
	files, visible                                               []core.AudioFile
	libraryLabels                                                []*gtk.Label
	scanStatus, estimateStatus, jobStatus, tools                 *gtk.Label
	scanProgress                                                 *gtk.ProgressBar
	build, cancel, rescan                                        *gtk.Button
	logBuffer                                                    *gtk.TextBuffer
	lastJob                                                      *core.Job
	burnSource                                                   *core.Job
	authorJobID                                                  string
	authorProject, burnProject                                   core.Project
	lastLog                                                      string
	estimate                                                     *core.Estimate
	estimateError                                                string
	startError                                                   string
	smokeError                                                   error
	active, dirty                                                bool
	projectPath                                                  string
	saved                                                        core.Project
	stack                                                        *gtk.Stack
	welcome                                                      *gtk.Box
	actions                                                      map[string]*gtk.MenuItem
	actionButtons                                                map[string]*gtk.Button
	capacity                                                     *gtk.ProgressBar
	standard, recorder, speed                                    *gtk.ComboBoxText
	formatNote                                                   *gtk.Label
	logDock                                                      *gtk.Expander
	folder                                                       *gtk.Entry
	recorderRevision                                             uint64
	quitAfterJob                                                 bool
	smokeMode                                                    bool
}

func Run(engine *core.Engine, options Options) error {
	if options.Language != "en" && options.Language != "fr" && options.Language != "plt" {
		options.Language = "plt"
	}
	if err := gtk.InitCheck(nil); err != nil {
		return err
	}
	ctx, stop := context.WithCancel(context.Background())
	a := &app{engine: engine, ctx: ctx, stop: stop, lang: options.Language, project: newProject(options.Language), smokeMode: options.Smoke}
	if options.Project != "" {
		p, err := readProject(options.Project)
		if err != nil {
			stop()
			return fmt.Errorf("%s", errorText(a.lang, err))
		}
		a.project = p
		a.active = true
		a.projectPath = options.Project
		a.saved = cloneProject(p)
	}
	a.construct()
	a.window.ShowAll()
	if a.active {
		a.stack.SetVisibleChildName("workspace")
	} else {
		a.stack.SetVisibleChildName("welcome")
	}
	a.setLocked(false)
	if a.active {
		a.changed()
	}
	if !options.Smoke {
		a.scan()
	}
	glib.TimeoutAdd(250, func() bool {
		if a.closed {
			return false
		}
		if a.scanning {
			a.scanProgress.Pulse()
		}
		if !a.polling {
			a.polling = true
			seq := a.jobRevision
			a.work(func() {
				j := engine.Snapshot()
				a.idle(func() {
					a.polling = false
					if seq == a.jobRevision {
						a.showJob(j)
					}
				})
			})
		}
		return true
	})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	a.work(func() {
		select {
		case <-signals:
			a.idle(a.quit)
		case <-ctx.Done():
		}
	})
	if options.Smoke {
		glib.TimeoutAdd(300, func() bool { a.smoke(); return false })
	}
	gtk.Main()
	a.closed = true
	stop()
	signal.Stop(signals)
	a.wg.Wait()
	return a.smokeError
}

func (a *app) work(f func()) { a.wg.Add(1); go func() { defer a.wg.Done(); f() }() }
func (a *app) idle(f func()) {
	glib.IdleAdd(func() {
		if !a.closed {
			f()
		}
	})
}
func (a *app) context() context.Context { return core.WithLanguage(a.ctx, a.lang) }
func (a *app) t(en, mg string) string {
	return translate(a.lang, en, mg)
}
func (a *app) bind(en, mg string, set func(string)) {
	a.translations = append(a.translations, translation{en, mg, set})
	set(a.t(en, mg))
}
func (a *app) label(en, mg string) *gtk.Label {
	l, _ := gtk.LabelNew("")
	l.SetXAlign(0)
	a.bind(en, mg, l.SetText)
	return l
}
func (a *app) button(en, mg string, f func()) *gtk.Button {
	b, _ := gtk.ButtonNew()
	a.bind(en, mg, b.SetLabel)
	b.Connect("clicked", f)
	return b
}
func box(vertical bool, spacing int) *gtk.Box {
	o := gtk.ORIENTATION_HORIZONTAL
	if vertical {
		o = gtk.ORIENTATION_VERTICAL
	}
	b, _ := gtk.BoxNew(o, spacing)
	return b
}
func entry() *gtk.Entry { e, _ := gtk.EntryNew(); return e }
func combo(values ...string) *gtk.ComboBoxText {
	c, _ := gtk.ComboBoxTextNew()
	for _, v := range values {
		c.Append(v, v)
	}
	return c
}
func scroll(w gtk.IWidget) *gtk.ScrolledWindow {
	s, _ := gtk.ScrolledWindowNew(nil, nil)
	s.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	s.Add(w)
	s.SetVExpand(true)
	return s
}
func plainLabel(s string) *gtk.Label {
	l, _ := gtk.LabelNew(s)
	l.SetXAlign(0)
	l.SetSelectable(true)
	return l
}

func (a *app) constructEditor() *gtk.Box {
	outer := box(true, 8)
	outer.SetBorderWidth(12)
	a.edit = box(true, 8)
	outer.PackStart(a.edit, true, true, 0)
	projectRow := box(false, 8)
	projectRow.PackStart(a.label("Project", "Tetikasa"), false, false, 0)
	a.name = entry()
	projectRow.PackStart(a.name, true, true, 0)
	a.standard = combo()
	a.standard.Append("pal", "PAL")
	a.standard.Append("ntsc", "NTSC")
	a.standard.Connect("changed", a.profileChanged)
	projectRow.PackStart(a.standard, false, false, 0)
	a.edit.PackStart(projectRow, false, false, 0)
	profile := box(false, 8)
	a.rate = combo("44100", "48000", "88200", "96000", "176400", "192000")
	a.bits = combo("16", "24")
	a.channels = combo("1", "2")
	a.media = combo("dvd5", "dvd9")
	for _, f := range []struct {
		en, mg string
		c      *gtk.ComboBoxText
	}{{"Rate (Hz)", "Taham-peo (Hz)", a.rate}, {"Bits", "Bit", a.bits}, {"Channels", "Fantsona", a.channels}, {"Media", "Kapila", a.media}} {
		profile.PackStart(a.label(f.en, f.mg), false, false, 0)
		profile.PackStart(f.c, false, false, 0)
		f.c.Connect("changed", a.profileChanged)
	}
	a.iso, _ = gtk.CheckButtonNewWithLabel("ISO")
	a.iso.Connect("toggled", a.profileChanged)
	profile.PackStart(a.iso, false, false, 0)
	a.edit.PackStart(profile, false, false, 0)
	a.formatNote = plainLabel("")
	a.formatNote.SetLineWrap(true)
	a.formatNote.SetMaxWidthChars(100)
	a.edit.PackStart(a.formatNote, false, false, 0)
	paned, _ := gtk.PanedNew(gtk.ORIENTATION_HORIZONTAL)
	paned.SetPosition(360)
	a.edit.PackStart(paned, true, true, 0)
	left := box(true, 6)
	right := box(true, 6)
	left.SetMarginEnd(8)
	right.SetMarginStart(8)
	paned.Pack1(left, true, false)
	paned.Pack2(right, true, false)
	left.PackStart(a.label("MEDIA BROWSER", "FITADIAVANA RAKITRA"), false, false, 0)
	a.folder = entry()
	a.folder.SetText(a.engine.Root())
	a.folder.SetTooltipText(a.engine.Root())
	a.folder.Connect("changed", a.renderLibrary)
	navigation := box(false, 4)
	navigation.PackStart(a.button("Up", "Miakatra", func() {
		path, _ := a.folder.GetText()
		if !filepath.IsAbs(path) {
			path = filepath.Join(a.engine.Root(), path)
		}
		parent := filepath.Dir(filepath.Clean(path))
		if rel, err := filepath.Rel(a.engine.Root(), parent); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			a.folder.SetText(parent)
		}
	}), false, false, 0)
	navigation.PackStart(a.folder, true, true, 0)
	left.PackStart(navigation, false, false, 0)
	quick := box(false, 4)
	quick.PackStart(a.button("All", "Rehetra", func() { a.folder.SetText(a.engine.Root()) }), false, false, 0)
	for _, name := range []string{"moebius", "soti_mp3", filepath.Join("stories_of_the_imperium", "soti_mp3")} {
		path := filepath.Join(a.engine.Root(), name)
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			b := a.button(filepath.Base(name), filepath.Base(name), func() { a.folder.SetText(path) })
			b.SetTooltipText(path)
			quick.PackStart(b, false, false, 0)
		}
	}
	left.PackStart(quick, false, false, 0)
	search := box(false, 6)
	a.filter = entry()
	a.bind("Filter paths", "Sivano ny lalan-drakitra", a.filter.SetPlaceholderText)
	search.PackStart(a.filter, true, true, 0)
	a.rescan = a.button("Rescan", "Hikaroka indray", a.scan)
	search.PackStart(a.rescan, false, false, 0)
	left.PackStart(search, false, false, 0)
	a.filter.Connect("changed", a.renderLibrary)
	a.library, _ = gtk.ListBoxNew()
	a.library.SetSelectionMode(gtk.SELECTION_MULTIPLE)
	a.library.Connect("row-activated", func() { a.addSelected() })
	left.PackStart(scroll(a.library), true, true, 0)
	a.scanStatus = plainLabel("")
	left.PackStart(a.scanStatus, false, false, 0)
	a.scanProgress, _ = gtk.ProgressBarNew()
	left.PackStart(a.scanProgress, false, false, 0)
	left.PackStart(a.button("Add selected to group", "Ampidiro ao amin'ny vondrona", a.addSelected), false, false, 0)
	groupRow := box(false, 6)
	a.groups, _ = gtk.ComboBoxTextNew()
	groupRow.PackStart(a.groups, true, true, 0)
	addGroup := a.button("+ Group", "+ Vondrona", a.addGroup)
	deleteGroup := a.button("Delete group", "Fafao ny vondrona", a.deleteGroup)
	a.actionButtons["group-add"] = addGroup
	a.actionButtons["group-delete"] = deleteGroup
	groupRow.PackStart(addGroup, false, false, 0)
	groupRow.PackStart(deleteGroup, false, false, 0)
	right.PackStart(groupRow, false, false, 0)
	a.groups.Connect("changed", func() {
		if !a.loading {
			i := a.groups.GetActive()
			if i >= 0 && i < len(a.project.Groups) {
				a.group = i
				a.renderTracks()
			}
		}
	})
	a.groupName = entry()
	right.PackStart(a.groupName, false, false, 0)
	a.groupName.Connect("changed", func() {
		if a.loading || a.locked() {
			return
		}
		s, _ := a.groupName.GetText()
		a.project.Groups[a.group].Name = s
		a.renderGroups()
		a.changed()
	})
	right.PackStart(a.label("COMPILATION", "RAKI-PEO HO AN'NY KAPILA"), false, false, 0)
	columns := box(false, 8)
	for i, pair := range [][2]string{{"#", "#"}, {"Filename", "Anaran-drakitra"}, {"Duration", "Faharetana"}, {"Source path", "Lalana niaviany"}} {
		l := a.label(pair[0], pair[1])
		widths := []int{32, 280, 72, 180}
		l.SetSizeRequest(widths[i], -1)
		columns.PackStart(l, i == 3, true, 0)
	}
	style(columns, "table-heading")
	right.PackStart(columns, false, false, 0)
	a.tracks, _ = gtk.ListBoxNew()
	a.tracks.SetSelectionMode(gtk.SELECTION_SINGLE)
	right.PackStart(scroll(a.tracks), true, true, 0)
	trackButtons := box(false, 6)
	trackButtons.PackStart(a.button("Up", "Miakatra", func() { a.move(-1) }), false, false, 0)
	trackButtons.PackStart(a.button("Down", "Midina", func() { a.move(1) }), false, false, 0)
	trackButtons.PackStart(a.button("Remove track", "Esory ny raki-peo", a.removeTrack), false, false, 0)
	right.PackStart(trackButtons, false, false, 0)
	a.estimateStatus = plainLabel("")
	a.estimateStatus.SetLineWrap(true)
	a.capacity, _ = gtk.ProgressBarNew()
	style(a.capacity, "disc-capacity")
	outer.PackStart(a.capacity, false, false, 0)
	outer.PackStart(a.estimateStatus, false, false, 0)
	a.tools = plainLabel("")
	a.tools.SetLineWrap(true)
	a.tools.SetMaxWidthChars(100)
	jobRow := box(false, 8)
	a.build = a.button("Create Image", "Hamorona sary kapila", a.start)
	style(a.build, "primary")
	a.cancel = a.button("Cancel", "Ajanony", a.cancelJob)
	jobRow.PackStart(a.build, false, false, 0)
	jobRow.PackStart(a.cancel, false, false, 0)
	a.recorder = combo()
	a.recorder.Connect("changed", a.updateActions)
	a.recorder.SetSizeRequest(180, -1)
	a.recorder.SetTooltipText(a.t("Disc recorder", "Mpanoratra kapila"))
	a.speed = speedCombo()
	jobRow.PackStart(a.recorder, false, false, 0)
	jobRow.PackStart(a.speed, false, false, 0)
	a.speed.SetTooltipText(a.t("Write speed (Nx)", "Hafainganam-panoratana (Nx)"))
	a.jobStatus = plainLabel("")
	a.jobStatus.SetLineWrap(true)
	a.jobStatus.SetMaxWidthChars(70)
	outer.PackStart(jobRow, false, false, 0)
	style(a.jobStatus, "footer-status")
	outer.PackEnd(a.jobStatus, false, false, 0)
	view, _ := gtk.TextViewNew()
	view.SetEditable(false)
	view.SetCursorVisible(false)
	view.SetWrapMode(gtk.WRAP_WORD_CHAR)
	a.logBuffer, _ = view.GetBuffer()
	view.SetMonospace(true)
	logScroll := scroll(view)
	logScroll.SetVExpand(false)
	logScroll.SetSizeRequest(-1, 120)
	a.logDock, _ = gtk.ExpanderNew("")
	a.bind("Details & Job Log", "Antsipiriany sy tatitry ny asa", a.logDock.SetLabel)
	details := box(true, 6)
	details.PackStart(a.tools, false, false, 0)
	paths := plainLabel(a.engine.Root() + "\n" + a.engine.Output())
	paths.SetEllipsize(pango.ELLIPSIZE_MIDDLE)
	details.PackStart(paths, false, false, 0)
	details.PackStart(logScroll, true, true, 0)
	a.logDock.Add(details)
	outer.PackStart(a.logDock, false, true, 0)
	a.name.Connect("changed", func() {
		if !a.loading && !a.locked() {
			a.project.Name, _ = a.name.GetText()
			a.changed()
		}
	})
	a.loadWidgets()
	a.refreshTools()
	return outer
}

func (a *app) locked() bool { return a.busy || a.pending || a.canceling }
func (a *app) setLocked(locked bool) {
	a.busy = locked
	a.edit.SetSensitive(!a.locked())
	a.build.SetSensitive(a.active && !a.locked() && a.estimate != nil && a.estimate.Fits)
	a.cancel.SetSensitive(a.locked())
	if a.canceling {
		a.cancel.SetSensitive(false)
	}
	a.rescan.SetSensitive(!a.locked() && !a.scanning)
	a.updateActions()
}
func (a *app) shutdown() {
	if a.closed {
		return
	}
	a.closed = true
	a.stop()
	a.window.Destroy()
	gtk.MainQuit()
}
func (a *app) confirm(text string) bool {
	d := gtk.MessageDialogNew(a.window, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE, "%s", text)
	d.AddButton(a.t("No", "Tsia"), gtk.RESPONSE_NO)
	d.AddButton(a.t("Yes", "Eny"), gtk.RESPONSE_YES)
	d.SetDefaultResponse(gtk.RESPONSE_NO)
	defer d.Destroy()
	return d.Run() == gtk.RESPONSE_YES
}
func (a *app) error(err error) {
	d := gtk.MessageDialogNew(a.window, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_NONE, "%s", errorText(a.lang, err))
	d.AddButton(a.t("Close", "Akatona"), gtk.RESPONSE_CLOSE)
	d.Run()
	d.Destroy()
}

func (a *app) loadWidgets() {
	a.loading = true
	a.name.SetText(a.project.Name)
	a.rate.SetActiveID(strconv.Itoa(a.project.SampleRate))
	a.bits.SetActiveID(strconv.Itoa(a.project.Bits))
	a.channels.SetActiveID(strconv.Itoa(a.project.Channels))
	a.media.SetActiveID(a.project.Capacity)
	a.media.RemoveAll()
	if a.project.Format == "vcd" {
		a.media.Append("cd74", "CD 74 min")
		a.media.Append("cd80", "CD 80 min")
	} else {
		a.media.Append("dvd5", "DVD-5")
		a.media.Append("dvd9", "DVD-9")
	}
	a.media.SetActiveID(a.project.Capacity)
	a.standard.SetActiveID(a.project.VideoStandard)
	if a.project.VideoStandard == "" {
		a.standard.SetActiveID("pal")
	}
	a.standard.SetSensitive(a.project.Format == "vcd")
	a.standard.SetNoShowAll(true)
	a.standard.SetVisible(a.project.Format == "vcd")
	for _, c := range []*gtk.ComboBoxText{a.rate, a.bits, a.channels} {
		c.SetSensitive(a.project.Format != "vcd")
	}
	a.iso.SetSensitive(a.project.Format != "vcd")
	if a.project.Format == "vcd" {
		a.formatNote.SetText(a.t("VIDEO CD  /  MPEG-1 + MP2. Audio-only tracks use a static black picture. Image: CUE + BIN; CD-R/RW only.", "VIDEO CD  /  MPEG-1 + MP2. Sary mainty tsy mihetsika ho an'ny raki-peo. Sary kapila: CUE + BIN; CD-R/RW ihany."))
	} else {
		a.formatNote.SetText(a.t("DVD-AUDIO  /  Lossless PCM. Requires a true DVD-Audio player; MP3 expands to PCM.", "DVD-AUDIO  /  PCM tsy very kalitao. Mila mpamaky tena mahazaka DVD-Audio; miitatra ho PCM ny MP3."))
	}
	a.iso.SetActive(a.project.ISO)
	a.loading = false
	a.renderGroups()
	a.renderTracks()
}
func (a *app) profileChanged() {
	if a.loading || a.locked() {
		return
	}
	a.project.SampleRate, _ = strconv.Atoi(a.rate.GetActiveID())
	a.project.Bits, _ = strconv.Atoi(a.bits.GetActiveID())
	a.project.Channels, _ = strconv.Atoi(a.channels.GetActiveID())
	a.project.Capacity = a.media.GetActiveID()
	a.project.ISO = a.iso.GetActive()
	if a.project.Format == "vcd" {
		a.project.VideoStandard = a.standard.GetActiveID()
	}
	a.changed()
}
func clearList(l *gtk.ListBox) {
	for {
		r := l.GetRowAtIndex(0)
		if r == nil {
			break
		}
		r.Destroy()
	}
}
func addRow(l *gtk.ListBox, s string) *gtk.Label {
	label := plainLabel(s)
	label.SetSelectable(false)
	label.SetMarginStart(6)
	label.SetMarginEnd(6)
	label.SetMarginTop(4)
	label.SetMarginBottom(4)
	label.SetMaxWidthChars(42)
	label.SetEllipsize(pango.ELLIPSIZE_MIDDLE)
	l.Add(label)
	return label
}
func (a *app) renderGroups() {
	was := a.loading
	a.loading = true
	a.groups.RemoveAll()
	for i, g := range a.project.Groups {
		a.groups.Append(strconv.Itoa(i), fmt.Sprintf("%d: %s", i+1, g.Name))
	}
	a.groups.SetActive(a.group)
	a.loading = was
}
func (a *app) renderTracks() {
	was := a.loading
	a.loading = true
	a.groupName.SetText(a.project.Groups[a.group].Name)
	a.loading = was
	clearList(a.tracks)
	for i, path := range a.project.Groups[a.group].Tracks {
		row := box(false, 8)
		duration := "--:--"
		for _, f := range a.files {
			if f.Path == path && f.Error == "" {
				duration = formatDuration(f.Duration)
				break
			}
		}
		for j, value := range []string{fmt.Sprintf("%02d", i+1), filepath.Base(path), duration, filepath.Dir(path)} {
			l := plainLabel(value)
			l.SetSelectable(false)
			l.SetEllipsize(pango.ELLIPSIZE_MIDDLE)
			l.SetMaxWidthChars(1)
			l.SetTooltipText(value)
			widths := []int{32, 280, 72, 180}
			l.SetSizeRequest(widths[j], -1)
			row.PackStart(l, j == 3, true, 0)
		}
		row.SetMarginTop(7)
		row.SetMarginBottom(7)
		a.tracks.Add(row)
	}
	a.tracks.ShowAll()
}
func (a *app) renderLibrary() {
	query, _ := a.filter.GetText()
	query = strings.ToLower(query)
	folder, _ := a.folder.GetText()
	folder = strings.TrimSpace(folder)
	if filepath.IsAbs(folder) {
		if rel, err := filepath.Rel(a.engine.Root(), folder); err == nil {
			folder = rel
		}
	}
	if folder == "." {
		folder = ""
	}
	clearList(a.library)
	a.visible = nil
	a.libraryLabels = nil
	for _, f := range a.files {
		if !strings.Contains(strings.ToLower(f.Path), query) {
			continue
		}
		if folder != "" && f.Path != folder && !strings.HasPrefix(f.Path, strings.TrimSuffix(folder, "/")+"/") {
			continue
		}
		a.visible = append(a.visible, f)
		a.libraryLabels = append(a.libraryLabels, addRow(a.library, a.libraryDetail(f)))
		a.libraryLabels[len(a.libraryLabels)-1].SetTooltipText(a.libraryDetail(f))
		if f.Error != "" {
			r := a.library.GetRowAtIndex(len(a.visible) - 1)
			r.SetSelectable(false)
			r.SetSensitive(false)
		}
	}
	a.library.ShowAll()
	a.scanStatus.SetText(fmt.Sprintf(a.t("%d / %d files", "%d / %d rakitra"), len(a.visible), len(a.files)))
}
func (a *app) libraryDetail(f core.AudioFile) string {
	if f.Error != "" {
		return f.Path + "  [" + f.Error + "]"
	}
	return fmt.Sprintf(a.t("%s\n%s  |  %d Hz, %d ch", "%s\n%s  |  %d Hz, %d fantsona"), f.Path, formatDuration(f.Duration), f.SampleRate, f.Channels)
}
func (a *app) addSelected() {
	if a.locked() || !a.active {
		return
	}
	var paths []string
	for i, f := range a.visible {
		if r := a.library.GetRowAtIndex(i); r != nil && r.IsSelected() && f.Error == "" {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) == 0 {
		return
	}
	if a.project.Format == "vcd" && len(a.project.Groups[a.group].Tracks)+len(paths) > 98 {
		a.error(fmt.Errorf("%s", a.t("Video CD supports at most 98 tracks", "Raki-peo 98 fara-fahabetsany ho an'ny Video CD")))
		return
	}
	if !appendTracks(&a.project.Groups[a.group], paths) {
		a.error(fmt.Errorf("%s", a.t("At most 99 tracks per group", "Raki-peo 99 fara-fahabetsany isaky ny vondrona")))
		return
	}
	a.renderTracks()
	a.changed()
}
func (a *app) addGroup() {
	if a.locked() || !a.active || a.project.Format == "vcd" || len(a.project.Groups) >= 9 {
		return
	}
	a.project.Groups = append(a.project.Groups, core.Group{Name: groupName(a.lang, len(a.project.Groups)+1), Tracks: []string{}})
	a.group = len(a.project.Groups) - 1
	a.renderGroups()
	a.renderTracks()
	a.changed()
}
func (a *app) deleteGroup() {
	if a.locked() || len(a.project.Groups) <= 1 {
		return
	}
	if len(a.project.Groups[a.group].Tracks) > 0 && !a.confirm(a.t("Delete this group and its track list? Source files will not be changed.", "Hafafa ity vondrona sy lisitry ny raki-peo ity? Tsy ovaina ny rakitra niaviany.")) {
		return
	}
	a.project.Groups = slices.Delete(a.project.Groups, a.group, a.group+1)
	a.group = min(a.group, len(a.project.Groups)-1)
	a.renderGroups()
	a.renderTracks()
	a.changed()
}
func (a *app) move(delta int) {
	if a.locked() {
		return
	}
	r := a.tracks.GetSelectedRow()
	if r == nil {
		return
	}
	i := moveTrack(&a.project.Groups[a.group], r.GetIndex(), delta)
	a.renderTracks()
	a.tracks.SelectRow(a.tracks.GetRowAtIndex(i))
	a.changed()
}
func (a *app) removeTrack() {
	if a.locked() {
		return
	}
	r := a.tracks.GetSelectedRow()
	if r == nil {
		return
	}
	i := r.GetIndex()
	g := &a.project.Groups[a.group]
	g.Tracks = slices.Delete(g.Tracks, i, i+1)
	a.renderTracks()
	if len(g.Tracks) > 0 {
		a.tracks.SelectRow(a.tracks.GetRowAtIndex(min(i, len(g.Tracks)-1)))
	}
	a.changed()
}

func (a *app) scan() {
	if a.locked() {
		return
	}
	if a.scanCancel != nil {
		a.scanCancel()
	}
	a.scanRevision++
	seq := a.scanRevision
	ctx, cancel := context.WithCancel(a.context())
	a.scanCancel = cancel
	a.scanning = true
	a.rescan.SetSensitive(false)
	a.scanStatus.SetText(a.t("Scanning and probing audio...", "Mikaroka sy mandinika feo..."))
	a.work(func() {
		files, err := a.engine.Library(ctx)
		cancel()
		a.idle(func() {
			if seq != a.scanRevision {
				return
			}
			a.scanning = false
			a.scanProgress.SetFraction(0)
			a.rescan.SetSensitive(!a.locked())
			if err != nil {
				if ctx.Err() == context.Canceled && a.ctx.Err() != nil {
					return
				}
				a.scanStatus.SetText(err.Error())
				return
			}
			slices.SortStableFunc(files, func(x, y core.AudioFile) int { return strings.Compare(x.Path, y.Path) })
			a.files = files
			a.renderLibrary()
			a.renderTracks()
		})
	})
}

func (a *app) changed() {
	a.dirty = a.active && !reflect.DeepEqual(a.project, a.saved)
	a.updateTitle()
	a.revision++
	seq := a.revision
	if a.estimateCancel != nil {
		a.estimateCancel()
	}
	a.estimate = nil
	a.capacity.SetFraction(0)
	a.setLocked(a.busy)
	a.estimateError = ""
	a.estimateStatus.SetText(a.t("Estimating...", "Manombana..."))
	if !a.active {
		a.estimateStatus.SetText("")
		return
	}
	glib.TimeoutAdd(350, func() bool {
		if a.closed || seq != a.revision {
			return false
		}
		ctx, cancel := context.WithCancel(a.context())
		a.estimateCancel = cancel
		p := cloneProject(a.project)
		a.work(func() {
			e, err := a.engine.Estimate(ctx, p)
			cancel()
			a.idle(func() {
				if seq != a.revision {
					return
				}
				if err != nil {
					a.estimateError = err.Error()
				} else {
					a.estimate = &e
				}
				a.showEstimate()
			})
		})
		return false
	})
}
func (a *app) showEstimate() {
	a.setLocked(a.busy)
	if a.estimate != nil {
		e := a.estimate
		fraction := 0.0
		if e.CapacityBytes > 0 {
			fraction = float64(e.EstimatedBytes) / float64(e.CapacityBytes)
		}
		a.capacity.SetFraction(min(1, fraction))
		ctx, _ := a.capacity.GetStyleContext()
		ctx.RemoveClass("near-full")
		ctx.RemoveClass("over-full")
		if !e.Fits {
			ctx.AddClass("over-full")
		} else if fraction > .9 {
			ctx.AddClass("near-full")
		}
		fit := a.t("fits", "antonona")
		if !e.Fits {
			fit = a.t("TOO LARGE", "LEHIBE LOATRA")
		}
		a.estimateStatus.SetText(fmt.Sprintf(a.t("Duration %.1f min | Disc %.2f / %.2f GB (%s) | Workspace %.2f GB", "Faharetana %.1f min | Kapila %.2f / %.2f GB (%s) | Toerana ilaina %.2f GB"), e.Duration/60, float64(e.EstimatedBytes)/1e9, float64(e.CapacityBytes)/1e9, fit, float64(e.WorkspaceBytes)/1e9))
		count := 0
		for _, group := range a.project.Groups {
			count += len(group.Tracks)
		}
		text, _ := a.estimateStatus.GetText()
		a.estimateStatus.SetText(fmt.Sprintf(a.t("%d tracks | ", "%d raki-peo | "), count) + text)
	} else if a.estimateError != "" {
		a.estimateStatus.SetText(a.estimateError)
	}
}
func (a *app) refreshTools() {
	ctx := a.context()
	lang := a.lang
	a.work(func() {
		tools := a.engine.Tools(ctx)
		a.idle(func() {
			if lang != a.lang {
				return
			}
			var parts []string
			for _, t := range tools {
				state := a.t("ready", "vonona")
				if !t.Ready {
					state = a.t("missing/incompatible", "tsy hita/tsy mifanaraka")
				}
				parts = append(parts, t.Name+": "+state+" ("+t.Detail+")")
			}
			a.tools.SetText(strings.Join(parts, " | "))
		})
	})
}
func (a *app) setLanguage(lang string) {
	if lang != "en" && lang != "plt" && lang != "fr" {
		return
	}
	a.lang = lang
	for _, t := range a.translations {
		t.set(a.t(t.en, t.mg))
	}
	for i, label := range a.libraryLabels {
		label.SetText(a.libraryDetail(a.visible[i]))
		label.SetTooltipText(a.libraryDetail(a.visible[i]))
	}
	a.loadWidgets()
	if a.recorder.GetActiveID() == "" {
		a.recorder.RemoveAll()
		a.recorder.Append("", a.t("No recorder found", "Tsy hita ny mpanoratra kapila"))
		a.recorder.SetActive(0)
	}
	a.recorder.SetTooltipText(a.t("Disc recorder", "Mpanoratra kapila"))
	a.speed.SetTooltipText(a.t("Write speed (Nx)", "Hafainganam-panoratana (Nx)"))
	a.refreshTools()
	a.showEstimate()
	a.showJobText()
	a.updateTitle()
	if a.scanning {
		a.scanStatus.SetText(a.t("Scanning and probing audio...", "Mikaroka sy mandinika feo..."))
	} else {
		a.scanStatus.SetText(fmt.Sprintf(a.t("%d / %d files", "%d / %d rakitra"), len(a.visible), len(a.files)))
	}
}

func (a *app) start() {
	if a.smokeMode || !a.active || a.locked() || a.estimate == nil || !a.estimate.Fits {
		return
	}
	a.jobRevision++
	a.startError = ""
	a.pending = true
	a.setLocked(false)
	a.showJobText()
	if a.scanCancel != nil {
		a.scanCancel()
		a.scanRevision++
		a.scanning = false
		a.scanProgress.SetFraction(0)
	}
	ctx, cancel := context.WithCancel(a.context())
	a.startCancel = cancel
	p := cloneProject(a.project)
	a.work(func() {
		j, err := a.engine.Start(ctx, p)
		a.idle(func() {
			a.pending = false
			if err != nil {
				cancel()
				a.setLocked(false)
				a.startError = err.Error()
				a.showJobText()
				return
			}
			a.authorJobID = j.ID
			a.authorProject = p
			a.showJob(j)
		})
	})
}
func (a *app) cancelJob() {
	if !a.locked() || a.canceling {
		return
	}
	if a.startCancel != nil {
		a.startCancel()
	}
	a.canceling = true
	a.setLocked(a.busy)
	a.work(func() { _ = a.engine.Cancel(); a.idle(func() { a.canceling = false; a.setLocked(a.busy) }) })
}
func (a *app) showJob(j *core.Job) {
	// A snapshot taken before Start returns must not unlock a pending submission.
	if a.pending {
		return
	}
	a.lastJob = j
	a.rememberAuthoredImage(j)
	a.setLocked(j != nil && j.State == "running")
	a.showJobText()
	if a.quitAfterJob && !a.locked() {
		a.quitAfterJob = false
		a.quit()
	}
}
func (a *app) showJobText() {
	if a.pending {
		a.jobStatus.SetText(a.t("Validating and starting...", "Manamarina sy manomboka..."))
		return
	}
	if a.startError != "" {
		a.jobStatus.SetText(a.startError)
		return
	}
	j := a.lastJob
	if j == nil {
		a.jobStatus.SetText(a.t("Ready. Originals are never modified.", "Vonona. Tsy ovaina mihitsy ny rakitra niaviany."))
		return
	}
	state := j.State
	if a.lang == "fr" {
		if s, ok := french[state]; ok {
			state = s
		}
	}
	if a.lang == "plt" {
		if s, ok := map[string]string{"running": "mandeha", "completed": "vita", "failed": "tsy nahomby", "cancelled": "nofoanana"}[state]; ok {
			state = s
		}
	}
	a.jobStatus.SetText(strings.TrimSpace(state + " " + j.Output + " " + j.Error))
	text := strings.Join(j.Log, "\n")
	if text != a.lastLog {
		a.lastLog = text
		a.logBuffer.SetText(text)
	}
}

func (a *app) chooseProject(save bool) bool {
	if a.locked() || (save && !a.active) {
		return false
	}
	action := gtk.FILE_CHOOSER_ACTION_OPEN
	title := a.t("Import project JSON", "Hanafatra tetikasa JSON")
	accept := a.t("Open", "Sokafy")
	if save {
		action = gtk.FILE_CHOOSER_ACTION_SAVE
		title = a.t("Export project JSON", "Hanondrana tetikasa JSON")
		accept = a.t("Save", "Tehirizo")
	}
	d, err := gtk.FileChooserDialogNewWith2Buttons(title, a.window, action, a.t("Cancel", "Ajanony"), gtk.RESPONSE_CANCEL, accept, gtk.RESPONSE_ACCEPT)
	if err != nil {
		a.error(err)
		return false
	}
	defer d.Destroy()
	f, _ := gtk.FileFilterNew()
	f.SetName(a.t("Project JSON (*.json)", "Tetikasa JSON (*.json)"))
	f.AddPattern("*.json")
	d.AddFilter(f)
	if save {
		d.SetCurrentName(a.t("project.json", "tetikasa.json"))
		d.SetDoOverwriteConfirmation(true)
	}
	if d.Run() != gtk.RESPONSE_ACCEPT {
		return false
	}
	path := d.GetFilename()
	d.Hide()
	if save {
		if !strings.EqualFold(filepath.Ext(path), ".json") {
			a.error(fmt.Errorf("%s", a.t("Use a .json filename", "Ampiasao ny anaran-drakitra .json")))
			return false
		}
		return a.saveTo(path)
	} else {
		return a.openDocument(path)
	}
}
func (a *app) smoke() {
	// Synthetic paths only: exercise the same widget callbacks without authoring.
	if !a.active && (a.stack.GetVisibleChildName() != "welcome" || a.actions["save"].GetSensitive() || a.actions["burn"].GetSensitive()) {
		a.smokeError = fmt.Errorf("GTK smoke welcome action state failed")
	}
	if a.active && a.stack.GetVisibleChildName() != "workspace" {
		a.smokeError = fmt.Errorf("GTK smoke startup project not visible")
	}
	a.actions["new-dvd"].Activate()
	a.files = []core.AudioFile{{Path: "smoke-a.wav", Duration: 1, SampleRate: 48000, Channels: 2}, {Path: "smoke-b.wav", Duration: 2, SampleRate: 48000, Channels: 2}, {Path: "invalid.wav", Error: "Synthetic probe error"}}
	a.renderLibrary()
	a.library.SelectRow(a.library.GetRowAtIndex(0))
	a.library.SelectRow(a.library.GetRowAtIndex(1))
	a.library.SelectRow(a.library.GetRowAtIndex(2))
	if a.library.GetRowAtIndex(2).IsSelected() || a.library.GetRowAtIndex(2).GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke invalid source row selectable")
	}
	a.addSelected()
	a.addSelected()
	a.tracks.SelectRow(a.tracks.GetRowAtIndex(3))
	a.removeTrack()
	a.tracks.SelectRow(a.tracks.GetRowAtIndex(2))
	a.removeTrack()
	a.tracks.SelectRow(a.tracks.GetRowAtIndex(1))
	a.move(-1)
	a.addGroup()
	a.groupName.SetText("Smoke group")
	a.groups.SetActive(0)
	before := cloneProject(a.project)
	// Locale changes must not invalidate an estimate or alter document/job state.
	estimate := &core.Estimate{Fits: true}
	a.estimate = estimate
	job := &core.Job{State: "running", Log: []string{"raw external diagnostic"}}
	a.lastJob = job
	dirty, revision, pathBefore := a.dirty, a.revision, a.projectPath
	for _, lang := range []string{"en", "fr", "plt"} {
		a.language.SetActiveID(lang)
		if !reflect.DeepEqual(a.project, before) || a.dirty != dirty || a.revision != revision || a.projectPath != pathBefore || a.estimate != estimate || a.lastJob != job {
			a.smokeError = fmt.Errorf("GTK smoke %s switching changed document, estimate or job", lang)
		}
		for _, binding := range a.translations {
			if lang == "fr" && binding.en != binding.mg {
				if _, ok := french[binding.en]; !ok {
					a.smokeError = fmt.Errorf("GTK smoke untranslated French label: %s", binding.en)
				}
			}
		}
	}
	a.estimate = nil
	a.lastJob = nil
	a.showEstimate()
	for _, lang := range []string{"en", "fr", "plt"} {
		a.language.SetActiveID(lang)
		// Respond on the nested modal main loop, never touching a drive.
		respond := func(response gtk.ResponseType) {
			glib.TimeoutAdd(50, func() bool {
				gtk.WindowListToplevels().Foreach(func(value interface{}) {
					w := value.(*gtk.Window)
					if w.GetVisible() && w.GetModal() {
						w.Emit("response", glib.TYPE_NONE, int(response))
					}
				})
				return false
			})
		}
		respond(gtk.RESPONSE_CLOSE)
		a.actions["about"].Activate()
		respond(gtk.RESPONSE_NO)
		if a.confirm(a.t("Delete this group and its track list? Source files will not be changed.", "Hafafa ity vondrona sy lisitry ny raki-peo ity? Tsy ovaina ny rakitra niaviany.")) {
			a.smokeError = fmt.Errorf("GTK smoke confirmation response failed")
		}
		respond(gtk.RESPONSE_CLOSE)
		a.error(fmt.Errorf("empty track path"))
	}
	a.language.SetActiveID("plt")
	detail, _ := a.libraryLabels[0].GetText()
	if !a.library.GetRowAtIndex(0).IsSelected() || !a.library.GetRowAtIndex(1).IsSelected() || !strings.Contains(detail, "fantsona") {
		a.smokeError = fmt.Errorf("GTK smoke library language/selection validation failed")
	}
	a.addGroup()
	if a.project.Groups[2].Name != "Vondrona 3" {
		a.smokeError = fmt.Errorf("GTK smoke localized group name validation failed")
	}
	a.deleteGroup()
	a.groups.SetActive(0)
	a.language.SetActiveID("en")
	if a.build.GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke build enabled without estimate")
	}
	a.estimate = &core.Estimate{Fits: true}
	a.showEstimate()
	if !a.build.GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke fitting estimate did not enable build")
	}
	a.pending = true
	a.setLocked(false)
	if a.edit.GetSensitive() || !a.language.GetSensitive() || a.build.GetSensitive() || a.actions["new-vcd"].GetSensitive() || a.actions["save"].GetSensitive() || a.welcome.GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke edit-lock validation failed")
	}
	a.pending = false
	a.setLocked(false)
	a.estimate.Fits = false
	a.showEstimate()
	if a.build.GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke oversized estimate enabled build")
	}
	a.changed()
	if a.project.Groups[0].Tracks[0] != "smoke-b.wav" || a.project.Groups[1].Name != "Smoke group" || a.project.Name != before.Name || a.group != 0 {
		a.smokeError = fmt.Errorf("GTK smoke state validation failed")
	}
	dir, err := os.MkdirTemp("", "kapila-maker-smoke-")
	if err != nil {
		a.smokeError = err
		a.shutdown()
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "project.json")
	if !a.dirty || !a.saveTo(path) || a.dirty || a.projectPath != path {
		a.smokeError = fmt.Errorf("GTK smoke save lifecycle failed")
	}
	a.language.SetActiveID("plt")
	a.language.SetActiveID("fr")
	a.language.SetActiveID("en")
	if a.dirty {
		a.smokeError = fmt.Errorf("GTK smoke language dirtied project")
	}
	a.name.SetText("Saved smoke project")
	// Respond to the real modal prompt on its nested GTK main loop.
	respond := func(response gtk.ResponseType) {
		glib.TimeoutAdd(50, func() bool {
			windows := gtk.WindowListToplevels()
			windows.Foreach(func(value interface{}) {
				w := value.(*gtk.Window)
				title, _ := w.GetTitle()
				if title == a.t("Unsaved Project", "Tetikasa tsy voatahiry") {
					w.Emit("response", glib.TYPE_NONE, int(response))
				}
			})
			return false
		})
	}
	respond(gtk.RESPONSE_CANCEL)
	a.actions["close"].Activate()
	if !a.active || !a.dirty {
		a.smokeError = fmt.Errorf("GTK smoke cancel discarded document")
	}
	respond(gtk.RESPONSE_ACCEPT)
	if !a.mayLeave() || a.dirty {
		a.smokeError = fmt.Errorf("GTK smoke prompt save failed")
	}
	p, err := readProject(path)
	if err != nil || p.Name != "Saved smoke project" {
		a.smokeError = fmt.Errorf("GTK smoke Save did not remember path: %v", err)
	}
	a.name.SetText("Discarded")
	respond(gtk.RESPONSE_REJECT)
	a.actions["new-vcd"].Activate()
	if a.project.Format != "vcd" || a.project.Capacity != "cd80" || a.project.VideoStandard != "pal" || len(a.project.Groups) != 1 || !a.dirty || a.actions["group-add"].GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke VCD creation failed")
	}
	a.standard.SetActiveID("ntsc")
	a.media.SetActiveID("cd74")
	if a.project.VideoStandard != "ntsc" || a.project.Capacity != "cd74" {
		a.smokeError = fmt.Errorf("GTK smoke VCD profile callbacks failed")
	}
	a.addGroup()
	if len(a.project.Groups) != 1 {
		a.smokeError = fmt.Errorf("GTK smoke VCD group limit failed")
	}
	if !a.saveTo(path) {
		a.smokeError = fmt.Errorf("GTK smoke VCD save failed")
	}
	a.actions["close"].Activate()
	if a.active || a.stack.GetVisibleChildName() != "welcome" {
		a.smokeError = fmt.Errorf("GTK smoke close failed")
	}
	if !a.openDocument(path) || !a.active || a.dirty || a.project.Format != "vcd" || a.projectPath != path {
		a.smokeError = fmt.Errorf("GTK smoke Open lifecycle failed")
	}
	a.actions["close"].Activate()
	a.actions["new-dvd"].Activate()
	a.files = a.files[:2]
	a.renderLibrary()
	a.library.SelectRow(a.library.GetRowAtIndex(0))
	a.addSelected()
	// Only inspect eligibility; never activate Burn or run drive simulation.
	a.authorJobID = "smoke-image"
	a.authorProject = cloneProject(a.project)
	a.showJob(&core.Job{ID: "smoke-image", State: "completed", Format: "vcd", Kind: "author", Output: "synthetic.cue"})
	a.recorder.RemoveAll()
	a.recorder.Append("", "No recorder")
	a.recorder.SetActive(0)
	a.updateActions()
	if a.actions["burn"].GetSensitive() || a.actionButtons["burn"].GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke burn enabled without a recorder")
	}
	a.recorder.RemoveAll()
	a.recorder.Append("/dev/smoke-not-a-drive", "Synthetic recorder")
	a.recorder.SetActive(0)
	choices := recorderChoices(a.recorder)
	if choices.GetIDColumn() != 1 || choices.GetActiveID() != "/dev/smoke-not-a-drive" || choices.GetActiveText() != "Synthetic recorder" {
		a.smokeError = fmt.Errorf("GTK smoke recorder model lost device ID")
	}
	modalSpeed := speedCombo()
	if a.speed.GetActiveID() != "8" || modalSpeed.GetActiveID() != "8" {
		a.smokeError = fmt.Errorf("GTK smoke default burn speed is not 8x")
	}
	for _, id := range []string{"1", "2", "4", "8", "16", "24", "48"} {
		a.speed.SetActiveID(id)
		modalSpeed.SetActiveID(a.speed.GetActiveID())
		request, err := makeBurnRequest(a.burnSource.ID, choices.GetActiveID(), modalSpeed.GetActiveID(), false)
		if err != nil || request.Speed < 1 || request.Speed > 52 || request.Device == "" || request.JobID != "smoke-image" || request.Confirmed || modalSpeed.GetActiveText() != id+"x" {
			a.smokeError = fmt.Errorf("GTK smoke invalid burn request: %+v %v", request, err)
		}
	}
	if a.speed.SetActiveID("0") || modalSpeed.SetActiveID("0") {
		a.smokeError = fmt.Errorf("GTK smoke automatic speed still selectable")
	}
	a.speed.SetActiveID("8")
	choices.Destroy()
	modalSpeed.Destroy()
	if !a.actions["burn"].GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke eligible VCD image disabled")
	}
	for _, state := range []string{"completed", "failed", "cancelled", "completed"} {
		a.showJob(&core.Job{ID: "smoke-burn", State: state, Format: "vcd", Kind: "burn", Output: "synthetic.cue"})
		request, err := makeBurnRequest(a.burnSource.ID, a.recorder.GetActiveID(), a.speed.GetActiveID(), false)
		if !a.actions["burn"].GetSensitive() || err != nil || request.JobID != "smoke-image" {
			a.smokeError = fmt.Errorf("GTK smoke %s burn lost original author source", state)
		}
	}
	a.project.Name += " edited"
	a.updateActions()
	if a.actions["burn"].GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke changed document can burn stale image")
	}
	a.project = cloneProject(a.burnProject)
	a.burnSource = nil
	a.updateActions()
	if a.actions["burn"].GetSensitive() {
		a.smokeError = fmt.Errorf("GTK smoke burn enabled without source")
	}
	a.lastJob = nil
	a.changed()
	log.Print("GTK smoke: welcome, Project menus, DVD/VCD profiles, atomic Save, Save/Discard/Cancel, edit lock, invalid rows, ordering, en/fr/plt switching, About/confirmation/error dialogs, positive burn speeds, recorder IDs and missing-device guard; no authoring, burn or drive simulation")
	glib.TimeoutAdd(1000, func() bool {
		if a.estimateError == "" {
			a.smokeError = fmt.Errorf("GTK smoke: asynchronous estimate did not reject synthetic source paths")
		}
		if a.build.GetSensitive() {
			a.smokeError = fmt.Errorf("GTK smoke rejected estimate enabled build")
		}
		a.shutdown()
		return false
	})
}
