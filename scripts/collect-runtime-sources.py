#!/usr/bin/env python3
"""Collect exact bundled runtime sources, never bottles or homepage-only offers.

Outputs are independent of the utility collector's files in SOURCE-ARCHIVES.
An unavailable/mismatched source is a release error, not a partial success.
"""

import argparse
from contextlib import contextmanager
import csv
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import zipfile


class CollectionError(RuntimeError):
    pass


def run(*args, env=None):
    return subprocess.check_output(args, text=True, encoding="utf-8", env=env)


def component(value):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9@+_.:-]*", value):
        raise CollectionError(f"Unsafe package/version component: {value!r}")
    return value


def digest(path):
    result = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            result.update(block)
    return result.hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def finish_inventory(output, records):
    write_json(output / "SOURCE-MANIFEST.json", {"schema": 1, "packages": records})
    # SHA256SUMS covers every published file except itself, including provenance.
    files = sorted(p for p in output.rglob("*") if p.is_file())
    (output / "SHA256SUMS").write_text("".join(
        f"{digest(path)}  {path.relative_to(output).as_posix()}\n" for path in files
    ), encoding="utf-8")


def source_notices(archive, destination):
    """Read root notices only. Never extract archive paths or follow links."""
    pattern = re.compile(r"^(copying|licen[cs]e|notice|copyright)([._-].*)?$", re.I)

    def save(name, size, opener):
        parts = PurePosixPath(name).parts
        if (not 1 <= len(parts) <= 2 or ".." in parts or name.startswith("/")
                or not pattern.fullmatch(parts[-1]) or size > 2 * 1024 * 1024):
            return
        with opener() as stream:
            data = stream.read(2 * 1024 * 1024 + 1)
        if len(data) > 2 * 1024 * 1024:
            return
        destination.mkdir(parents=True, exist_ok=True)
        # Archive-controlled names never become output paths.
        target = destination / (hashlib.sha256(name.encode()).hexdigest()[:16] + ".txt")
        target.write_bytes(data)

    try:
        if zipfile.is_zipfile(archive):
            with zipfile.ZipFile(archive) as source:
                for item in source.infolist():
                    if not item.is_dir() and (item.external_attr >> 16) & 0o170000 != 0o120000:
                        save(item.filename, item.file_size, lambda: source.open(item))
        else:
            with tarfile.open(archive, "r|*") as source:
                for item in source:
                    if item.isfile():
                        save(item.name, item.size, lambda: source.extractfile(item))
    except (tarfile.TarError, zipfile.BadZipFile, EOFError):
        # Notices are supplemental; the complete source archive is always retained.
        pass


def brew_version(info):
    stable = info.get("versions", {}).get("stable")
    if not stable:
        raise CollectionError(f"No stable source version for {info.get('name')}")
    revision = int(info.get("revision", 0))
    return stable + (f"_{revision}" if revision else "")


def normalized_brew_version(version):
    return re.sub(r"_0$", "", version)


@contextmanager
def installed_brew_tap(stage, inventory, env):
    repository = Path(run("brew", "--repository", env=env).strip())
    parent = repository / "Library/Taps/kapila-maker"
    parent.mkdir(parents=True, exist_ok=True)
    # Tap.installed? checks for a directory, not a Git repository. No tap-new
    # templates, Git identity/configuration, network tap or history are needed.
    with tempfile.TemporaryDirectory(prefix="homebrew-runtime-sources-", dir=parent) as temporary:
        tap = Path(temporary)
        (tap / "Formula").mkdir()
        selected = []
        for item in inventory:
            name, version = item["formula"], component(item["version"])
            snapshots = stage / "THIRD-PARTY-LICENSES" / name / version / "formula"
            recipes = sorted(snapshots.rglob("*.rb"))
            if (len(recipes) != 1 or recipes[0].name != name + ".rb"
                    or any(p.is_symlink() for p in (recipes[0], *recipes[0].parents)
                           if p.is_relative_to(stage))):
                raise CollectionError(f"Missing/unsafe or ambiguous installed formula snapshot for {name} {version}")
            shutil.copyfile(recipes[0], tap / "Formula" / (name + ".rb"))
            full_name = f"kapila-maker/{tap.name.removeprefix('homebrew-')}/{name}"
            selected.append((item, full_name, recipes, snapshots))
        yield selected


