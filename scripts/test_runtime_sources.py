#!/usr/bin/env python3
"""Offline tests: python3 -m unittest discover -s scripts -p test_runtime_sources.py."""

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
import zipfile


spec = importlib.util.spec_from_file_location(
    "runtime_sources", Path(__file__).with_name("collect-runtime-sources.py"))
sources = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sources)


class RuntimeSourcesTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tar = sources.gnu_tar()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.stage = self.root / "stage"
        self.stage.mkdir()
        self.output = self.root / "output"
        self.output.mkdir()
        self.scratch = self.root / "scratch"
        self.scratch.mkdir()
        self.repository = self.root / "brew"

    def mac_stage(self, version="2.0_1", name="glib"):
        recipe = self.stage / "THIRD-PARTY-LICENSES" / name / version / "formula" / (name + ".rb")
        recipe.parent.mkdir(parents=True)
        recipe.write_text('class Glib < Formula\n  url "https://example.test/glib.tar.xz"\nend\n')
        (self.stage / "MACOS-RUNTIME-MANIFEST.json").write_text(json.dumps([
            {"formula": name, "version": version, "install_receipt": {"built_as_bottle": True}}
        ]))

    def mac_info(self, version="2.0", revision=1):
        return {"formulae": [{"name": "glib", "full_name": "glib", "revision": revision,
                              "versions": {"stable": version},
                               "urls": {"stable": {"url": "https://example.test/glib.tar.xz",
                                                   "checksum": "upstream-checksum"}},
                              "ruby_source_checksum": {"sha256": sources.digest(next(self.stage.rglob("*.rb")))},
                               "resources": [{"name": "patch", "url": "https://example.test/fix.patch"}]}]}

    def brew_metadata(self, args, info=None):
        if args == ("brew", "--repository"):
            return str(self.repository)
        self.assertEqual(args[:3], ("brew", "info", "--json=v2"))
        self.assertEqual(len(args), 4)
        user, tap, name = args[3].split("/")
        self.assertEqual(user, "kapila-maker")
        self.assertTrue(tap.startswith("runtime-sources-"))
        recipe = self.repository / "Library/Taps" / user / ("homebrew-" + tap) / "Formula" / (name + ".rb")
        self.assertEqual(recipe.read_bytes(), next(self.stage.rglob("*.rb")).read_bytes())
        self.assertFalse((recipe.parent.parent / ".git").exists())
        info = info if info is not None else self.mac_info()
        info["formulae"][0]["full_name"] = args[3]
        return json.dumps(info)

    def windows_stage(self):
        (self.stage / "WINDOWS-RUNTIME-PACKAGES.txt").write_text(
            "mingw-w64-x86_64-glib2 2.86.3-1\nmingw-w64-x86_64-gcc 15.2.0-1\n")
        (self.stage / "WINDOWS-RUNTIME-FILES.tsv").write_text(
            "SHA256\tPackage\tInstalled version\tMSYS2 source\tStaged path\tOwnership path\n"
            "abc\tmingw-w64-x86_64-glib2\t2.86.3-1\t/mingw64/bin/glib.dll\tglib.dll\t/a\n"
            "def\tmingw-w64-x86_64-gcc\t15.2.0-1\t/a\tTHIRD-PARTY-LICENSES/gcc/COPYING\t/a\n")

    def make_source_package(self, version="2.86.3", base="mingw-w64-glib2", omit_source=False,
                            srcinfo=None):
        archive = self.root / "package.src.tar.gz"
        text = (f"pkgbase = {base}\n\tpkgver = {version}\n\tpkgrel = 1\n"
                "\tsource = https://example.test/glib.tar.xz\n"
                "\tsource = fix.patch\n\tsha256sums = abc\n")
        entries = {f"{base}/.SRCINFO": (text if srcinfo is None else srcinfo).encode(),
                   f"{base}/PKGBUILD": b"# exact recipe\n",
                   f"{base}/fix.patch": b"patch", f"{base}/glib.tar.xz": b"upstream source"}
        if omit_source:
            del entries[f"{base}/glib.tar.xz"]
        with tarfile.open(archive, "w:gz") as target:
            for name, data in entries.items():
                item = tarfile.TarInfo(name)
                item.size = len(data)
                target.addfile(item, io.BytesIO(data))
        return archive

    def assert_hashes(self, output):
        hashed = set()
        for line in (output / "SHA256SUMS").read_text().splitlines():
            expected, relative = line.split("  ", 1)
            path = output / relative
            self.assertEqual(expected, hashlib.sha256(path.read_bytes()).hexdigest())
            hashed.add(relative)
        self.assertEqual(hashed, {p.relative_to(output).as_posix() for p in output.rglob("*")
                                 if p.is_file() and p.name != "SHA256SUMS"})

    def test_macos_fetches_only_sources_and_preserves_recipe_resources_hashes(self):
        self.mac_stage()
        commands = []

        def brew(*args, env=None):
            commands.append(args)
            self.assertTrue(Path(env["HOMEBREW_CACHE"]).is_relative_to(self.scratch))
            if args[1] != "fetch":
                return self.brew_metadata(args)
            self.assertEqual(args, ("brew", "fetch", "--build-from-source", "--retry", commands[-2][-1]))
            downloads = Path(env["HOMEBREW_CACHE"]) / "downloads"
            downloads.mkdir()
            for name, data in (("hash--glib.tar.xz", b"source"), ("hash--fix.patch", b"patch"),
                               ("hash--source-resource.json", b"source resource"),
                               ("api.jws.json", b"api"), ("lock.lock", b"lock")):
                (downloads / name).write_bytes(data)
            (downloads / "symlink").symlink_to(downloads / "hash--glib.tar.xz")
            return ""

        with patch.object(sources, "run", side_effect=brew):
            sources.collect_macos(self.stage, self.output, self.scratch)
        self.assertEqual(len(commands), 3)
        self.assertEqual(list((self.repository / "Library/Taps/kapila-maker").iterdir()), [])
        result = json.loads((self.output / "SOURCE-MANIFEST.json").read_text())["packages"][0]
        self.assertEqual(len(result["archives"]), 3)
        self.assertEqual(result["resources"], self.mac_info()["formulae"][0]["resources"])
        self.assertTrue((self.output / result["installed_formula_snapshots"][0]).is_file())
        self.assert_hashes(self.output)

    def test_macos_rejects_source_version_or_revision_mismatch_before_fetch(self):
        self.mac_stage()
        for info in (self.mac_info("2.1"), self.mac_info(revision=2)):
            with self.subTest(info=info), patch.object(
                    sources, "run", side_effect=lambda *args, env=None: self.brew_metadata(args, info)) as run:
                with self.assertRaisesRegex(sources.CollectionError, "version mismatch"):
                    sources.collect_macos(self.stage, self.output, self.scratch)
                self.assertEqual(run.call_count, 2)
                self.assertEqual(list((self.repository / "Library/Taps/kapila-maker").iterdir()), [])

    def test_macos_installed_adwaita_50_snapshot_not_latest_api_51(self):
        self.mac_stage("50.0", "adwaita-icon-theme")
        recipe = next(self.stage.rglob("*.rb"))
        recipe.write_text('class AdwaitaIconTheme < Formula\n'
                          '  url "https://example.test/adwaita-50.0.tar.xz"\n'
                          '  sha256 "' + "a" * 64 + '"\n'
                          '  resource "icons" do\n'
                          '    url "https://example.test/icons-50.0.tar.xz"\n'
                          '    sha256 "' + "b" * 64 + '"\n  end\nend\n')
        original = recipe.read_bytes()
        info = self.mac_info("50.0", 0)
        formula = info["formulae"][0]
        formula["name"] = "adwaita-icon-theme"
        formula["urls"]["stable"] = {"url": "https://example.test/adwaita-50.0.tar.xz",
                                       "checksum": "a" * 64}
        formula["resources"] = [{"name": "icons", "url": "https://example.test/icons-50.0.tar.xz",
                                 "checksum": "b" * 64}]
        commands = []

        def brew(*args, env=None):
            commands.append(args)
            self.assertEqual(env.get("HOMEBREW_NO_INSTALL_FROM_API"), os.environ.get("HOMEBREW_NO_INSTALL_FROM_API"))
            if args == ("brew", "info", "--json=v2", "adwaita-icon-theme"):
                return json.dumps(self.mac_info("51.0", 0))
            if args[1] != "fetch":
                return self.brew_metadata(args, info)
            self.assertEqual(args, ("brew", "fetch", "--build-from-source", "--retry", commands[-2][-1]))
            downloads = Path(env["HOMEBREW_CACHE"]) / "downloads"
            downloads.mkdir()
            (downloads / "hash--adwaita-50.0.tar.xz").write_bytes(b"50.0 source")
            (downloads / "hash--icons-50.0.tar.xz").write_bytes(b"50.0 resource")
            return ""

        with patch.object(sources, "run", side_effect=brew):
            sources.collect_macos(self.stage, self.output, self.scratch)
        self.assertEqual(len(commands), 3)
        record = json.loads((self.output / "SOURCE-MANIFEST.json").read_text())["packages"][0]
        self.assertEqual(record["version"], "50.0")
        self.assertEqual(record["origin"], formula["urls"])
        self.assertEqual(record["resources"], formula["resources"])
        self.assertEqual(len(record["archives"]), 2)
        self.assertEqual(recipe.read_bytes(), original)
        self.assertEqual((self.output / record["installed_formula_snapshots"][0]).read_bytes(), original)
        self.assert_hashes(self.output)

    def test_macos_rejects_same_version_changed_recipe_or_missing_origin(self):
        self.mac_stage()
        for field in ("ruby_source_checksum", "urls"):
            info = self.mac_info()
            info["formulae"][0][field] = {}
            with self.subTest(field=field), patch.object(
                    sources, "run", side_effect=lambda *args, env=None: self.brew_metadata(args, info)) as run:
                with self.assertRaisesRegex(sources.CollectionError, "snapshot metadata mismatch"):
                    sources.collect_macos(self.stage, self.output, self.scratch)
                self.assertEqual(run.call_count, 2)

    def test_macos_private_tap_cleanup_on_fetch_failure_leaves_other_taps(self):
        self.mac_stage()
        existing = self.repository / "Library/Taps/kapila-maker/homebrew-runtime-sources-existing/Formula/user.rb"
        existing.parent.mkdir(parents=True)
        existing.write_bytes(b"user recipe")
        original = next(self.stage.rglob("*.rb")).read_bytes()

        def brew(*args, env=None):
            if args[1] == "fetch":
                raise sources.CollectionError("fetch failed")
            return self.brew_metadata(args)

        with patch.object(sources, "run", side_effect=brew):
            with self.assertRaisesRegex(sources.CollectionError, "fetch failed"):
                sources.main([str(self.stage)])
        self.assertEqual(list(existing.parents[2].iterdir()), [existing.parents[1]])
        self.assertEqual(existing.read_bytes(), b"user recipe")
        self.assertEqual(next(self.stage.rglob("*.rb")).read_bytes(), original)
        self.assertEqual(list((self.stage / "SOURCE-ARCHIVES").iterdir()), [])

    def test_macos_normalizes_zero_revision(self):
        self.mac_stage()
        self.assertEqual(sources.normalized_brew_version("1.2_0"), "1.2")
        self.assertEqual(sources.brew_version(self.mac_info(revision=0)["formulae"][0]), "2.0")
        self.assertNotEqual(sources.normalized_brew_version("1.2_1"), "1.2")

    def test_macos_requires_installed_recipe(self):
        self.mac_stage()
        next(self.stage.rglob("*.rb")).unlink()
        with patch.object(sources, "run", side_effect=lambda *args, env=None: self.brew_metadata(args)):
            with self.assertRaisesRegex(sources.CollectionError, "snapshot"):
                sources.collect_macos(self.stage, self.output, self.scratch)

    def test_macos_empty_cache_and_bottles_fail(self):
        self.mac_stage()
        for bottle in (False, True):
            def brew(*args, env=None):
                if args[1] != "fetch":
                    return self.brew_metadata(args)
                downloads = Path(env["HOMEBREW_CACHE"]) / "downloads"
                downloads.mkdir(parents=True, exist_ok=True)
                if bottle:
                    (downloads / "glib.bottle.tar.gz").write_bytes(b"bottle")
                return ""
            with self.subTest(bottle=bottle), patch.object(sources, "run", side_effect=brew):
                # Each collection uses a fresh private cache.
                scratch = self.scratch / str(bottle)
                scratch.mkdir()
                output = self.output / str(bottle)
                output.mkdir()
                with self.assertRaises(sources.CollectionError):
                    sources.collect_macos(self.stage, output, scratch)

    def test_macos_rejects_ambiguous_or_symlinked_snapshots_before_info(self):
        self.mac_stage()
        recipe = next(self.stage.rglob("*.rb"))
        extra = recipe.with_name("other.rb")
        extra.write_bytes(b"ambiguous recipe")
        for unsafe in ("ambiguous", "symlink"):
            if unsafe == "symlink":
                extra.unlink()
                original = self.root / "original.rb"
                recipe.rename(original)
                recipe.symlink_to(original)
            with self.subTest(unsafe=unsafe), patch.object(
                    sources, "run", side_effect=lambda *args, env=None: self.brew_metadata(args)) as run:
                with self.assertRaisesRegex(sources.CollectionError, "snapshot"):
                    sources.collect_macos(self.stage, self.output, self.scratch)
                self.assertEqual(run.call_count, 1)
                self.assertEqual(list((self.repository / "Library/Taps/kapila-maker").iterdir()), [])

    def test_windows_filters_license_only_owners(self):
        self.windows_stage()
        self.assertEqual(sources.windows_packages(self.stage), {"mingw-w64-x86_64-glib2": "2.86.3-1"})

    def test_windows_inventory_version_mismatch_fails(self):
        self.windows_stage()
        file = self.stage / "WINDOWS-RUNTIME-FILES.tsv"
        file.write_text(file.read_text().replace("2.86.3-1", "2.86.2-1"))
        with self.assertRaisesRegex(sources.CollectionError, "inventory mismatch"):
            sources.windows_packages(self.stage)

    def test_gnu_tar_uses_explicit_path_from_which_outside_windows(self):
        with patch.object(sources.sys, "platform", "linux"), patch.object(
                sources.shutil, "which", return_value=self.tar) as which:
            self.assertEqual(sources.gnu_tar(), self.tar)
        which.assert_called_once_with("tar")
        self.assertTrue(Path(shutil.which("tar")).is_absolute())

    def test_native_windows_collection_never_uses_system32_tar(self):
        self.windows_stage()
        archive = self.make_source_package()
        native_tar = r"C:\msys64\usr\bin\tar.exe"
        system_tar = r"C:\Windows\System32\tar.exe"
        real_run = sources.run
        for msystem in ("MINGW64", ""):
            commands = []

            def which(name):
                if name == "pacman-conf":
                    return r"C:\msys64\usr\bin\pacman-conf.exe"
                self.fail(f"Implicit executable lookup forbidden: {name} ({system_tar})")

            def run(*args, env=None):
                commands.append(args)
                if args == ("cygpath", "-w", "/usr/bin/tar.exe"):
                    return native_tar + "\n"
                self.assertEqual(args[0], native_tar)
                return real_run(self.tar, *args[1:])

            def download(url, target):
                target.write_bytes(archive.read_bytes())

            output = self.output / (msystem or "pacman-root")
            output.mkdir()
            with self.subTest(msystem=msystem), patch.object(
                    sources.sys, "platform", "win32"), patch.dict(
                    os.environ, {"MSYSTEM": msystem}), patch.object(
                    sources.shutil, "which", side_effect=which), patch.object(
                    sources, "run", side_effect=run), patch.object(
                    sources, "installed_pkgbase", return_value="mingw-w64-glib2"), patch.object(
                    sources, "download", side_effect=download), patch("builtins.print") as log:
                sources.collect_windows(self.stage, output, self.scratch)
            tar_calls = [args for args in commands if args[0] == native_tar]
            self.assertEqual(tar_calls[0], (native_tar, "--version"))
            self.assertEqual(len(tar_calls), 4)
            self.assertEqual([args[2] for args in tar_calls[1:]], ["-tf", "-xOf", "-xOf"])
            self.assertTrue(all(args[1] == "--force-local" for args in tar_calls[1:]))
            diagnostics = [call.args[0] for call in log.call_args_list
                           if call.args[0].startswith("MSYS2 source archive tool:")]
            self.assertEqual(len(diagnostics), 1)
            self.assertIn(native_tar, diagnostics[0])
            self.assertIn("GNU tar", diagnostics[0])
            self.assert_hashes(output)

    def test_native_windows_archive_drive_path_is_passed_with_force_local(self):
        archive = self.make_source_package()
        native_archive = r"D:\a\_temp\runtime-source-cache-test\package.src.tar.gz"
        native_tar = r"C:\msys64\usr\bin\tar.exe"
        real_run = sources.run
        commands = []

        def run(*args, env=None):
            commands.append(args)
            self.assertEqual(args[:2], (native_tar, "--force-local"))
            self.assertEqual(args[3], native_archive)
            return real_run(self.tar, *args[1:3], str(archive), *args[4:])

        with patch.object(sources, "run", side_effect=run):
            sources.validate_source_package(native_archive, "mingw-w64-glib2", "2.86.3-1",
                                            self.output / "recipe", native_tar)
        self.assertEqual(len(commands), 3)

    def test_non_gnu_tar_is_rejected_before_download(self):
        self.windows_stage()
        with patch.object(sources.sys, "platform", "win32"), patch.dict(
                os.environ, {"MSYSTEM": "MINGW64"}), patch.object(
                sources, "run", side_effect=[r"C:\msys64\usr\bin\tar.exe", "bsdtar 3.7.2\n"]), patch.object(
                sources, "download") as download:
            with self.assertRaisesRegex(sources.CollectionError, "Expected GNU tar"):
                sources.collect_windows(self.stage, self.output, self.scratch)
        download.assert_not_called()

    def test_exact_allsource_package_validation(self):
        archive = self.make_source_package()
        fields = sources.validate_source_package(archive, "mingw-w64-glib2", "2.86.3-1",
                                                self.output / "recipe", self.tar)
        self.assertEqual(fields["pkgver"], ["2.86.3"])
        self.assertTrue((self.output / "recipe/PKGBUILD").is_file())

    def test_srcinfo_empty_assignments_and_equals_in_values(self):
        self.assertEqual(sources.srcinfo_fields(
            "source = \nsource=   \t\npkgdesc = \n"
            "depends = mingw-w64-ucrt-x86_64-glib2\ndepends = \noptdepends = \n"
            "source_x86_64 = https://example.test/glib.tar.xz?a=b=c\n"
            " # source = ignored\nnot an assignment\n"), {
                "source": ["", ""], "pkgdesc": [""],
                "depends": ["mingw-w64-ucrt-x86_64-glib2", ""], "optdepends": [""],
                "source_x86_64": ["https://example.test/glib.tar.xz?a=b=c"]})

    def test_source_package_empty_global_sources_allow_arch_sources(self):
        text = ("pkgbase = mingw-w64-glib2\npkgver = 2.86.3\npkgrel = 1\n"
                "source = \nsource=   \npkgdesc = \n"
                "source_x86_64 = https://example.test/glib.tar.xz?a=b=c\n")
        archive = self.make_source_package(srcinfo=text)
        fields = sources.validate_source_package(
            archive, "mingw-w64-glib2", "2.86.3-1", self.output / "recipe", self.tar)
        self.assertEqual(fields["source"], ["", ""])
        self.assertEqual(fields["pkgdesc"], [""])
        self.assertEqual((self.output / "recipe/.SRCINFO").read_text(), text)

    def test_source_package_empty_sources_and_identity_fail_closed(self):
        text = ("pkgbase = mingw-w64-glib2\npkgver = 2.86.3\npkgrel = 1\n"
                "source_x86_64 = glib.tar.xz\n")
        cases = [(text.replace("source_x86_64 = glib.tar.xz",
                               "source = \nsource_x86_64=   "), "No sources declared")]
        for key, value in (("pkgbase", "mingw-w64-glib2"), ("pkgver", "2.86.3"),
                           ("pkgrel", "1"), ("epoch", "0")):
            original = text + "epoch = 0\n" if key == "epoch" else text
            cases.append((original.replace(f"{key} = {value}", f"{key} = "), "version mismatch"))
        for metadata, error in cases:
            with self.subTest(metadata=metadata), self.assertRaisesRegex(sources.CollectionError, error):
                sources.validate_source_package(
                    self.make_source_package(srcinfo=metadata), "mingw-w64-glib2", "2.86.3-1",
                    self.output / "recipe", self.tar)

    def test_mocked_srcinfo_empty_source_still_checks_arch_source_presence(self):
        text = ("pkgbase = mingw-w64-gdk-pixbuf2\npkgver = 2.44.8\npkgrel = 1\n"
                "source = \npkgdesc=   \nsource_x86_64 = gdk-pixbuf.tar.xz\n")
        for present in (False, True):
            members = "mingw-w64-gdk-pixbuf2/.SRCINFO\nmingw-w64-gdk-pixbuf2/PKGBUILD\n"
            if present:
                members += "mingw-w64-gdk-pixbuf2/gdk-pixbuf.tar.xz\n"
            with self.subTest(present=present), patch.object(
                    sources, "run", side_effect=[members, text, "# recipe\n"]):
                if present:
                    fields = sources.validate_source_package(
                        self.root / "package.src.tar.zst", "mingw-w64-gdk-pixbuf2", "2.44.8-1",
                        self.output / "recipe", self.tar)
                    self.assertEqual(fields["source"], [""])
                else:
                    with self.assertRaisesRegex(sources.CollectionError, "missing declared source"):
                        sources.validate_source_package(
                            self.root / "package.src.tar.zst", "mingw-w64-gdk-pixbuf2", "2.44.8-1",
                            self.output / "recipe", self.tar)

    def test_source_package_version_and_base_mismatch_fail(self):
        for base, version in (("mingw-w64-glib2", "2.86.2-1"), ("other", "2.86.3-1")):
            with self.subTest(base=base, version=version), self.assertRaises(sources.CollectionError):
                sources.validate_source_package(self.make_source_package(), base, version,
                                                self.output / "recipe", self.tar)

    def test_source_package_missing_upstream_source_fails(self):
        with self.assertRaisesRegex(sources.CollectionError, "missing declared source"):
            sources.validate_source_package(self.make_source_package(omit_source=True),
                                            "mingw-w64-glib2", "2.86.3-1", self.output / "recipe", self.tar)

    def test_windows_downloads_complete_archive_and_preserves_hashes(self):
        self.windows_stage()
        archive = self.make_source_package()

        def download(url, target):
            self.assertEqual(url, "https://repo.msys2.org/mingw/sources/mingw-w64-glib2-2.86.3-1.src.tar.zst")
            target.write_bytes(archive.read_bytes())

        with patch.object(sources, "installed_pkgbase", return_value="mingw-w64-glib2") as base:
            with patch.object(sources, "download", side_effect=download):
                sources.collect_windows(self.stage, self.output, self.scratch)
        base.assert_called_once_with("mingw-w64-x86_64-glib2", "2.86.3-1")
        result = json.loads((self.output / "SOURCE-MANIFEST.json").read_text())["packages"][0]
        self.assertEqual((self.output / result["archive"]).read_bytes(), archive.read_bytes())
        self.assert_hashes(self.output)

    def test_windows_unavailable_exact_sources_fail_without_recipe_substitution(self):
        self.windows_stage()
        error = urllib.error.HTTPError("url", 404, "not found", {}, None)
        with patch.object(sources, "installed_pkgbase", return_value="mingw-w64-glib2"):
            with patch.object(sources, "download", side_effect=error) as download:
                with self.assertRaisesRegex(sources.CollectionError, "release blocked"):
                    sources.collect_windows(self.stage, self.output, self.scratch)
                self.assertEqual(download.call_count, 3)

    def test_pacman_pkgbase_is_from_exact_installed_metadata(self):
        db = self.root / "pacman"
        desc = db / "local/mingw-w64-x86_64-gcc-libs-15.2.0-1/desc"
        desc.parent.mkdir(parents=True)
        desc.write_text("%NAME%\nmingw-w64-x86_64-gcc-libs\n\n%VERSION%\n15.2.0-1\n\n"
                        "%BASE%\nmingw-w64-gcc\n\n")
        with patch.object(sources, "run", return_value=str(db)), patch.object(sources.sys, "platform", "linux"):
            self.assertEqual(sources.installed_pkgbase("mingw-w64-x86_64-gcc-libs", "15.2.0-1"),
                             "mingw-w64-gcc")

    def test_license_readers_never_extract_archive_paths_or_links(self):
        archive = self.root / "source.tar"
        with tarfile.open(archive, "w") as target:
            for name in ("COPYING", "project/LICENSE", "../../NOTICE", "/COPYRIGHT"):
                item = tarfile.TarInfo(name)
                item.size = 4
                target.addfile(item, io.BytesIO(b"text"))
            link = tarfile.TarInfo("LICENSE")
            link.type = tarfile.SYMTYPE
            link.linkname = "/etc/passwd"
            target.addfile(link)
        sources.source_notices(archive, self.output / "notices")
        self.assertEqual(len(list((self.output / "notices").iterdir())), 2)
        archive = self.root / "source.zip"
        with zipfile.ZipFile(archive, "w") as target:
            target.writestr("project/COPYING", "notice")
            target.writestr("../LICENSE", "escape")
        sources.source_notices(archive, self.output / "zip-notices")
        self.assertEqual(len(list((self.output / "zip-notices").iterdir())), 1)

    def test_failed_collection_does_not_publish_or_touch_utilities(self):
        self.mac_stage()
        utility = self.stage / "SOURCE-ARCHIVES/utility.tar.xz"
        utility.parent.mkdir()
        utility.write_bytes(b"utility source")
        with patch.object(sources, "collect_macos", side_effect=sources.CollectionError("fetch failed")):
            with self.assertRaises(sources.CollectionError):
                sources.main([str(self.stage)])
        self.assertEqual(list(utility.parent.iterdir()), [utility])
        self.assertEqual(utility.read_bytes(), b"utility source")

    def test_linux_without_bundled_runtime_is_noop(self):
        with patch.object(sources.sys, "platform", "linux"):
            self.assertEqual(sources.main([str(self.stage)]), 0)
        self.assertFalse((self.stage / "SOURCE-ARCHIVES").exists())


if __name__ == "__main__":
    unittest.main()
