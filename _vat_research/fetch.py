import sys, re, urllib.request, gzip, io, os
from lxml import html as LH

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36"

def get(url):
    req = urllib.request.Request(url, headers={
        "User-Agent": UA,
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Accept-Language": "vi,en;q=0.9",
    })
    with urllib.request.urlopen(req, timeout=90) as r:
        data = r.read()
        if r.headers.get("Content-Encoding") == "gzip":
            data = gzip.decompress(data)
        return data

def text_of(url, drop_tags=("script","style","nav","header","footer","noscript")):
    raw = get(url)
    doc = LH.fromstring(raw)
    for t in drop_tags:
        for e in doc.iter(t):
            e.drop_tree()
    txt = doc.text_content()
    txt = re.sub(r"[ \t\r\f\v]+", " ", txt)
    txt = re.sub(r"\n\s*\n+", "\n", txt)
    return txt

if __name__ == "__main__":
    url = sys.argv[1]
    out = sys.argv[2] if len(sys.argv) > 2 else None
    try:
        t = text_of(url)
    except Exception as e:
        print("ERR", type(e).__name__, e)
        sys.exit(1)
    if out:
        with open(out, "w", encoding="utf-8") as f:
            f.write(t)
        print("WROTE", out, len(t))
    else:
        print(t)
