"""Portable app-layout tests. Native codesign and DMG tests require macOS."""

from pathlib import Path
import plistlib
import tempfile
import unittest
from unittest.mock import patch

import make_app


class AppTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.stage = self.root / "flat stage with spaces"
        self.stage.mkdir()
        self.contents = self.stage / make_app.APP_NAME / "Contents"
        self.plist = self.root / "Info.plist"
        self.plist.write_bytes(plistlib.dumps({"CFBundleExecutable": "kapila-maker",
                                              "CFBundlePackageType": "APPL"}))
        self.meta = {}
        for name in make_app.EXECUTABLES:
            self.image(name)
        for name in make_app.HELPERS:
            self.image("tools/bin/" + name)
        self.text("README.txt", "readme")
        self.text("launch-gtk.command", "old flat launcher")
        self.text("share/glib-2.0/schemas/gschemas.compiled")
        self.text("etc/fonts/fonts.conf")
        self.text("tools/share/dvda-author/config.xml")
        self.text("tools/share/licenses/COPYING")
        self.text("THIRD-PARTY-LICENSES/glib/COPYING")
        self.text("THIRD-PARTY.txt")
        self.text("MACOS-RUNTIME-MANIFEST.json", "[]")
        self.text("BUILD-UTILITIES.txt")

    def text(self, relative, text="data"):
        path = self.stage / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        return path

    def image(self, relative, deps=(), rpaths=(), identity=None):
        path = self.stage / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b"\xcf\xfa\xed\xfe" + b"fake Mach-O")
        path.chmod(0o755)
        self.meta[path] = (list(deps), list(rpaths), identity)
        return path

    def test_destination_mapping(self):
        cases = {
            "kapila-maker": "MacOS/kapila-maker",
            "kapila-maker-web": "MacOS/kapila-maker-web",
            "tools/bin/ffmpeg": "Helpers/ffmpeg",
            "lib/libgtk-3.0.dylib": "Frameworks/libgtk-3.0.dylib",
            "lib/gdk-pixbuf-2.0/2.10.0/loaders/svg.so":
                "Frameworks/gdk-pixbuf-2.0/2.10.0/loaders/svg.so",
            "tools/share/dvda-author/config.xml": "Resources/tools/share/dvda-author/config.xml",
            "share/icons/Adwaita/index.theme": "Resources/share/icons/Adwaita/index.theme",
            "etc/fonts/fonts.conf": "Resources/etc/fonts/fonts.conf",
            "THIRD-PARTY-LICENSES/COPYING": "Resources/THIRD-PARTY-LICENSES/COPYING",
            "MACOS-RUNTIME-MANIFEST.json": "Resources/MACOS-RUNTIME-MANIFEST.json",
            "BUILD-UTILITIES.txt": "Resources/BUILD-UTILITIES.txt",
            "README.txt": "Resources/README.txt",
        }
        for old, new in cases.items():
            with self.subTest(old=old):
                self.assertEqual(make_app.destination(Path(old)), Path(new))
        with self.assertRaisesRegex(RuntimeError, "Unrecognized"):
            make_app.destination(Path("SOURCE-ARCHIVES/source.tar.gz"))

    def test_moves_rewrites_and_signs_inside_out(self):
        system = "/usr/lib/libSystem.B.dylib"
        library = self.image("lib/libfirst.dylib", ["@loader_path/libsecond.dylib", system],
                             identity="@rpath/libfirst.dylib")
        self.image("lib/libsecond.dylib", identity="@rpath/libsecond.dylib")
        module = self.image("lib/gdk-pixbuf-2.0/2.10.0/loaders/svg.so",
                            ["@loader_path/../../../libfirst.dylib"])
        self.meta[self.stage / "kapila-maker"] = (["@loader_path/lib/libfirst.dylib"], [], None)
        self.meta[self.stage / "tools/bin/ffmpeg"] = (
            ["@loader_path/../../lib/libfirst.dylib"], ["/old/Homebrew/lib"], None)
        events = []
        original_images = {path: path.read_bytes() for path in self.meta}

        def metadata(path):
            # Metadata must be read while every original image still exists.
            self.assertTrue(all(source.exists() for source in self.meta))
            self.assertFalse(self.contents.exists())
            events.append(("metadata", path))
            return self.meta[path]

        def run(*args):
            self.assertTrue(all((self.contents / make_app.destination(p.relative_to(self.stage))).exists()
                                for p in self.meta))
            self.assertNotIn("--remove-signature", args)
            if args[0] == "install_name_tool":
                original = next(p for p in self.meta if self.contents /
                                make_app.destination(p.relative_to(self.stage)) == args[-1])
                self.assertEqual(args[-1].read_bytes(), original_images[original])
            events.append(args)
            return ""

        with patch.object(make_app, "metadata", side_effect=metadata), \
                patch.object(make_app, "run", side_effect=run):
            app = make_app.make_app(self.stage, self.plist)

        expected = self.contents / "Frameworks/libfirst.dylib"
        self.assertTrue(expected.is_file())
        self.assertFalse(library.exists())
        self.assertFalse(module.exists())
        self.assertEqual({p.name for p in (self.contents / "MacOS").iterdir()},
                         set(make_app.EXECUTABLES))
        self.assertEqual({p.name for p in (self.contents / "Helpers").iterdir()},
                         set(make_app.HELPERS))
        self.assertTrue((self.contents / "Resources/tools/share/dvda-author/config.xml").is_file())
        self.assertTrue((self.contents / "Resources/etc/fonts/fonts.conf").is_file())
        self.assertEqual((self.contents / "Resources/README.txt").read_text(), "readme")
        self.assertEqual((self.stage / "README.txt").read_text(), "readme")
        self.assertFalse(any(path.is_symlink() for path in app.rglob("*")))
        for name in make_app.EXECUTABLES:
            self.assertEqual((self.stage / name).resolve(), self.contents / "MacOS" / name)
        launcher = (self.stage / "launch-gtk.command").read_text()
        self.assertIn('exec "$directory/kapila-maker.app/Contents/MacOS/kapila-maker" "$@"', launcher)
        self.assertNotIn("FONTCONFIG", launcher)

        patches = [event for event in events if event[0] == "install_name_tool"]
        for target in (self.contents / "MacOS/kapila-maker", self.contents / "Helpers/ffmpeg"):
            command = next(event for event in patches if event[-1] == target)
            self.assertIn("@loader_path/../Frameworks/libfirst.dylib", command)
        helper = next(event for event in patches if event[-1] == self.contents / "Helpers/ffmpeg")
        self.assertIn("-delete_rpath", helper)
        library_edit = next(event for event in patches if event[-1] == expected)
        self.assertIn("@rpath/libfirst.dylib", library_edit)
        # Module and dylib dependencies retain correct relative paths within Frameworks.
        self.assertNotIn("-change", library_edit)
        self.assertFalse(any(event[-1] == self.contents /
                             "Frameworks/gdk-pixbuf-2.0/2.10.0/loaders/svg.so" for event in patches))
        self.assertFalse(any(system in event for event in patches))
        first_mutation = next(i for i, event in enumerate(events) if event[0] != "metadata")
        self.assertEqual(first_mutation, len(self.meta))
        signings = [event for event in events if event[:2] == ("codesign", "--force")]
        self.assertEqual(signings[-1][-1], app)
        self.assertTrue(all("--deep" not in event for event in signings))
        depths = [len(event[-1].parts) for event in signings[:-1]]
        self.assertEqual(depths, sorted(depths, reverse=True))
        last_patch = max(i for i, event in enumerate(events) if event[0] == "install_name_tool")
        first_sign = next(i for i, event in enumerate(events) if event[:2] == ("codesign", "--force"))
        self.assertLess(last_patch, first_sign)
        self.assertEqual(events[-1], ("codesign", "--verify", "--deep", "--strict", app))

    def test_bad_dependencies_fail_before_move(self):
        for dep in ("@loader_path/lib/missing.dylib", "/usr/local/opt/gtk/libgtk.dylib",
                    "@rpath/libgtk.dylib", "@loader_path/../outside.dylib"):
            with self.subTest(dep=dep), patch.object(make_app, "metadata", side_effect=self.meta.__getitem__):
                self.meta[self.stage / "kapila-maker"] = ([dep], [], None)
                with self.assertRaises(RuntimeError):
                    make_app.make_app(self.stage, self.plist)
                self.assertFalse(self.contents.exists())
                self.assertTrue((self.stage / "kapila-maker").is_file())

    def test_code_in_resources_is_rejected(self):
        self.image("share/hidden-code.dylib")
        with patch.object(make_app, "metadata", side_effect=self.meta.__getitem__):
            with self.assertRaisesRegex(RuntimeError, "cannot be installed in Resources"):
                make_app.make_app(self.stage, self.plist)
        self.assertFalse(self.contents.exists())

    def test_missing_helper_is_rejected(self):
        (self.stage / "tools/bin/ffmpeg").unlink()
        with self.assertRaisesRegex(RuntimeError, "Required native Mach-O"):
            make_app.make_app(self.stage, self.plist)
        self.assertFalse(self.contents.exists())

    def test_existing_app_is_not_overwritten(self):
        self.contents.mkdir(parents=True)
        marker = self.contents / "untouched"
        marker.write_text("existing app")
        with self.assertRaisesRegex(RuntimeError, "Refusing to overwrite"):
            make_app.make_app(self.stage, self.plist)
        self.assertEqual(marker.read_text(), "existing app")


if __name__ == "__main__":
    unittest.main()
