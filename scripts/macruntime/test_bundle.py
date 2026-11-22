"""Portable dependency-graph tests; real Mach-O/signature checks run on macOS."""

import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import bundle


class RuntimeTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.stage = self.root / "package with spaces"
        self.stage.mkdir()
        self.cellar = self.root / "Cellar"
        self.cellar.mkdir()
        self.builder = bundle.Bundler.__new__(bundle.Bundler)
        self.builder.stage = self.stage
        self.builder.cellar = self.cellar
        self.builder.sources = {}
        self.builder.visited = set()
        self.builder.info = {}
        self.builder.kegs = set()
        self.builder.search = []

    def image(self, path, contents=b"library"):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b"\xcf\xfa\xed\xfe" + contents)
        return path

    def test_load_command_parser(self):
        text = """Load command 0
          cmd LC_LOAD_DYLIB
         name /some path/libtest.dylib (offset 24)
Load command 1
          cmd LC_RPATH
         path @loader_path/../lib (offset 12)
Load command 2
          cmd LC_ID_DYLIB
         name @rpath/self.dylib (offset 24)
"""
        with patch.object(bundle, "run", side_effect=["", text]):
            self.assertEqual(bundle.metadata(Path("test")),
                             (["/some path/libtest.dylib"], ["@loader_path/../lib"],
                              "@rpath/self.dylib"))

    def test_recursive_graph_and_nested_module(self):
        main = self.image(self.stage / "kapila-maker")
        utility = self.image(self.stage / "tools/bin/ffmpeg")
        module_source = self.image(self.root / "modules/loader.so")
        module = self.stage / "lib/gdk-pixbuf-2.0/2.10.0/loaders/loader.so"
        first = self.image(self.cellar / "first/1/lib/libfirst.dylib")
        second = self.image(self.cellar / "second/2/lib/libsecond.dylib")
        self.builder.copy(module_source, module)
        self.builder.sources[main] = main
        self.builder.sources[utility] = utility
        self.builder.info = {
            main: (["@rpath/libfirst.dylib"], [str(first.parent)], None),
            utility: ([str(first)], [], None),
            module_source: ([str(first)], [], None),
            first: ([str(second)], [], str(first)),
            second: (["/usr/lib/libSystem.B.dylib"], [], str(second)),
        }
        commands = []

        def fake_run(*args):
            commands.append(tuple(map(str, args)))
            return "x86_64" if args[:2] == ("lipo", "-archs") else ""

        with patch.object(bundle, "run", side_effect=fake_run), \
                patch.object(bundle.subprocess, "run"):
            self.builder.walk(main, main)
            self.builder.walk(module, main)
            self.builder.walk(utility, utility)
        self.assertTrue((self.stage / "lib/libsecond.dylib").is_file())
        edits = [command for command in commands if command[0] == "install_name_tool"]
        self.assertTrue(any("@loader_path/libsecond.dylib" in edit for edit in edits))
        self.assertTrue(any("@loader_path/../../../libfirst.dylib" in edit for edit in edits))
        self.assertTrue(any("@loader_path/lib/libfirst.dylib" in edit for edit in edits))
        self.assertTrue(any("@loader_path/../../lib/libfirst.dylib" in edit for edit in edits))
        self.assertTrue(any("-delete_rpath" in edit for edit in edits))

    def test_executable_and_inherited_rpaths(self):
        source = self.image(self.root / "modules/loader.so")
        executable = self.image(self.root / "bin/app")
        dependency = self.image(self.root / "lib/libtest.dylib")
        rpath = self.builder.expand("@executable_path/../lib", source, executable)
        self.assertEqual(self.builder.resolve("@rpath/libtest.dylib", source,
                                              executable, [rpath]), dependency)
        with self.assertRaisesRegex(RuntimeError, "Cannot resolve"):
            self.builder.resolve("@rpath/missing.dylib", source, executable, [])

    def test_basename_collision_is_fatal(self):
        first = self.image(self.root / "first/libtest.dylib", b"one")
        second = self.image(self.root / "second/libtest.dylib", b"two")
        target = self.stage / "lib/libtest.dylib"
        self.builder.copy(first, target)
        with self.assertRaisesRegex(RuntimeError, "collision"):
            self.builder.copy(second, target)

    def test_verifier_rejects_external_and_escaping_references(self):
        self.image(self.stage / "app")
        self.image(self.root / "outside.dylib")
        for dependency in ("/usr/local/opt/foo/lib/foo.dylib", "@rpath/foo.dylib",
                           "@loader_path/../outside.dylib"):
            with self.subTest(dependency=dependency), \
                    patch.object(bundle, "metadata", return_value=([dependency], [], None)):
                with self.assertRaises(RuntimeError):
                    bundle.verify(self.stage)

    def test_external_symlinks_are_rejected(self):
        source = self.image(self.root / "outside.dylib")
        (self.stage / "external.dylib").symlink_to(source)
        with self.assertRaisesRegex(RuntimeError, "external staged symlink"):
            bundle.images(self.stage)


if __name__ == "__main__":
    unittest.main()