def collect_macos(stage, output, scratch):
    inventory = json.loads((stage / "MACOS-RUNTIME-MANIFEST.json").read_text())
    if not isinstance(inventory, list) or not inventory:
        raise CollectionError("Empty or invalid macOS runtime inventory")
    names = [component(item["formula"]) for item in inventory]
    if len(names) != len(set(names)):
        raise CollectionError("Duplicate macOS runtime formula")
    env = dict(os.environ, HOMEBREW_NO_AUTO_UPDATE="1", HOMEBREW_NO_ANALYTICS="1",
               HOMEBREW_CACHE=str(scratch / "brew-info"))
    with installed_brew_tap(stage, inventory, env) as selected:
        collect_brew_sources(output, scratch, env, selected)


def collect_brew_sources(output, scratch, env, selected):
    metadata = json.loads(run("brew", "info", "--json=v2",
                              *(full_name for _, full_name, _, _ in selected), env=env))
    write_json(output / "brew-info.json", metadata)
    records = []
    # Validate the entire closure before starting potentially large downloads.
    validated = []
    for item, full_name, recipes, snapshots in selected:
        name, version = item["formula"], component(item["version"])
        matches = [info for info in metadata["formulae"]
                   if info.get("full_name") == full_name and info["name"] == name]
        if len(matches) != 1:
            raise CollectionError(f"No unique Homebrew source metadata for {name}")
        info = matches[0]
        if normalized_brew_version(version) != normalized_brew_version(brew_version(info)):
            raise CollectionError(f"Homebrew source version mismatch: {name} installed {version}, "
                                  f"available {brew_version(info)}")
        # This binds URL/checksum/resource metadata to the exact installed Ruby
        # recipe, even when the latest recipe has the same version/revision.
        stable = info.get("urls", {}).get("stable", {})
        if (info.get("ruby_source_checksum", {}).get("sha256") != digest(recipes[0])
                or not stable.get("url") or not stable.get("checksum")):
            raise CollectionError(f"Homebrew source snapshot metadata mismatch: {name} {version}")
        validated.append((item, full_name, info, recipes, snapshots))
    for item, full_name, info, recipes, snapshots in validated:
        name, version = item["formula"], item["version"]
        formula_dir = output / "formulae" / name / version
        formula_dir.mkdir(parents=True)
        for recipe in recipes:
            target = formula_dir / recipe.relative_to(snapshots)
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(recipe, target)
        write_json(formula_dir / "installed.json", item)
        write_json(formula_dir / "source-info.json", info)
        # A separate fresh cache per formula also makes shared-resource origins explicit.
        cache = scratch / name
        cache.mkdir()
        env["HOMEBREW_CACHE"] = str(cache)
        print(f"Fetching Homebrew sources: {name} {version}", flush=True)
        run("brew", "fetch", "--build-from-source", "--retry", full_name, env=env)
        downloads = cache / "downloads"
        archives = []
        for path in sorted(downloads.rglob("*")):
            if (not path.is_file() or path.is_symlink()
                    or any(p in ("api", "locks", "trash") for p in path.relative_to(downloads).parts)
                    or path.name.endswith((".jws.json", ".bottle_manifest.json", ".lock", ".incomplete"))):
                continue
            if ".bottle." in path.name:
                raise CollectionError(f"Unexpected bottle in source cache: {path.name}")
            target = output / "archives" / name / path.name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
            archives.append({"path": target.relative_to(output).as_posix(),
                             "sha256": digest(target)})
            source_notices(target, output / "notices" / name / target.name)
        if not archives:
            raise CollectionError(f"Homebrew fetched no source files for {name}")
        records.append({"formula": name, "version": version, "archives": archives,
                        "origin": info.get("urls", {}), "resources": info.get("resources", []),
                        "source_metadata": (formula_dir / "source-info.json").relative_to(output).as_posix(),
                        "installed_formula_snapshots": [
                            (formula_dir / p.relative_to(snapshots)).relative_to(output).as_posix()
                            for p in recipes]})
    finish_inventory(output, records)


