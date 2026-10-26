#!/usr/bin/env python3
"""Mirror verified GitHub release files to Gitee; publish latest manifests last."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

REPO = "kidmao100/dilmeter-releases"
API = f"https://gitee.com/api/v5/repos/{REPO}"


class NoAuthRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        if urllib.parse.urlparse(newurl).scheme != "https":
            raise RuntimeError("Refusing non-HTTPS redirect")
        redirect = super().redirect_request(req, fp, code, msg, headers, newurl)
        if redirect is not None:
            redirect.remove_header("Authorization")
        return redirect


def public_download(url):
    if urllib.parse.urlparse(url).scheme != "https":
        raise RuntimeError("Public download must use HTTPS")
    opener = urllib.request.build_opener(NoAuthRedirect())
    with opener.open(urllib.request.Request(url, headers={"User-Agent": "Dilmeter-release-check"}), timeout=120) as response:
        return response.read(501 << 20)


def validate_artifacts(directory, version, notes):
    result = {}
    for app in ("DilmeterCN", "DilmeterRT", "DilmeterOT"):
        manifest = json.loads((directory / f"{app}.json").read_text(encoding="utf-8-sig"))
        data = (directory / f"{app}.zip").read_bytes()
        if (manifest["version"] != version or manifest["size"] != len(data)
                or manifest["sha256"].lower() != hashlib.sha256(data).hexdigest()
                or manifest["notes"].replace("\r\n", "\n").strip() != notes.strip()):
            raise RuntimeError(f"Release validation failed: {app}")
        result[app] = manifest
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--release-dir", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--notes-file", type=Path, required=True)
    parser.add_argument("--token-file", type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r"\d+\.\d+\.\d+", args.version):
        raise RuntimeError("Expected a stable release version")
    notes = args.notes_file.read_text(encoding="utf-8-sig").replace("\r\n", "\n").strip()
    manifests = validate_artifacts(args.release_dir, args.version, notes)
    token = (args.token_file.read_text(encoding="utf-8-sig") if args.token_file
             else os.environ.get("GITEE_ACCESS_TOKEN", "")).strip()
    if not token or "\n" in token or "\r" in token:
        raise RuntimeError("GITEE_ACCESS_TOKEN or a valid --token-file is required")
    opener = urllib.request.build_opener(NoAuthRedirect())

    def request(path, method="GET", fields=None, upload=None, allow_missing=False):
        headers = {"Authorization": "Bearer " + token, "Accept": "application/json"}
        data = None
        if upload:
            boundary = "dilmeter-" + uuid.uuid4().hex
            headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
            data = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{upload.name}"\r\n'
                    'Content-Type: application/octet-stream\r\n\r\n').encode() + upload.read_bytes() + f"\r\n--{boundary}--\r\n".encode()
        elif fields is not None:
            headers["Content-Type"] = "application/x-www-form-urlencoded"
            data = urllib.parse.urlencode(fields).encode()
        try:
            with opener.open(urllib.request.Request(API + path, data=data, headers=headers, method=method), timeout=180) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            if allow_missing and error.code == 404:
                return None
            # Do not log request headers, credentials, or server response bodies.
            raise RuntimeError(f"Gitee API {method} {path}: HTTP {error.code}") from None

    tag = "v" + args.version
    release = request("/releases/tags/" + tag, allow_missing=True)
    if release is None:
        release = request("/releases", "POST", {
            "tag_name": tag, "name": "Dilmeter " + tag, "body": notes,
            "target_commitish": "master", "prerelease": "true",
        })
    release_path = f'/releases/{release["id"]}'
    attachment_path = release_path + "/attach_files"
    existing = request(attachment_path)
    if not isinstance(existing, list):
        raise RuntimeError("Unexpected Gitee attachment response")
    urls = {}
    for app, manifest in manifests.items():
        name = app + ".zip"
        matches = [entry for entry in existing if entry["name"] == name]
        if len(matches) > 1:
            raise RuntimeError(f"Duplicate existing attachment: {name}")
        entry = matches[0] if matches else request(attachment_path, "POST", upload=args.release_dir / name)
        url = entry["browser_download_url"]
        data = public_download(url)
        if len(data) != manifest["size"] or hashlib.sha256(data).hexdigest() != manifest["sha256"].lower():
            raise RuntimeError(f"Unauthenticated download verification failed: {name}")
        urls[app] = url
        print(f"Verified public attachment: {name}", flush=True)

    # Publish only after every ZIP is anonymously downloadable and byte-identical.
    request(release_path, "PATCH", {"tag_name": tag, "name": "Dilmeter " + tag, "body": notes, "prerelease": "false"})
    for app, original in manifests.items():
        manifest = dict(original, url=urls[app])
        payload = (json.dumps(manifest, ensure_ascii=False, indent=2) + "\n").encode()
        path = f"/contents/latest/{app}.json"
        old = request(path + "?ref=master", allow_missing=True)
        fields = {"content": base64.b64encode(payload).decode(), "message": f"Publish {app} {tag}", "branch": "master"}
        if old:
            fields["sha"] = old["sha"]
        request(path, "PUT" if old else "POST", fields)
        public = public_download(f"https://gitee.com/{REPO}/raw/master/latest/{app}.json?version={args.version}")
        if json.loads(public) != manifest:
            raise RuntimeError(f"Public latest manifest verification failed: {app}")
    report = {"version": args.version, "release": f"https://gitee.com/{REPO}/releases/tag/{tag}", "downloads": urls}
    (args.release_dir / "gitee-verification.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"Gitee mirror incomplete: {error}", file=sys.stderr)
        sys.exit(1)
