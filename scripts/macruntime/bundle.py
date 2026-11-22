#!/usr/bin/env python3
"""Build-time-only relocatable Intel macOS runtime builder; no Python at runtime.

Usage: bundle-macos.sh STAGE
       bundle-macos.sh --verify-only STAGE

Verification checks all Mach-O files, not just the main GUI. It does not launch
GTK or utilities and therefore does not require a display or a physical drive.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys


SYSTEM = ("/usr/lib/", "/System/Library/")
MAGIC = {b"\xfe\xed\xfa\xce", b"\xce\xfa\xed\xfe", b"\xfe\xed\xfa\xcf",
         b"\xcf\xfa\xed\xfe", b"\xca\xfe\xba\xbe", b"\xbe\xba\xfe\xca",
         b"\xca\xfe\xba\xbf", b"\xbf\xba\xfe\xca"}
LOADS = {"LC_LOAD_DYLIB", "LC_LOAD_WEAK_DYLIB", "LC_REEXPORT_DYLIB",
         "LC_LOAD_UPWARD_DYLIB", "LC_LAZY_LOAD_DYLIB"}


def run(*args):
    result = subprocess.run([str(a) for a in args], text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise RuntimeError(f"{' '.join(map(str, args))}:\n{result.stderr}")
    return result.stdout


def macho(path):
    if not path.is_file():
        return False
    with path.open("rb") as stream:
        return stream.read(4) in MAGIC


def images(stage):
    for path in stage.rglob("*"):
        if path.is_symlink() and (not path.exists() or not inside(path, stage)):
            raise RuntimeError(f"Dangling or external staged symlink: {path}")
    return sorted(p for p in stage.rglob("*") if not p.is_symlink() and macho(p))


def metadata(path):
    # -arch prevents duplicate load commands for universal binaries.
    run("lipo", "-verify_arch", "x86_64", path)
    text = run("otool", "-arch", "x86_64", "-l", path)
    deps, rpaths, identity = [], [], None
    command = None
    for line in text.splitlines():
        match = re.match(r"\s*cmd (LC_\w+)\s*$", line)
        if match:
            command = match[1]
        match = re.match(r"\s*(?:name|path) (.*) \(offset \d+\)\s*$", line)
        if not match:
            continue
        if command in LOADS:
            deps.append(match[1])
        elif command == "LC_RPATH":
            rpaths.append(match[1])
        elif command == "LC_ID_DYLIB":
            identity = match[1]
    return deps, rpaths, identity


def inside(path, directory):
    return path.resolve().is_relative_to(directory.resolve())


def verify(stage):
    binaries = images(stage)
    if not binaries:
        raise RuntimeError("No Mach-O images found")
    for binary in binaries:
        deps, rpaths, _ = metadata(binary)
        if rpaths:
            raise RuntimeError(f"Unexpected runtime search paths in {binary}: {rpaths}")
        for dep in deps:
            if dep.startswith(SYSTEM):
                continue
            if not dep.startswith("@loader_path/"):
                raise RuntimeError(f"External/unresolved dependency in {binary}: {dep}")
            target = binary.parent / dep[len("@loader_path/"):]
            if not inside(target, stage) or not macho(target):
                raise RuntimeError(f"Missing or escaping dependency in {binary}: {dep}")
        run("codesign", "--verify", "--strict", binary)
    print(f"Verified {len(binaries)} Intel Mach-O images; no non-system external dependencies")


class Bundler:
    def __init__(self, stage):
        self.stage = stage
        self.prefix = Path(run("brew", "--prefix").strip())
        self.cellar = Path(run("brew", "--cellar").strip()).resolve()
        self.sources = {}
        self.visited = set()
        self.info = {}
        self.kegs = set()
        self.search = [self.prefix / "lib"]
        self.search.extend(sorted((self.prefix / "opt").glob("*/lib")))

    def formula(self, name):
        return Path(run("brew", "--prefix", name).strip())

    def record(self, source):
        # Record exact installed kegs, including runtime-data-only dependencies.
        source = source.resolve()
        cellar = self.cellar
        if inside(source, cellar):
            parts = source.relative_to(cellar).parts
            if len(parts) >= 2:
                self.kegs.add(cellar / parts[0] / parts[1])

    def copy(self, source, target):
        source = source.resolve(strict=True)
        self.record(source)
        if target in self.sources:
            previous = self.sources[target]
            if previous != source:
                def digest(path):
                    return hashlib.sha256(path.read_bytes()).digest()
                if digest(previous) != digest(source):
                    raise RuntimeError(f"Library name collision at {target}: {previous} vs {source}")
            return
        if target.exists() and source != target.resolve():
            raise RuntimeError(f"Refusing to overwrite untracked staged file: {target}")
        target.parent.mkdir(parents=True, exist_ok=True)
        if source != target.resolve():
            shutil.copy2(source, target)
        target.chmod(target.stat().st_mode | 0o200)
        self.sources[target] = source

    def tree(self, source, target):
        if not source.is_dir():
            raise RuntimeError(f"Required runtime data missing: {source}")
        self.record(source)
        # Dereference Homebrew symlinks: the package must not retain keg links.
        shutil.copytree(source, target, symlinks=False, dirs_exist_ok=True)

    def assets(self):
        gtk = self.formula("gtk+3")
        pixbuf = self.formula("gdk-pixbuf")
        glib = self.formula("glib")
        for location in ("bin/gtk-query-immodules-3.0", "libexec/gtk-query-immodules-3.0"):
            if (gtk / location).is_file():
                self.copy(gtk / location, self.stage / "tools/bin/gtk-query-immodules-3.0")
                break
        self.copy(pixbuf / "bin/gdk-pixbuf-query-loaders",
                  self.stage / "tools/bin/gdk-pixbuf-query-loaders")
        loaders = self.prefix / "lib/gdk-pixbuf-2.0/2.10.0/loaders"
        if not loaders.is_dir():
            loaders = pixbuf / "lib/gdk-pixbuf-2.0/2.10.0/loaders"
        modules = sorted(p for p in loaders.glob("*") if macho(p))
        if not modules:
            raise RuntimeError(f"No GDK Pixbuf loader modules in {loaders}")
        for source in modules:
            self.copy(source, self.stage / "lib/gdk-pixbuf-2.0/2.10.0/loaders" / source.name)
        # Optional GTK input/print modules and GIO backends are dlopen roots too.
        for prefix, directory in ((gtk, "lib/gtk-3.0"), (glib, "lib/gio/modules")):
            root = self.prefix / directory
            if not root.is_dir():
                root = prefix / directory
            if root.is_dir():
                self.tree(root, self.stage / directory)
                for source in root.rglob("*"):
                    if macho(source):
                        self.record(source)
                        self.sources[self.stage / directory / source.relative_to(root)] = source.resolve()
        (self.stage / "lib/gio/modules").mkdir(parents=True, exist_ok=True)
        schemas = self.stage / "share/glib-2.0/schemas"
        self.tree(glib / "share/glib-2.0/schemas", schemas)
        gtk_schemas = gtk / "share/glib-2.0/schemas"
        if gtk_schemas.is_dir():
            self.tree(gtk_schemas, schemas)
        desktop_schemas = self.prefix / "opt/gsettings-desktop-schemas/share/glib-2.0/schemas"
        if desktop_schemas.is_dir():
            self.tree(desktop_schemas, schemas)
        run(glib / "bin/glib-compile-schemas", "--strict", schemas)
        self.tree(gtk / "share/gtk-3.0", self.stage / "share/gtk-3.0")
        for formula, theme in (("adwaita-icon-theme", "Adwaita"), ("hicolor-icon-theme", "hicolor")):
            self.tree(self.formula(formula) / "share/icons" / theme,
                      self.stage / "share/icons" / theme)
        fontconfig = self.formula("fontconfig")
        fonts = fontconfig / "etc/fonts"
        if not fonts.is_dir():
            fonts = self.prefix / "etc/fonts"
        self.record(fontconfig)
        self.tree(fonts, self.stage / "etc/fonts")
        # Replace host-specific includes and cache paths. Native macOS font dirs
        # supply fonts without redistributing Apple's font files.
        (self.stage / "etc/fonts/fonts.conf").write_text('''<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <dir>/System/Library/Fonts</dir>
  <dir>/Library/Fonts</dir>
  <dir>~/Library/Fonts</dir>
  <cachedir prefix="xdg">fontconfig</cachedir>
  <config><rescan><int>30</int></rescan></config>
</fontconfig>
''')
        # Copied conf.d links may have absolute build-machine includes. The
        # portable config above deliberately does not include those fragments.

    def getinfo(self, source):
        if source not in self.info:
            self.info[source] = metadata(source)
        return self.info[source]

    def expand(self, value, source, executable):
        for token, base in (("@loader_path", source.parent),
                            ("@executable_path", executable.parent)):
            if value == token or value.startswith(token + "/"):
                return str(base) + value[len(token):]
        return value

    def resolve(self, dep, source, executable, rpaths):
        candidates = []
        if dep.startswith("@rpath/"):
            suffix = dep[len("@rpath/"):]
            candidates.extend(Path(r) / suffix for r in rpaths if not r.startswith("@"))
            candidates.extend(p / suffix for p in self.search)
        else:
            candidates.append(Path(self.expand(dep, source, executable)))
        for candidate in candidates:
            if candidate.is_absolute() and candidate.is_file() and macho(candidate):
                return candidate.resolve()
        raise RuntimeError(f"Cannot resolve {dep} from {source}; tried {candidates}")

    def walk(self, target, executable, inherited=()):
        if target in self.visited:
            return
        self.visited.add(target)
        source = self.sources[target]
        # Ship only the requested Intel slice, so an unexamined arm64 slice
        # cannot retain external install names in a universal dependency.
        if len(run("lipo", "-archs", target).split()) > 1:
            temporary = target.with_name(target.name + ".intel-tmp")
            run("lipo", target, "-thin", "x86_64", "-output", temporary)
            os.replace(temporary, target)
        deps, own_rpaths, identity = self.getinfo(source)
        rpaths = tuple(self.expand(r, source, executable) for r in own_rpaths) + inherited
        changes = []
        for dep in deps:
            if dep.startswith(SYSTEM):
                continue
            resolved = self.resolve(dep, source, executable, rpaths)
            destination = self.stage / "lib" / resolved.name
            self.copy(resolved, destination)
            self.walk(destination, executable, rpaths)
            relative = os.path.relpath(destination, target.parent)
            changes.extend(("-change", dep, "@loader_path/" + relative))
        # Never depend on DYLD_* or runtime @rpath resolution. Remove stale
        # signatures before editing and re-sign every image after all changes.
        subprocess.run(["codesign", "--remove-signature", str(target)],
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for rpath in dict.fromkeys(own_rpaths):
            changes.extend(("-delete_rpath", rpath))
        if identity:
            changes.extend(("-id", "@rpath/" + target.name))
        if changes:
            try:
                run("install_name_tool", *changes, target)
            except RuntimeError as error:
                raise RuntimeError(f"{error}\nIf load-command space is exhausted, rebuild "
                                   "this image with -Wl,-headerpad_max_install_names.") from error

    def notices(self):
        directory = self.stage / "THIRD-PARTY-LICENSES"
        directory.mkdir(exist_ok=True)
        manifest = []
        for keg in sorted(self.kegs):
            receipt = keg / "INSTALL_RECEIPT.json"
            item = {"formula": keg.parent.name, "version": keg.name}
            if receipt.is_file():
                item["install_receipt"] = json.loads(receipt.read_text())
            # Homebrew's installed formula snapshot includes upstream source
            # URLs and hashes, without relying on current formula revisions.
            formula_dir = keg / ".brew"
            if formula_dir.is_dir():
                self.tree(formula_dir, directory / keg.parent.name / keg.name / "formula")
            notices = [p for p in keg.iterdir() if p.is_file() and
                       re.match(r"(?i)^(copying|licen[cs]e|notice|copyright|authors)", p.name)]
            for source in notices:
                target = directory / keg.parent.name / keg.name / source.name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, target)
            licenses = keg / "share/licenses"
            if licenses.is_dir():
                self.tree(licenses, directory / keg.parent.name / keg.name / "licenses")
            manifest.append(item)
        (self.stage / "MACOS-RUNTIME-MANIFEST.json").write_text(json.dumps(manifest, indent=2) + "\n")
        (self.stage / "THIRD-PARTY.txt").write_text(
            "Runtime formula versions and installed source metadata: MACOS-RUNTIME-MANIFEST.json.\n"
            "Installed notices and formula snapshots (upstream URLs and source hashes): THIRD-PARTY-LICENSES/.\n"
            "This inventory is not a corresponding-source distribution or a complete license audit.\n"
            "Release distributors must also supply required utility/runtime licenses and corresponding\n"
            "source, including build scripts and patches, as required by their respective licenses.\n")

    def build(self):
        required = ["kapila-burner", "kapila-burner-web"]
        required += ["tools/bin/" + name for name in
                     ("ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager")]
        for name in required:
            if not macho(self.stage / name):
                raise RuntimeError(f"Required native Mach-O executable missing: {name}")
        self.assets()
        roots = images(self.stage)
        for target in roots:
            self.sources.setdefault(target, target.resolve())
        # Snapshot metadata before rewriting: staged executables are their own
        # sources, whereas copied libraries retain their original keg source.
        for source in self.sources.values():
            self.getinfo(source)
        gui_source = self.sources[self.stage / "kapila-burner"]
        gui_rpaths = tuple(self.expand(r, gui_source, gui_source)
                           for r in self.getinfo(gui_source)[1])
        for root in roots:
            if root.parent in (self.stage, self.stage / "tools/bin"):
                self.walk(root, self.sources[root])
            else:
                self.walk(root, gui_source, gui_rpaths)
        for binary in images(self.stage):
            run("codesign", "--force", "--sign", "-", binary)
        self.notices()
        launcher = Path(__file__).resolve().parents[1] / "launch-gtk-macos.sh"
        shutil.copy2(launcher, self.stage / "launch-gtk.command")
        (self.stage / "launch-gtk.command").chmod(0o755)
        verify(self.stage)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify-only", action="store_true")
    parser.add_argument("stage", type=Path)
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error("Mach-O bundling/verification requires macOS and Xcode command-line tools")
    stage = args.stage.resolve(strict=True)
    if not stage.is_dir():
        parser.error("STAGE must be an existing directory")
    if args.verify_only:
        verify(stage)
    else:
        Bundler(stage).build()


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError) as error:
        sys.exit(f"macOS runtime: {error}")
