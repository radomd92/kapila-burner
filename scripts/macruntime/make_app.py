#!/usr/bin/env python3
"""Convert an already bundled flat STAGE into the standard signed macOS app.

The flat runtime must contain only system and @loader_path dependencies.
Snapshot and validate the entire graph before moving or modifying any image.
This is a build-time tool; the finished app needs neither Python nor Homebrew.
"""

import argparse
import os
from pathlib import Path
import plistlib
import shutil
import sys

from bundle import SYSTEM, images, inside, metadata, run


APP_NAME = "Kapila Burner.app"
EXECUTABLES = ("kapila-burner", "kapila-burner-web")
HELPERS = ("ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager",
           "gdk-pixbuf-query-loaders", "gtk-query-immodules-3.0")


def destination(relative):
    """Map a flat-stage path to its new path relative to Contents."""
    parts = relative.parts
    if relative.as_posix() in EXECUTABLES:
        return Path("MacOS") / relative
    if parts[:2] == ("tools", "bin"):
        return Path("Helpers").joinpath(*parts[2:])
    if parts[0] == "lib":
        return Path("Frameworks").joinpath(*parts[1:])
    if parts[0] in ("share", "etc", "tools", "THIRD-PARTY-LICENSES"):
        return Path("Resources") / relative
    if len(parts) == 1 and (relative.suffix.lower() in (".txt", ".json", ".tsv")
                            or relative.name.startswith(("README", "THIRD-PARTY", "BUILD-"))):
        return Path("Resources") / relative
    raise RuntimeError(f"Unrecognized flat-stage entry: {relative}")


def snapshot(stage, contents):
    """Return the complete move plan and load-command edits, without mutating."""
    paths = sorted(stage.rglob("*"))
    plan = {}
    for source in paths:
        if source == stage / "launch-gtk.command":
            if source.is_symlink() or not source.is_file():
                raise RuntimeError(f"Expected a regular launcher file: {source}")
            continue
        if source.is_symlink():
            raise RuntimeError(f"Flat stage must not contain symlinks: {source}")
        plan[source] = contents / destination(source.relative_to(stage))

    binaries = images(stage)
    binary_set = set(binaries)
    for relative in (*EXECUTABLES, *("tools/bin/" + name for name in HELPERS)):
        if stage / relative not in binary_set:
            raise RuntimeError(f"Required native Mach-O executable missing: {relative}")
    if not (stage / "README.txt").is_file():
        raise RuntimeError("Missing staged README.txt")

    # All otool reads happen here, before any source file is moved or patched.
    edits = {}
    for source in binaries:
        target = plan[source]
        location = target.relative_to(contents).parts[0]
        if location == "Resources":
            raise RuntimeError(f"Mach-O code cannot be installed in Resources: {source}")
        deps, rpaths, identity = metadata(source)
        changes = []
        for dep in deps:
            if dep.startswith(SYSTEM):
                continue
            if not dep.startswith("@loader_path/"):
                raise RuntimeError(f"Unbundled dependency in {source}: {dep}")
            old_dependency = (source.parent / dep[len("@loader_path/"):]).resolve()
            if not inside(old_dependency, stage) or old_dependency not in binary_set:
                raise RuntimeError(f"Missing or escaping dependency in {source}: {dep}")
            new_dependency = plan[old_dependency]
            new_name = "@loader_path/" + os.path.relpath(new_dependency, target.parent)
            if dep != new_name:
                changes.extend(("-change", dep, new_name))
        for rpath in dict.fromkeys(rpaths):
            changes.extend(("-delete_rpath", rpath))
        if identity:
            changes.extend(("-id", "@rpath/" + target.name))
        edits[target] = changes

    # Executable directories contain code only, not auxiliary tool data.
    for source, target in plan.items():
        if source.is_file() and target.relative_to(contents).parts[0] in ("MacOS", "Helpers"):
            if source not in binary_set:
                raise RuntimeError(f"Non-Mach-O file in executable directory: {source}")
    return plan, edits


def make_app(stage, plist):
    stage = stage.resolve(strict=True)
    app = stage / APP_NAME
    contents = app / "Contents"
    if app.exists() or app.is_symlink():
        raise RuntimeError(f"Refusing to overwrite existing app: {app}")
    if not stage.is_dir():
        raise RuntimeError(f"STAGE must be a directory: {stage}")
    with plist.open("rb") as stream:
        info = plistlib.load(stream)
    if info.get("CFBundleExecutable") != EXECUTABLES[0] or info.get("CFBundlePackageType") != "APPL":
        raise RuntimeError("Info.plist must declare the native kapila-burner APPL executable")
    plan, edits = snapshot(stage, contents)

    for directory in ("MacOS", "Helpers", "Frameworks", "Resources"):
        (contents / directory).mkdir(parents=True, exist_ok=True)
    for source, target in plan.items():
        if source.is_dir():
            target.mkdir(parents=True, exist_ok=True)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            if source == stage / "README.txt":
                shutil.copy2(source, target)
            else:
                source.rename(target)
    for source in sorted(plan, key=lambda p: len(p.parts), reverse=True):
        if source.is_dir():
            source.rmdir()
    shutil.copy2(plist, contents / "Info.plist")

    # Every image was signed by the flat bundler. Remove those signatures before
    # changing load commands, then sign all nested code before sealing the app.
    for binary in edits:
        binary.chmod(binary.stat().st_mode | 0o200)
        run("codesign", "--remove-signature", binary)
    for binary, changes in edits.items():
        if changes:
            run("install_name_tool", *changes, binary)
    for binary in sorted(edits, key=lambda p: (-len(p.parts), str(p))):
        run("codesign", "--force", "--sign", "-", binary)
    run("codesign", "--force", "--sign", "-", app)
    run("codesign", "--verify", "--deep", "--strict", app)

    for name in EXECUTABLES:
        (stage / name).symlink_to(Path(APP_NAME) / "Contents/MacOS" / name)
    launcher = stage / "launch-gtk.command"
    launcher.write_text('''#!/bin/bash
set -euo pipefail
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$directory/Kapila Burner.app/Contents/MacOS/kapila-burner" "$@"
''')
    launcher.chmod(0o755)
    return app


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("stage", type=Path)
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error("App creation requires macOS and Xcode command-line tools")
    plist = Path(__file__).resolve().parents[1] / "macos-Info.plist"
    print(make_app(args.stage, plist))


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, plistlib.InvalidFileException) as error:
        sys.exit(f"macOS app: {error}")
