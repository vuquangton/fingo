import urllib.request, re, sys

def get(u):
    r = urllib.request.Request(u, headers={'User-Agent': 'Mozilla/5.0', 'Accept-Language': 'vi'})
    resp = urllib.request.urlopen(r, timeout=90)
    return resp, resp.read()

u = sys.argv[1]
resp, data = get(u)
print("content-type:", resp.headers.get('Content-Type'), "len:", len(data))
if data[:4] == b'%PDF':
    open(sys.argv[2], 'wb').write(data)
    print("saved PDF ->", sys.argv[2])
else:
    h = data.decode('utf-8', 'replace')
    for m in sorted(set(re.findall(r'(?i)(?:https?://[^"\'\s<>]+|/[^"\'\s<>]*\.(?:pdf|doc|docx|zip))', h))):
        if any(k in m.lower() for k in ('pdf', 'file', 'download', 'tai-ve', 'attach', 'api')):
            print("  ", m)
