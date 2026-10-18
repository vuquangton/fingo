import urllib.request, re, sys
def get(u):
    r=urllib.request.Request(u, headers={'User-Agent':'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124 Safari/537.36','Accept-Language':'vi'})
    return urllib.request.urlopen(r, timeout=60).read().decode('utf-8','replace')
h=get(sys.argv[1])
out=set()
for m in re.findall(r'https://datafiles\.chinhphu\.vn/[^"\'<> ]+', h):
    if not any(k in m for k in ('theme','Icon','Logo','banner','tienich','csdl')):
        out.add(m)
for m in sorted(out):
    print(m)
