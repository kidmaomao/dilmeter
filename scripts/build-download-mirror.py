"""Build a link-only snapshot of the public gear downloads page for Netlify.

Only this small allowlist is fetched. Release ZIPs, logs and credentials are
never copied. Rerun after every public release to refresh the static fallback.
"""
import argparse
import re
import urllib.request
import zipfile
from pathlib import Path
from urllib.parse import urljoin

BASE = 'https://gear.noginogi.sbs/'
FILES = ('downloads/index.html', 'downloads/downloads.css',
         'downloads/dilmeter-downloads.js', 'site.css', 'site.js', 'assets/favicon.ico')


def build(output):
    output.mkdir(parents=True, exist_ok=True)
    # Refuse a dirty publish folder instead of accidentally including binaries.
    existing = {p.relative_to(output).as_posix() for p in output.rglob('*') if p.is_file()}
    allowed = set(FILES) | {'index.html', '_redirects', '_headers'}
    if existing - allowed:
        raise ValueError(f'Unexpected files in mirror: {existing - allowed}')
    for name in FILES:
        request = urllib.request.Request(urljoin(BASE, name), headers={'User-Agent': 'Dilmeter-download-mirror/1.0'})
        with urllib.request.urlopen(request, timeout=30) as response:
            data = response.read()
        if name.endswith(('.html', '.css', '.js')):
            text = data.decode('utf-8')
            if name == 'downloads/index.html':
                text = re.sub(r'href="([^"\s]+\.zip)"', lambda m: 'href="'+urljoin(BASE+'downloads/', m[1])+'"', text)
                text = text.replace('本站直链下载', 'gear 本站下载')
                text = text.replace('<main class="downloads-shell">', '<main class="downloads-shell">\n<p class="mirror-notice">Netlify 下载页镜像 · 文件由 gear / GitHub / Gitee / 蓝奏云提供。<a href="https://gear.noginogi.sbs/downloads/">访问主站</a></p>')
                text = text.replace('rel="noopener noreferrer" hidden>GitHub', 'rel="noopener noreferrer">GitHub')
                text = text.replace('<title>软件下载 - noginogi</title>', '<title>软件下载镜像 - noginogi</title>\n<link rel="canonical" href="https://gear.noginogi.sbs/downloads/" />')
            elif name == 'site.js':
                # Other sections remain at gear, not nonexistent mirror routes.
                text = re.sub(r'href: "(/[^\"]+)"', lambda m: 'href: "'+(m[1] if m[1]=='/downloads/' else urljoin(BASE,m[1]))+'"',text)
                text = text.replace('brand.href = "/index.html"', 'brand.href = "https://gear.noginogi.sbs/"')
            elif name.endswith('.css'):
                text = re.sub(r'url\(([\"\']?)(?!data:|https?:|#)([^)\"\']+)\1\)', lambda m: 'url("'+urljoin(urljoin(BASE,name),m[2])+'")',text)
                if name == 'downloads/downloads.css':
                    text += '\n.mirror-notice { margin: 0 0 14px; padding: 12px 16px; border: 1px solid var(--line); border-radius: 8px; background: var(--panel); color: var(--text); font-size: 14px; line-height: 1.8; }\n.mirror-notice a { color: var(--accent-strong); font-weight: 700; text-decoration: underline; white-space: nowrap; }\n'
            data = text.encode('utf-8')
        target = output / name
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_bytes(data)
    (output/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=/downloads/"><a href="/downloads/">软件下载</a>',encoding='utf-8')
    # 302 only: never proxy large binary downloads through Netlify.
    links = re.findall(r'href="(https://gear\.noginogi\.sbs/downloads/[^"\s]+\.zip)"', (output/'downloads/index.html').read_text(encoding='utf-8'))
    redirects = ['/ /downloads/ 302'] + [url.removeprefix(BASE.rstrip('/'))+' '+url+' 302' for url in sorted(set(links))]
    (output/'_redirects').write_text('\n'.join(redirects)+'\n',encoding='utf-8')
    (output/'_headers').write_text('/*\n  X-Content-Type-Options: nosniff\n  Referrer-Policy: strict-origin-when-cross-origin\n/downloads/\n  Cache-Control: public, max-age=300\n',encoding='utf-8')
    archive = output.parent / 'netlify-download-mirror.zip'
    with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
        for name in sorted(allowed):
            z.write(output/name,name)
    print(f'Link-only mirror: {output.resolve()} ({len(allowed)} files)')
    print(f'Upload archive: {archive.resolve()} ({archive.stat().st_size} bytes)')


if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--output',type=Path,default=Path('artifacts/netlify-download-mirror'))
    build(parser.parse_args().output)