def windows_packages(stage):
    packages = {}
    for line in (stage / "WINDOWS-RUNTIME-PACKAGES.txt").read_text().splitlines():
        name, version = line.split()
        component(name)
        component(version)
        if name in packages and packages[name] != version:
            raise CollectionError(f"Conflicting installed versions for {name}")
        packages[name] = version
    actual = set()
    with (stage / "WINDOWS-RUNTIME-FILES.tsv").open(newline="") as stream:
        reader = csv.DictReader(stream, delimiter="\t")
        for row in reader:
            name, version = row["Package"], row["Installed version"]
            if packages.get(name) != version:
                raise CollectionError(f"Runtime file/package inventory mismatch for {name}")
            path = row["Staged path"].replace("\\", "/")
            parts = PurePosixPath(path).parts
            if not parts or ".." in parts or path.startswith("/"):
                raise CollectionError(f"Unsafe staged runtime path: {path}")
            if parts[0].upper() != "THIRD-PARTY-LICENSES":
                actual.add(name)
    if not actual:
        raise CollectionError("Windows inventory contains no runtime files")
    return {name: packages[name] for name in sorted(actual)}


def installed_pkgbase(name, version):
    db = run("pacman-conf", "DBPath").strip()
    if sys.platform == "win32":
        db = run("cygpath", "-w", db).strip()
    desc = (Path(db) / "local" / f"{name}-{version}" / "desc").read_text(encoding="utf-8")
    fields = {}
    for section in desc.split("\n\n"):
        lines = section.strip().splitlines()
        if lines:
            fields[lines[0]] = lines[1:]
    if fields.get("%NAME%") != [name] or fields.get("%VERSION%") != [version]:
        raise CollectionError(f"Installed pacman metadata mismatch for {name} {version}")
    base = fields.get("%BASE%")
    if not base or len(base) != 1:
        raise CollectionError(f"No installed pacman pkgbase for {name}")
    return component(base[0])


def download(url, target):
    # HTTPS authenticates transport; SHA256SUMS records the acquired bytes, not a
    # claim of an independently verified upstream digest or PGP signature.
    for attempt in range(3):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "runtime-source-collector/1"})
            with urllib.request.urlopen(request, timeout=120) as response, target.open("wb") as stream:
                shutil.copyfileobj(response, stream)
            return
        except urllib.error.HTTPError as error:
            if error.code == 404 or attempt == 2:
                raise
        except (OSError, urllib.error.URLError):
            if attempt == 2:
                raise


def srcinfo_fields(text):
    fields = {}
    for line in text.splitlines():
        if " = " in line and not line.lstrip().startswith("#"):
            key, value = line.strip().split(" = ", 1)
            fields.setdefault(key, []).append(value)
    return fields


def validate_source_package(archive, base, version, metadata_dir):
    # tar reads individual members to stdout only. No archive path is extracted.
    # GNU/MSYS2 tar must not treat a native Windows drive letter as a remote host.
    members = run("tar", "--force-local", "-tf", str(archive)).splitlines()
    if any(PurePosixPath(p).is_absolute() or ".." in PurePosixPath(p).parts for p in members):
        raise CollectionError(f"Unsafe source package paths: {archive.name}")
    prefix = base + "/"
    for name in (".SRCINFO", "PKGBUILD"):
        if members.count(prefix + name) != 1:
            raise CollectionError(f"Source package lacks unique {prefix + name}")
    text = run("tar", "--force-local", "-xOf", str(archive), prefix + ".SRCINFO")
    fields = srcinfo_fields(text)
    try:
        available = fields["pkgver"][0] + "-" + fields["pkgrel"][0]
        epoch = fields.get("epoch", ["0"])[0]
        if epoch != "0":
            available = epoch + ":" + available
        if fields["pkgbase"] != [base] or available != version:
            raise CollectionError(f"MSYS2 source version mismatch: {base} installed {version}, "
                                  f"archive {fields.get('pkgbase')} {available}")
    except (KeyError, IndexError) as error:
        raise CollectionError(f"Incomplete .SRCINFO in {archive.name}") from error
    sources = [value for key, values in fields.items()
               if key == "source" or key.startswith("source_") for value in values]
    if not sources:
        raise CollectionError(f"No sources declared in {archive.name}")
    for source in sources:
        if "::" in source:
            filename = source.split("::", 1)[0]
        else:
            filename = PurePosixPath(urllib.parse.urlsplit(source).path).name
        # VCS allsource packages contain a directory, rather than a tarball.
        candidates = {prefix + filename, prefix + filename + "/"}
        if not filename or not candidates.intersection(members):
            raise CollectionError(f"Source package missing declared source {source}")
    metadata_dir.mkdir(parents=True)
    (metadata_dir / ".SRCINFO").write_text(text, encoding="utf-8")
    (metadata_dir / "PKGBUILD").write_text(
        run("tar", "--force-local", "-xOf", str(archive), prefix + "PKGBUILD"), encoding="utf-8")
    return fields


