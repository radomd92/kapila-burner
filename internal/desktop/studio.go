//go:build gtk

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"dvd-audio-studio/internal/core"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

func style(w gtk.IWidget, name string) {
	c, _ := w.ToWidget().GetStyleContext()
	c.AddClass(name)
}

func formatDuration(seconds float64) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func speedCombo() *gtk.ComboBoxText {
	c := combo()
	for _, speed := range []string{"1", "2", "4", "8", "16", "24", "48"} {
		c.Append(speed, speed+"x")
	}
	c.SetActiveID("8")
	return c
}

func recorderChoices(source *gtk.ComboBoxText) *gtk.ComboBoxText {
	c := combo()
	model, _ := source.GetModel()
	c.SetModel(model)
	c.SetIDColumn(source.GetIDColumn())
	c.SetActiveID(source.GetActiveID())
	return c
}

// Confirmation is deliberately left false until the final warning is accepted.
func makeBurnRequest(jobID, device, speed string, dummy bool) (core.BurnRequest, error) {
	s, err := strconv.Atoi(speed)
	if err != nil || s < 1 || s > 52 {
		return core.BurnRequest{}, fmt.Errorf("Write speed must be between 1x and 52x")
	}
	if jobID == "" || strings.TrimSpace(device) == "" {
		return core.BurnRequest{}, fmt.Errorf("Select a completed image and recorder first")
	}
	return core.BurnRequest{JobID: jobID, Device: device, Speed: s, Dummy: dummy}, nil
}

