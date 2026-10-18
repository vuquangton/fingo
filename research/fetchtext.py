import sys, re, urllib.request, gzip, io, html

def fetch(url):
    req = urllib.request.Request(url, headers={
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36',
        'Accept-Language': 'vi,en;q=0.8',
        'Accept': 'text/html,application/xhtml+xml,*/*',
    })
    with urllib.request.urlopen(req, timeout=60) as r:
        data = r.read()
        if r.headers.get('Content-Encoding') == 'gzip':
            data = gzip.decompress(data)
        return data.decode('utf-8', 'replace')

def text(h):
    h = re.sub(r'(?is)<(script|style|noscript|svg|head)[^>]*>.*?</\1>', ' ', h)
    h = re.sub(r'(?is)<br\s*/?>', '\n', h)
    h = re.sub(r'(?is)</(p|div|li|tr|h[1-6]|td)>', '\n', h)
    h = re.sub(r'(?s)<[^>]+>', ' ', h)
    h = html.unescape(h)
    h = re.sub(r'[ \t\xa0]+', ' ', h)
    h = re.sub(r'\n\s*\n+', '\n', h)
    return h.strip()

for url in sys.argv[1:]:
    try:
        t = text(fetch(url))
        print(f"\n########## {url}  ({len(t)} chars) ##########")
        print(t)
    except Exception as e:
        print(f"\n########## {url} FAILED: {e} ##########")