def collect_windows(stage, output, scratch):
    packages = windows_packages(stage)
    groups = {}
    for name, version in packages.items():
        base = installed_pkgbase(name, version)
        groups.setdefault((base, version), []).append(name)
    records = []
    for (base, version), names in sorted(groups.items()):
        # Epoch belongs in .SRCINFO validation but not the repository filename.
        filename_version = version.split(":", 1)[-1]
        archive = None
        for extension in ("zst", "xz", "gz"):
            filename = f"{base}-{filename_version}.src.tar.{extension}"
            url = "https://repo.msys2.org/mingw/sources/" + urllib.parse.quote(filename)
            candidate = scratch / filename
            print(f"Fetching MSYS2 sources: {base} {version}", flush=True)
            try:
                download(url, candidate)
                archive = candidate
                break
            except urllib.error.HTTPError as error:
                if error.code != 404:
                    raise
        if archive is None:
            raise CollectionError(f"Exact MSYS2 all-source package unavailable: {base} {version}; "
                                  "release blocked (no current-recipe substitution)")
        metadata_dir = output / "recipes" / f"{base}-{filename_version}"
        fields = validate_source_package(archive, base, version, metadata_dir)
        target = output / "archives" / archive.name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(archive, target)
        records.append({"packages": names, "pkgbase": base, "version": version,
                        "origin": url, "archive": target.relative_to(output).as_posix(),
                        "sha256": digest(target), "source_metadata": fields,
                        "source_recipe": (metadata_dir / "PKGBUILD").relative_to(output).as_posix(),
                        "verification": "HTTPS; exact .SRCINFO base/version and source presence; "
                                        "local SHA256 (not independent signature verification)"})
    for name in ("WINDOWS-RUNTIME-PACKAGES.txt", "WINDOWS-RUNTIME-FILES.tsv"):
        shutil.copyfile(stage / name, output / name)
    finish_inventory(output, records)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("stage", type=Path)
    args = parser.parse_args(argv)
    stage = args.stage.resolve(strict=True)
    platforms = [name for name, manifest in (
        ("macos", "MACOS-RUNTIME-MANIFEST.json"),
        ("windows", "WINDOWS-RUNTIME-PACKAGES.txt")) if (stage / manifest).is_file()]
    if not platforms:
        if sys.platform.startswith("linux"):
            # Linux packages do not bundle GTK; utility sources remain untouched.
            return 0
        raise CollectionError("Missing native runtime inventory; run the bundler first")
    if len(platforms) != 1:
        raise CollectionError("Ambiguous stage contains both native runtime inventories")
    platform = platforms[0]
    root = stage / "SOURCE-ARCHIVES"
    root.mkdir(exist_ok=True)
    target = root / (platform + "-runtime")
    if target.exists():
        raise CollectionError(f"Runtime source output already exists: {target}; use a fresh stage")
    temp_root = os.environ.get("RUNNER_TEMP") or os.environ.get("TMPDIR") or os.environ.get("TMP")
    with tempfile.TemporaryDirectory(prefix="runtime-source-cache-", dir=temp_root) as cache:
        with tempfile.TemporaryDirectory(prefix=".runtime-sources-", dir=root) as pending:
            output = Path(pending) / "output"
            output.mkdir()
            collector = collect_macos if platform == "macos" else collect_windows
            collector(stage, output, Path(cache))
            output.rename(target)
    print(f"Runtime sources and SHA256SUMS: {target}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (CollectionError, OSError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        print(f"Runtime source collection failed: {error}", file=sys.stderr)
        sys.exit(1)