func (a *app) construct() {
	a.window, _ = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	a.window.SetDefaultSize(1220, 860)
	a.window.SetPosition(gtk.WIN_POS_CENTER)
	a.window.Connect("delete-event", func() bool { a.quit(); return true })
	style(a.window, "disc-studio")
	css, _ := gtk.CssProviderNew()
	if err := css.LoadFromData(studioCSS); err == nil {
		screen, _ := gdk.ScreenGetDefault()
		gtk.AddProviderForScreen(screen, css, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}
	outer := box(true, 0)
	a.window.Add(outer)
	a.actions = make(map[string]*gtk.MenuItem)
	a.actionButtons = make(map[string]*gtk.Button)
	accels, _ := gtk.AccelGroupNew()
	a.window.AddAccelGroup(accels)
	bar, _ := gtk.MenuBarNew()
	style(bar, "studio-menubar")
	outer.PackStart(bar, false, false, 0)
	menu := func(en, mg string) *gtk.Menu {
		item, _ := gtk.MenuItemNew()
		a.bind(en, mg, item.SetLabel)
		m, _ := gtk.MenuNew()
		item.SetSubmenu(m)
		bar.Append(item)
		return m
	}
	item := func(m *gtk.Menu, id, en, mg, key string, f func()) {
		w, _ := gtk.MenuItemNew()
		a.bind(en, mg, w.SetLabel)
		w.Connect("activate", f)
		m.Append(w)
		a.actions[id] = w
		if key != "" {
			k, mods := gtk.AcceleratorParse(key)
			w.AddAccelerator("activate", accels, k, mods, gtk.ACCEL_VISIBLE)
		}
	}
	sep := func(m *gtk.Menu) { s, _ := gtk.SeparatorMenuItemNew(); m.Append(s) }
	p := menu("Project", "Tetikasa")
	item(p, "new-dvd", "New DVD-Audio", "DVD-Audio vaovao", "<Primary>n", func() { a.newDocument("dvd-audio") })
	item(p, "new-vcd", "New Video CD", "Video CD vaovao", "<Primary><Shift>n", func() { a.newDocument("vcd") })
	sep(p)
	item(p, "open", "Open...", "Sokafy...", "<Primary>o", func() { a.chooseProject(false) })
	item(p, "save", "Save", "Tehirizo", "<Primary>s", func() { a.save() })
	item(p, "save-as", "Save As...", "Tehirizo amin'ny anarana...", "<Primary><Shift>s", func() { a.chooseProject(true) })
	sep(p)
	item(p, "close", "Close Project", "Akatona ny tetikasa", "<Primary>w", a.closeDocument)
	item(p, "quit", "Quit", "Hivoaka", "<Primary>q", a.quit)
	e := menu("Edit", "Fanovana")
	item(e, "add", "Add Selected", "Ampidiro ny voafidy", "<Primary>Return", a.addSelected)
	item(e, "remove", "Remove Track", "Esory ny raki-peo", "", a.removeTrack)
	sep(e)
	item(e, "up", "Move Up", "Afindrao miakatra", "<Alt>Up", func() { a.move(-1) })
	item(e, "down", "Move Down", "Afindrao midina", "<Alt>Down", func() { a.move(1) })
	sep(e)
	item(e, "group-add", "Add Group", "Ampio vondrona", "", a.addGroup)
	item(e, "group-delete", "Delete Group", "Fafao ny vondrona", "", a.deleteGroup)
	d := menu("Disc", "Kapila")
	item(d, "build", "Create Image", "Hamorona sary kapila", "<Primary>b", a.start)
	item(d, "burn", "Burn VCD Image...", "Handoro sary VCD...", "", a.burn)
	sep(d)
	item(d, "recorders", "Refresh Recorders", "Havaozy ny mpanoratra kapila", "", a.refreshRecorders)
	item(d, "refresh", "Scan Folder", "Hikaroka ao amin'ny lahatahiry", "F5", a.scan)
	v := menu("View", "Fijery")
	item(v, "malagasy", "Malagasy (Plateau)", "Malagasy (Plateau)", "", func() { a.language.SetActiveID("plt") })
	item(v, "english", "English", "English", "", func() { a.language.SetActiveID("en") })
	item(v, "french", "Français", "Français", "", func() { a.language.SetActiveID("fr") })
	sep(v)
	item(v, "log", "Show / Hide Job Log", "Asehoy / afeno ny tatitra", "<Primary>l", func() { a.logDock.SetExpanded(!a.logDock.GetExpanded()) })
	h := menu("Help", "Fanampiana")
	item(h, "about", "About kapila-maker", "Momba ny kapila-maker", "", func() {
		d := gtk.MessageDialogNew(a.window, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_NONE, "%s", a.t("kapila-maker\nNative DVD-Audio & Video CD authoring\n\nDVD: lossless PCM for DVD-Audio players.\nVCD: MPEG-1 video + MP2 sound; audio uses a black picture.\nCreate an image first. Burning always requires confirmation.\nSource files are never modified.", "kapila-maker\nFanamboarana DVD-Audio sy Video CD\n\nDVD: PCM tsy very kalitao ho an'ny mpamaky DVD-Audio.\nVCD: MPEG-1 sy MP2; sary mainty ho an'ny feo.\nMamorona sary kapila aloha. Mila fanekena ny fandoroana.\nTsy ovaina ny rakitra niaviany."))
		d.AddButton(a.t("Close", "Akatona"), gtk.RESPONSE_CLOSE)
		defer a.destroyDialog(&d.Dialog)
		a.runDialog(&d.Dialog)
	})
	toolbar := box(false, 4)
	toolbar.SetBorderWidth(8)
	style(toolbar, "studio-toolbar")
	outer.PackStart(toolbar, false, false, 0)
	tool := func(id, icon, en, mg string) {
		b := a.button(en, mg, func() { a.actions[id].Activate() })
		image, _ := gtk.ImageNewFromIconName(icon, gtk.ICON_SIZE_LARGE_TOOLBAR)
		b.SetImage(image)
		b.SetAlwaysShowImage(true)
		style(b, "studio-tool")
		a.actionButtons[id] = b
		toolbar.PackStart(b, false, false, 0)
	}
	divider := func() {
		s, _ := gtk.SeparatorNew(gtk.ORIENTATION_VERTICAL)
		s.SetMarginStart(6)
		s.SetMarginEnd(6)
		toolbar.PackStart(s, false, false, 0)
	}
	tool("new-dvd", "document-new", "New", "Vaovao")
	tool("open", "document-open", "Open", "Sokafy")
	tool("save", "document-save", "Save", "Tehirizo")
	divider()
	tool("add", "list-add", "Add", "Ampidiro")
	tool("remove", "list-remove", "Remove", "Esory")
	divider()
	tool("build", "media-optical", "Create Image", "Sary kapila")
	style(a.actionButtons["build"], "primary")
	tool("burn", "media-record", "Burn VCD Image...", "Handoro sary VCD...")
	spacer := box(false, 0)
	toolbar.PackStart(spacer, true, true, 0)
	a.language = combo()
	a.language.Append("plt", "Malagasy (Plateau)")
	a.language.Append("en", "English")
	a.language.Append("fr", "Français")
	a.language.SetActiveID(a.lang)
	a.language.Connect("changed", func() { a.setLanguage(a.language.GetActiveID()) })
	toolbar.PackStart(a.language, false, false, 0)
	dependencies := box(false, 8)
	dependencies.SetBorderWidth(8)
	dependencies.PackStart(a.button("External Tools", "Fitaovana ivelany", a.showTools), false, false, 0)
	a.tools = plainLabel("")
	a.tools.SetLineWrap(true)
	a.tools.SetMaxWidthChars(90)
	dependencies.PackStart(a.tools, true, true, 0)
	a.refreshTools()
	outer.PackStart(dependencies, false, false, 0)
	a.stack, _ = gtk.StackNew()
	outer.PackStart(a.stack, true, true, 0)
	a.welcome = box(true, 18)
	a.welcome.SetBorderWidth(48)
	style(a.welcome, "welcome")
	brand := a.label("kapila-maker", "kapila-maker")
	style(brand, "eyebrow")
	a.welcome.PackStart(brand, false, false, 0)
	title := a.label("Your music. A disc worth keeping.", "Ny mozikanao. Kapila mendrika hotehirizina.")
	title.SetLineWrap(true)
	style(title, "welcome-title")
	a.welcome.PackStart(title, false, false, 0)
	sub := a.label("Choose a compilation to begin, or open an existing project.", "Misafidiana karazana kapila hanombohana, na sokafy ny tetikasa efa misy.")
	sub.SetLineWrap(true)
	a.welcome.PackStart(sub, false, false, 0)
	tiles := box(false, 20)
	tiles.SetHomogeneous(true)
	for _, spec := range []struct{ id, icon, en, mg, detail, detailMG string }{
		{"new-dvd", "media-optical-dvd", "DVD Audio", "DVD Audio", "LOSSLESS LISTENING\nHigh-resolution PCM, up to 9 groups.\nDVD-5 / DVD-9 image authoring.", "FEO TSY VERY KALITAO\nPCM avo kalitao, vondrona 9 hatramin'ny.\nSary kapila DVD-5 / DVD-9."},
		{"new-vcd", "media-optical-cd", "Video CD", "Video CD", "CLASSIC VIDEO DISC\nMusic or video, PAL / NTSC.\n74 / 80 minute CD, CUE + BIN image.", "KAPILA VIDEO MAHAZATRA\nFeo na video, PAL / NTSC.\nCD 74 / 80 minitra, sary CUE + BIN."},
	} {
		id := spec.id
		b, _ := gtk.ButtonNew()
		style(b, "project-tile")
		content := box(true, 16)
		content.SetBorderWidth(24)
		image, _ := gtk.ImageNewFromIconName(spec.icon, gtk.ICON_SIZE_DIALOG)
		image.SetHAlign(gtk.ALIGN_START)
		content.PackStart(image, false, false, 0)
		l := a.label(spec.en, spec.mg)
		style(l, "tile-title")
		content.PackStart(l, false, false, 0)
		text := a.label(spec.detail, spec.detailMG)
		text.SetLineWrap(true)
		content.PackStart(text, false, false, 0)
		b.Add(content)
		b.Connect("clicked", func() { a.actions[id].Activate() })
		tiles.PackStart(b, true, true, 0)
	}
	a.welcome.PackStart(tiles, false, true, 12)
	open := a.button("Open an existing project...", "Sokafy ny tetikasa efa misy...", func() { a.actions["open"].Activate() })
	open.SetHAlign(gtk.ALIGN_START)
	a.welcome.PackStart(open, false, false, 0)
	note := a.label("IMAGE FIRST. BURN ONLY WHEN YOU CHOOSE.\nYour source files stay untouched.", "SARY KAPILA ALOHA. DORANA RAHA SAFIDIANAO IHANY.\nTsy ovaina ny rakitra niaviany.")
	note.SetLineWrap(true)
	style(note, "welcome-note")
	a.welcome.PackEnd(note, false, false, 0)
	a.stack.AddNamed(a.welcome, "welcome")
	a.stack.AddNamed(a.constructEditor(), "workspace")
	if a.active {
		a.stack.SetVisibleChildName("workspace")
	} else {
		a.stack.SetVisibleChildName("welcome")
	}
	a.updateTitle()
	a.refreshRecorders()
}

func (a *app) updateTitle() {
	title := a.t("kapila-maker", "kapila-maker")
	if a.active {
		title = a.project.Name + " - " + title
		if a.dirty {
			title = "* " + title
		}
	}
	a.window.SetTitle(title)
}

func (a *app) updateActions() {
	if a.actions == nil {
		return
	}
	free := !a.locked()
	for id, item := range a.actions {
		enabled := free
		switch id {
		case "english", "malagasy", "french", "log", "about", "quit":
			enabled = true
		case "save", "save-as", "close", "add", "remove", "up", "down":
			enabled = free && a.active
		case "group-add":
			enabled = free && a.active && a.project.Format != "vcd" && len(a.project.Groups) < 9
		case "group-delete":
			enabled = free && a.active && len(a.project.Groups) > 1
		case "build":
			enabled = free && a.active && a.estimate != nil && a.estimate.Fits
		case "burn":
			enabled = free && a.burnReady() && a.recorder != nil && a.recorder.GetActiveID() != ""
		case "refresh":
			enabled = free && a.active && !a.scanning
		}
		item.SetSensitive(enabled)
		if b := a.actionButtons[id]; b != nil {
			b.SetSensitive(enabled)
		}
	}
	if a.welcome != nil {
		a.welcome.SetSensitive(free)
	}
	if a.recorder != nil {
		a.recorder.SetSensitive(free)
		a.speed.SetSensitive(free)
	}
}

func (a *app) newDocument(format string) {
	if a.locked() || !a.mayLeave() {
		return
	}
	a.burnSource = nil
	a.authorJobID = ""
	a.project = newProject(a.lang)
	a.project.Format = format
	if format == "vcd" {
		a.project.Capacity = "cd80"
		a.project.VideoStandard = "pal"
		a.project.Name = a.t("Untitled Video CD", "Video CD tsy misy anarana")
	}
	a.saved = core.Project{}
	a.projectPath = ""
	a.active = true
	a.group = 0
	a.loadWidgets()
	a.stack.SetVisibleChildName("workspace")
	a.changed()
}

func (a *app) saveTo(path string) bool {
	// Validate the document without probing sources, so offline projects remain saveable.
	b, err := json.Marshal(a.project)
	if err == nil {
		_, err = decodeProject(strings.NewReader(string(b)))
	}
	if err == nil {
		err = exportProject(path, a.engine.Root(), a.project)
	}
	if err != nil {
		a.error(err)
		return false
	}
	a.projectPath = path
	a.saved = cloneProject(a.project)
	a.dirty = false
	a.updateTitle()
	return true
}

func (a *app) save() bool {
	if !a.active || a.locked() {
		return false
	}
	if a.projectPath == "" {
		return a.chooseProject(true)
	}
	return a.saveTo(a.projectPath)
}

func (a *app) openDocument(path string) bool {
	if a.locked() {
		return false
	}
	p, err := readProject(path)
	if err != nil {
		a.error(err)
		return false
	}
	if !a.mayLeave() {
		return false
	}
	a.burnSource = nil
	a.authorJobID = ""
	a.project = p
	a.active = true
	a.projectPath = path
	a.saved = cloneProject(p)
	a.group = 0
	a.loadWidgets()
	a.stack.SetVisibleChildName("workspace")
	a.changed()
	return true
}

func (a *app) mayLeave() bool {
	if !a.active || !a.dirty {
		return true
	}
	d := gtk.MessageDialogNew(a.window, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE, "%s", a.t("Save changes to this project before continuing?", "Tehirizina ve ny fanovana alohan'ny hanohizana?"))
	d.SetTitle(a.t("Unsaved Project", "Tetikasa tsy voatahiry"))
	d.AddButton(a.t("Cancel", "Ajanony"), gtk.RESPONSE_CANCEL)
	d.AddButton(a.t("Discard", "Aza tehirizina"), gtk.RESPONSE_REJECT)
	d.AddButton(a.t("Save", "Tehirizo"), gtk.RESPONSE_ACCEPT)
	d.SetDefaultResponse(gtk.RESPONSE_CANCEL)
	r := a.runDialog(&d.Dialog)
	a.destroyDialog(&d.Dialog)
	return r == gtk.RESPONSE_REJECT || (r == gtk.RESPONSE_ACCEPT && a.save())
}

func (a *app) closeDocument() {
	if a.locked() || !a.mayLeave() {
		return
	}
	a.burnSource = nil
	a.authorJobID = ""
	a.active = false
	a.dirty = false
	a.projectPath = ""
	a.stack.SetVisibleChildName("welcome")
	a.changed()
}

func (a *app) quit() {
	if a.locked() {
		if !a.confirm(a.t("Cancel the active job and quit? Cancelling a burn can spoil the disc.", "Ajanona ny asa ary hivoaka? Mety hanimba kapila ny fanafoanana fandoroana.")) {
			return
		}
		// Wait for cancellation before offering Save/Discard/Cancel.
		a.quitAfterJob = true
		a.cancelJob()
		return
	}
	if a.mayLeave() {
		a.shutdown()
	}
}

func (a *app) refreshRecorders() {
	if a.locked() {
		return
	}
	a.recorderRevision++
	seq := a.recorderRevision
	ctx := a.context()
	a.work(func() {
		recorders, err := a.engine.Recorders(ctx)
		a.idle(func() {
			if seq != a.recorderRevision {
				return
			}
			selected := a.recorder.GetActiveID()
			a.recorder.RemoveAll()
			for _, r := range recorders {
				a.recorder.Append(r.Device, r.Name+" ("+r.Device+")")
			}
			if !a.recorder.SetActiveID(selected) {
				a.recorder.SetActive(0)
			}
			if len(recorders) == 0 {
				a.recorder.Append("", a.t("No recorder found", "Tsy hita ny mpanoratra kapila"))
				a.recorder.SetActive(0)
			}
			if err != nil {
				a.recorder.SetTooltipText(err.Error())
			}
			a.updateActions()
		})
	})
}

func (a *app) rememberAuthoredImage(j *core.Job) {
	// Only our submitted author job may supply an image. Repeated snapshots of an
	// older job must not associate that image with a newly opened document.
	if j == nil || j.ID == "" || j.ID != a.authorJobID || j.State != "completed" || j.Kind != "author" {
		return
	}
	if j.Format != "vcd" {
		a.burnSource = nil
		return
	}
	if !a.active || !reflect.DeepEqual(a.project, a.authorProject) || !strings.EqualFold(filepath.Ext(j.Output), ".cue") {
		return
	}
	source := *j
	source.Log = append([]string(nil), j.Log...)
	a.burnSource = &source
	a.burnProject = cloneProject(a.authorProject)
}

func (a *app) burnReady() bool {
	j := a.burnSource
	return a.active && j != nil && j.ID != "" && j.State == "completed" && j.Format == "vcd" && j.Kind == "author" && strings.EqualFold(filepath.Ext(j.Output), ".cue") && reflect.DeepEqual(a.project, a.burnProject)
}

func (a *app) burn() {
	if a.smokeMode || a.locked() || !a.burnReady() {
		return
	}
	j := *a.burnSource
	d, _ := gtk.DialogNew()
	d.SetTitle(a.t("Burn Video CD Image", "Handoro sary Video CD"))
	d.SetTransientFor(a.window)
	d.SetModal(true)
	d.AddButton(a.t("Cancel", "Ajanony"), gtk.RESPONSE_CANCEL)
	d.AddButton(a.t("Review and Burn...", "Hamarino ary dory..."), gtk.RESPONSE_ACCEPT)
	d.SetDefaultResponse(gtk.RESPONSE_CANCEL)
	content, _ := d.GetContentArea()
	content.SetBorderWidth(18)
	content.SetSpacing(12)
	text := a.label("Only a completed Video CD image made by this session can be burned. Use a blank CD-R or CD-RW, never a DVD.", "Sary Video CD vita tamin'ity fotoana ity ihany no azo dorana. CD-R na CD-RW foana no ampiasao, fa tsy DVD.")
	text.SetLineWrap(true)
	text.SetMaxWidthChars(65)
	content.PackStart(text, false, false, 0)
	content.PackStart(a.label("Last created VCD image for this document:", "Sary VCD noforonina farany ho an'ity tetikasa ity:"), false, false, 0)
	path := plainLabel(j.Output)
	path.SetLineWrap(true)
	path.SetMaxWidthChars(65)
	content.PackStart(path, false, false, 0)
	// Copy the current recorder model; no drive operations occur in the dialog.
	recorders := recorderChoices(a.recorder)
	content.PackStart(a.label("Recorder", "Mpanoratra kapila"), false, false, 0)
	content.PackStart(recorders, false, false, 0)
	speed := speedCombo()
	speed.SetActiveID(a.speed.GetActiveID())
	content.PackStart(a.label("Write speed (Nx)", "Hafainganam-panoratana (Nx)"), false, false, 0)
	content.PackStart(speed, false, false, 0)
	dummy, _ := gtk.CheckButtonNewWithLabel(a.t("Dummy mode (drive simulation, if supported)", "Fomba fitsapana (raha zakan'ny mpanoratra kapila)"))
	content.PackStart(dummy, false, false, 0)
	r := a.runDialog(d)
	device := recorders.GetActiveID()
	selectedSpeed := speed.GetActiveID()
	simulate := dummy.GetActive()
	a.destroyDialog(d)
	if r != gtk.RESPONSE_ACCEPT {
		return
	}
	request, err := makeBurnRequest(j.ID, device, selectedSpeed, simulate)
	if err != nil {
		a.error(err)
		return
	}
	if !a.confirm(fmt.Sprintf(a.t("Burn this image to %s at %dx speed?\nDummy mode: %t\n\nCD-R/RW ONLY. Writing changes the disc permanently. Cancelling can spoil the disc. Continue?", "Doraina amin'ny %s amin'ny hafainganam-pandeha %dx?\nFitsapana: %t\n\nCD-R/RW IHANY. Manova tanteraka ny kapila ny fanoratana. Mety hanimba kapila ny fanafoanana. Hanohy?"), device, request.Speed, simulate)) {
		return
	}
	request.Confirmed = true
	a.jobRevision++
	a.pending = true
	a.startError = ""
	a.setLocked(false)
	a.showJobText()
	ctx, cancel := context.WithCancel(a.context())
	a.startCancel = cancel
	a.work(func() {
		job, err := a.engine.Burn(ctx, request)
		a.idle(func() {
			a.pending = false
			if err != nil {
				cancel()
				a.startError = err.Error()
				a.setLocked(false)
			} else {
				a.lastJob = job
				a.setLocked(job.State == "running")
			}
			a.showJobText()
		})
	})
}

const studioCSS = `
.disc-studio { background-color: #e7e9ed; color: #202937; }
.disc-studio .studio-menubar { background: #202b3b; color: #f1f3f7; padding: 3px 8px; }
.studio-menubar > menuitem { color: #f1f3f7; padding: 5px 12px; }
.studio-menubar > menuitem:hover { background: #3b4b61; }
.disc-studio .studio-toolbar { background: #f5f6f8; border-bottom: 1px solid #c2c9d2; }
.disc-studio .studio-tool { padding: 7px 9px; }
.disc-studio .primary { background: #cb651e; color: #ffffff; border-color: #ac5015; text-shadow: none; }
.disc-studio .primary:hover { background: #dd762a; }
.disc-studio .primary:disabled { background: #d5c4b8; color: #6f625a; border-color: #c4b7ad; }
.disc-studio .welcome { background: #263347; color: #eef2f8; }
.disc-studio .eyebrow { color: #f3a15b; font-size: 12px; font-weight: bold; letter-spacing: 3px; }
.disc-studio .welcome-title { font-size: 32px; font-weight: 300; }
.disc-studio .project-tile { background: #324259; border: 1px solid #52617a; border-radius: 6px; color: #edf2fa; text-shadow: none; box-shadow: none; }
.disc-studio .project-tile:hover { background: #3c4e68; border-color: #ef9a53; }
.disc-studio .project-tile image { color: #f3a15b; }
.disc-studio .tile-title { font-size: 26px; font-weight: bold; }
.disc-studio .welcome-note { color: #c5d0df; font-size: 12px; }
.disc-studio entry, .disc-studio textview text, .disc-studio list { background-color: #fcfcfd; color: #202937; }
.disc-studio row { border-bottom: 1px solid #e6e9ee; }
.disc-studio row:selected { background: #d3e0ef; color: #152c49; }
.disc-studio .table-heading { background: #d8dee7; padding: 7px 0; font-weight: bold; }
.disc-studio .disc-capacity trough { min-height: 12px; background: #cdd3dc; border: none; }
.disc-studio .disc-capacity progress { min-height: 12px; background: #4d9569; border: none; }
.disc-studio .disc-capacity.near-full progress { background: #d89a32; }
.disc-studio .disc-capacity.over-full progress { background: #c34f4f; }
`
