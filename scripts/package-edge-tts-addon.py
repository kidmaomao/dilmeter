"""Build a pinned, portable Windows x64 speech addon. Requires Python 3.10+ to build.

No pip, system Python installation or environment changes are needed by players.
Only pinned official CPython, PyPI and GNU files in the reviewed lock are downloaded.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
SUPPORT = Path(__file__).resolve().parent / "edge-tts-addon"


def download(item, cache):
    target = cache / item["filename"]
    if not target.is_file() or hashlib.sha256(target.read_bytes()).hexdigest() != item["sha256"]:
        print("Downloading", item["filename"], flush=True)
        for attempt in range(3):
            try:
                with urllib.request.urlopen(item["url"], timeout=20) as response:
                    data = response.read()
                break
            except (urllib.error.URLError, TimeoutError, OSError):
                if attempt == 2:
                    raise
                time.sleep(1)
        if hashlib.sha256(data).hexdigest() != item["sha256"]:
            raise ValueError("Download checksum mismatch: " + item["filename"])
        target.write_bytes(data)
    return target


def extract_zip(archive_path, destination, wheel=False):
    with zipfile.ZipFile(archive_path) as archive:
        for entry in archive.infolist():
            if entry.is_dir():
                continue
            name = PurePosixPath(entry.filename)
            if name.is_absolute() or ".." in name.parts or "\\" in entry.filename or ":" in entry.filename:
                raise ValueError("Unsafe archive path: " + entry.filename)
            if wheel and name.parts[0].endswith(".data"):
                # Application imports libraries directly; CLI entry point scripts are unnecessary.
                if name.parts[1] == "scripts":
                    continue
                if name.parts[1] not in ("purelib", "platlib"):
                    raise ValueError("Unsupported wheel data directory: " + entry.filename)
                name = PurePosixPath(*name.parts[2:])
            target = destination.joinpath(*name.parts)
            target.parent.mkdir(parents=True, exist_ok=True)
            if target.exists():
                raise ValueError("Archive file collision: " + str(target))
            target.write_bytes(archive.read(entry))


def verify_install(root, lock):
    python = root / "addons/edge-tts/python/python.exe"
    expected = {item["name"]: item["version"] for item in lock["wheels"]}
    probe = """import sys, json, ssl, certifi, edge_tts, aiohttp
from importlib.metadata import version
expected = json.loads(sys.stdin.read())
for name, wanted in expected.items():
 assert version(name) == wanted, (name, version(name), wanted)
assert sys.flags.isolated == 1
ssl.create_default_context(cafile=certifi.where())
print(json.dumps(dict(python=sys.version.split()[0], edgeTts=version('edge-tts'), isolated=True)))
"""
    env = dict(os.environ, PATH="", PYTHONHOME=str(root / "nonexistent"), PYTHONPATH=str(root / "nonexistent"))
    result = subprocess.run([str(python), "-B", "-c", probe], input=json.dumps(expected),
                            text=True, capture_output=True, env=env, cwd=root, timeout=30, check=True)
    details = json.loads(result.stdout)
    if details["python"] != lock["python"]["version"]:
        raise ValueError("Unexpected embedded Python version")
    print("Verified without system Python:", result.stdout.strip(), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "artifacts/edge-tts-addon-v1.0.0")
    args = parser.parse_args()
    if os.name != "nt":
        parser.error("Build on Windows so the delivered Python runtime can be verified")
    lock = json.loads((SUPPORT / "lock.json").read_text(encoding="utf-8"))
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    cache = ROOT / ".cache/edge-tts-addon/downloads"
    cache.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="addon-build-", dir=output) as temporary:
        work = Path(temporary)
        stage = work / "package"
        addon = stage / "addons/edge-tts"
        runtime = addon / "python"
        extract_zip(download(lock["python"], cache), runtime)
        # _pth isolates the runtime from registry, PATH, PYTHONHOME and user site packages.
        # No `import site`: imports are limited to these explicit, relocatable directories.
        pth = next(runtime.glob("python*._pth"))
        library_zip = "python" + "".join(lock["python"]["version"].split(".")[:2]) + ".zip"
        pth.write_text(library_zip + "\n.\nLib/site-packages\n", encoding="utf-8")
        for item in lock["wheels"]:
            extract_zip(download(item, cache), runtime / "Lib/site-packages", wheel=True)
            if "source" in item:
                source = download(item["source"], cache)
                (addon / "sources").mkdir(exist_ok=True)
                shutil.copyfile(source, addon / "sources" / source.name)
                with tarfile.open(source) as archive:
                    for entry in archive.getmembers():
                        if entry.isfile() and PurePosixPath(entry.name).name in ("LICENSE", "gpl-3.0.txt"):
                            target = addon / "licenses" / PurePosixPath(entry.name).name
                            target.parent.mkdir(exist_ok=True)
                            target.write_bytes(archive.extractfile(entry).read())
        for item in lock["licenses"]:
            (addon / "licenses").mkdir(exist_ok=True)
            license_path = SUPPORT / "licenses" / item["filename"]
            if hashlib.sha256(license_path.read_bytes()).hexdigest() != item["sha256"]:
                raise ValueError("License checksum mismatch: " + item["filename"])
            shutil.copyfile(license_path, addon / "licenses" / item["filename"])
        for required in (runtime / "LICENSE.txt", addon / "licenses/LICENSE", addon / "licenses/gpl-3.0.txt"):
            if not required.is_file():
                raise ValueError("Missing redistribution license: " + str(required))
        shutil.copyfile(SUPPORT / "lock.json", addon / "bundle-lock.json")
        shutil.copyfile(SUPPORT / "README.txt", stage / "晓晓语音附加包-使用说明.txt")
        verify_install(stage, lock)
        name = "Dilmeter-EdgeTTS-Addon-v" + lock["addonVersion"] + "-win-x64.zip"
        temporary_zip = work / name
        with zipfile.ZipFile(temporary_zip, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for path in sorted(stage.rglob("*")):
                if path.is_file():
                    info = zipfile.ZipInfo(path.relative_to(stage).as_posix(), (2026, 1, 1, 0, 0, 0))
                    info.compress_type = zipfile.ZIP_DEFLATED
                    info.external_attr = 0o644 << 16
                    archive.writestr(info, path.read_bytes(), compresslevel=9)
        # Check the actual deliverable after relocation to a path with Chinese characters/spaces.
        installed = work / "中文 软件目录"
        extract_zip(temporary_zip, installed)
        verify_install(installed, lock)
        digest = hashlib.sha256(temporary_zip.read_bytes()).hexdigest()
        final_zip = output / name
        os.replace(temporary_zip, final_zip)
        manifest = dict(version=lock["addonVersion"], minAppVersion=lock["minAppVersion"],
                        platform=lock["platform"], filename=name, size=final_zip.stat().st_size, sha256=digest)
        (output / "Dilmeter-EdgeTTS-Addon.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(manifest, indent=2))
        print("Created", final_zip)


if __name__ == "__main__":
    main()
