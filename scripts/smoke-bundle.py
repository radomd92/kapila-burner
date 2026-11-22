#!/usr/bin/env python3
"""Test a relocated distribution without external authoring tools or any burning."""
import json
import os
from pathlib import Path
import queue
import re
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request


def main():
    stage = Path(sys.argv[1]).resolve()
    app_layout = (stage / "MacOS").is_dir() and (stage / "Helpers").is_dir()
    executables = stage / "MacOS" if app_layout else stage
    utilities = stage / "Helpers" if app_layout else stage / "tools/bin"
    suffix = ".exe" if sys.platform == "win32" or os.environ.get("MSYSTEM") else ""
    env = os.environ.copy()
    env["G_DEBUG"] = "fatal-criticals"
    for key in ("FFMPEG", "FFPROBE", "DVDA_AUTHOR", "MKISOFS", "VCDIMAGER", "CDRDAO",
                "DYLD_LIBRARY_PATH", "DYLD_FALLBACK_LIBRARY_PATH", "GDK_PIXBUF_MODULE_FILE"):
        env.pop(key, None)
    system = str(Path(env.get("SYSTEMROOT", "C:/Windows")) / "System32") if suffix else "/usr/bin:/bin:/usr/sbin:/sbin"
    linux = sys.platform.startswith("linux")
    if linux:
        # Keep system tools visible so PATH fallback cannot masquerade as discovery.
        env["PATH"] = system
        env.pop("LD_LIBRARY_PATH", None)
    else:
        env["PATH"] = os.pathsep.join((str(executables), str(utilities), system))
    with tempfile.TemporaryDirectory(prefix="kapila relocated ") as temporary:
        work = Path(temporary)
        media, output = work / "media inputs", work / "output images"
        media.mkdir()
        subprocess.run([str(utilities / ("ffmpeg" + suffix)), "-nostdin", "-v", "error",
                        "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-ac", "2",
                        "-ar", "44100", "-c:a", "pcm_s16le", str(media / "sample.wav")],
                       cwd=work, env=env, check=True, timeout=60)
        server = subprocess.Popen([str(executables / ("kapila-maker-web" + suffix)), "-root", str(media),
                                   "-output", str(output), "-listen", "127.0.0.1:0"],
                                  cwd=work, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                  text=True, errors="replace")
        lines = queue.Queue()

        def read_output():
            for line in server.stdout:
                print(line.rstrip(), flush=True)
                lines.put(line)

        threading.Thread(target=read_output, daemon=True).start()
        try:
            deadline, base = time.monotonic() + 60, None
            while time.monotonic() < deadline and base is None:
                try:
                    line = lines.get(timeout=1)
                except queue.Empty:
                    if server.poll() is not None:
                        raise RuntimeError("Bundled web process exited during startup")
                    continue
                match = re.search(r"http://127\.0\.0\.1:\d+", line)
                if match:
                    base = match.group()
            if base is None:
                raise RuntimeError("Bundled server startup timed out")

            def request(path, body=None):
                data = json.dumps(body).encode() if body is not None else None
                req = urllib.request.Request(base + path, data=data,
                                             headers={"Content-Type": "application/json"})
                with urllib.request.urlopen(req, timeout=30) as response:
                    return json.load(response)

            status = request("/api/status?lang=en")
            expected = {"ffmpeg", "ffprobe", "dvda-author", "mkisofs", "vcdimager"}
            for tool in status["tools"]:
                if tool["name"] in expected:
                    path = Path(tool["path"]).resolve()
                    if (not tool["ready"] or not path.is_relative_to(stage)
                            or (linux and path != utilities / tool["name"])):
                        raise RuntimeError(f"Bundled utility unavailable or external: {tool}")
                    expected.remove(tool["name"])
            if expected:
                raise RuntimeError(f"Missing utility status: {expected}")
            sources = request("/api/library?lang=en")["files"]
            if len(sources) != 1 or sources[0].get("error"):
                raise RuntimeError(f"Bundled FFprobe failed: {sources}")
            for kind, standard in (("dvd-audio", ""), ("vcd", "pal"), ("vcd", "ntsc")):
                project = {"name": "Bundled runtime smoke", "format": kind, "videoStandard": standard,
                           "sampleRate": 44100, "bits": 16, "channels": 2, "iso": True,
                           "capacity": "dvd5" if kind == "dvd-audio" else "cd80",
                           "groups": [{"name": "Test", "tracks": ["sample.wav"]}]}
                estimate = request("/api/estimate?lang=en", project)
                if not estimate["fits"]:
                    raise RuntimeError(f"Synthetic track exceeds capacity: {estimate}")
                job = request("/api/jobs?lang=en", project)
                deadline = time.monotonic() + 120
                while job["state"] == "running" and time.monotonic() < deadline:
                    time.sleep(.2)
                    job = request("/api/status?lang=en")["job"]
                if job["state"] != "completed":
                    raise RuntimeError(f"Bundled {kind}/{standard} authoring failed: {job}")
                image = Path(job["output"])
                if not image.is_file() or image.stat().st_size == 0:
                    raise RuntimeError(f"Missing authored image: {image}")
                if kind == "vcd" and not image.with_suffix(".bin").is_file():
                    raise RuntimeError("VCD CUE has no paired BIN")
                if kind == "dvd-audio":
                    with image.open("rb") as stream:
                        header = stream.read(64 * 2048)
                    if not all(marker in header for marker in (b"CD001", b"BEA01", b"NSR02", b"TEA01")):
                        raise RuntimeError("DVD-Audio image lacks ISO/UDF volume descriptors")
                print(f"Relocated bundle PASS: {kind} {standard}: {image.name}", flush=True)
        finally:
            server.terminate()
            try:
                server.wait(timeout=10)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
        for language in ("plt", "en", "fr"):
            subprocess.run([str(executables / ("kapila-maker" + suffix)), "-root", str(media),
                            "-output", str(output), "-lang", language, "-smoke-test"],
                           cwd=work, env=env, check=True, timeout=90)
    print("Relocated GUI and all authoring utilities PASS; no physical burning", flush=True)


if __name__ == "__main__":
    main()
