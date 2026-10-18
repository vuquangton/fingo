"""Minimal PDF text extractor for Vietnamese legal PDFs (FlateDecode + ToUnicode CMaps)."""
import re, sys, zlib

def load_objects(data):
    objs = {}
    for m in re.finditer(rb'(\d+)\s+(\d+)\s+obj\b', data):
        num = int(m.group(1))
        start = m.end()
        e = data.find(b'endobj', start)
        objs[num] = data[start:e if e != -1 else len(data)]
    return objs

def get_stream(body):
    m = re.search(rb'stream\r?\n', body)
    if not m:
        return None
    s = m.end()
    e = body.find(b'endstream', s)
    raw = body[s:e]
    if b'/FlateDecode' in body[:m.start()]:
        try:
            raw = zlib.decompress(raw)
        except Exception:
            try:
                raw = zlib.decompressobj().decompress(raw)
            except Exception:
                return None
    return raw

def expand_objstm(objs, data):
    """Extract compressed objects from /ObjStm streams into objs (as raw bytes)."""
    for num, body in list(objs.items()):
        if b'/ObjStm' not in body:
            continue
        st = get_stream(body)
        if not st:
            continue
        mN = re.search(rb'/N\s+(\d+)', body)
        mF = re.search(rb'/First\s+(\d+)', body)
        if not (mN and mF):
            continue
        N, First = int(mN.group(1)), int(mF.group(1))
        header = st[:First].split()
        try:
            pairs = [(int(header[i]), int(header[i + 1])) for i in range(0, 2 * N, 2)]
        except Exception:
            continue
        for i, (onum, off) in enumerate(pairs):
            end = pairs[i + 1][1] + First if i + 1 < len(pairs) else len(st)
            objs.setdefault(onum, st[First + off:end])
    return objs

def parse_tounicode(cmap_bytes):
    """Return dict code->unicode string."""
    mp = {}
    txt = cmap_bytes
    for blk in re.findall(rb'beginbfchar(.*?)endbfchar', txt, re.S):
        for a, b in re.findall(rb'<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>', blk):
            try:
                mp[int(a, 16)] = bytes.fromhex(b.decode()).decode('utf-16-be', 'replace')
            except Exception:
                pass
    for blk in re.findall(rb'beginbfrange(.*?)endbfrange', txt, re.S):
        for a, b, c in re.findall(rb'<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>', blk):
            lo, hi = int(a, 16), int(b, 16)
            base = bytes.fromhex(c.decode())
            for k in range(lo, min(hi, lo + 65535) + 1):
                try:
                    mv = (int.from_bytes(base, 'big') + (k - lo)).to_bytes(len(base), 'big')
                    mp[k] = mv.decode('utf-16-be', 'replace')
                except Exception:
                    pass
        for a, b, arr in re.findall(rb'<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>\s*\[(.*?)\]', blk, re.S):
            lo = int(a, 16)
            vals = re.findall(rb'<([0-9A-Fa-f]+)>', arr)
            for i, v in enumerate(vals):
                try:
                    mp[lo + i] = bytes.fromhex(v.decode()).decode('utf-16-be', 'replace')
                except Exception:
                    pass
    return mp

def literal_to_bytes(s):
    out = bytearray()
    i = 0
    while i < len(s):
        ch = s[i]
        if ch == 0x5C and i + 1 < len(s):  # backslash
            nxt = s[i + 1]
            mp = {0x6E: 10, 0x72: 13, 0x74: 9, 0x62: 8, 0x66: 12}
            if nxt in mp:
                out.append(mp[nxt]); i += 2; continue
            if 0x30 <= nxt <= 0x37:
                j = i + 1; oct_s = b''
                while j < len(s) and len(oct_s) < 3 and 0x30 <= s[j] <= 0x37:
                    oct_s += bytes([s[j]]); j += 1
                out.append(int(oct_s, 8) & 0xFF); i = j; continue
            out.append(nxt); i += 2; continue
        out.append(ch); i += 1
    return bytes(out)

def decode_with_font(raw, font):
    """font: dict with 'map' and 'two_byte'."""
    mp = font.get('map') or {}
    two = font.get('two_byte')
    res = []
    if two is None:
        two = len(mp) > 0 and max(mp) > 255
    if two:
        for i in range(0, len(raw) - 1, 2):
            code = (raw[i] << 8) | raw[i + 1]
            if code in mp:
                res.append(mp[code])
            else:
                res.append('')
    else:
        for b in raw:
            if b in mp:
                res.append(mp[b])
            else:
                res.append(chr(b) if 32 <= b < 127 else '')
    return ''.join(res)

TOKEN = re.compile(rb'''
    (?P<hexstr><[0-9A-Fa-f\s]*>)
  | (?P<lit>\((?:\\.|[^\\()])*\))
  | (?P<name>/[^\s/<>\[\]()]+)
  | (?P<num>[-+]?\d*\.?\d+)
  | (?P<arr>[\[\]])
  | (?P<op>[A-Za-z'"*]+)
''', re.X)

def content_text(stream, fonts):
    out = []
    cur = {'map': {}, 'two_byte': None}
    stack = []
    lines = 0
    for m in TOKEN.finditer(stream):
        kind = m.lastgroup
        if kind == 'num':
            stack.append(float(m.group()))
        elif kind == 'hexstr':
            h = re.sub(rb'\s', b'', m.group()[1:-1])
            if len(h) % 2:
                h += b'0'
            try:
                stack.append(bytes.fromhex(h.decode()))
            except Exception:
                stack.append(b'')
        elif kind == 'lit':
            stack.append(literal_to_bytes(m.group()[1:-1]))
        elif kind == 'name':
            stack.append(m.group().decode('latin-1'))
        elif kind == 'op':
            op = m.group().decode('latin-1')
            if op == 'Tf':
                fname = None
                for v in reversed(stack):
                    if isinstance(v, str) and v.startswith('/'):
                        fname = v[1:]; break
                cur = fonts.get(fname, {'map': {}, 'two_byte': False})
            elif op in ('Tj', "'", '"'):
                for v in reversed(stack):
                    if isinstance(v, bytes):
                        out.append(decode_with_font(v, cur))
                        if op in ("'", '"'):
                            out.append('\n')
                        break
            elif op == 'TJ':
                for v in stack:
                    if isinstance(v, list):
                        for item in v:
                            if isinstance(item, bytes):
                                out.append(decode_with_font(item, cur))
                # array can also appear as bare stack items
                for v in stack:
                    if isinstance(v, bytes):
                        out.append(decode_with_font(v, cur))
            elif op == 'Td' or op == 'TD' or op == 'T*':
                out.append('\n')
            stack = []
    return ''.join(out)

def build_fonts(objs):
    fonts = {}
    # map font resource name -> font object number is per-page; approximate globally by /BaseFont order
    # Instead: for each /Font object build a name keyed by its object number too.
    by_obj = {}
    for num, body in objs.items():
        if b'/Font' not in body and b'/ToUnicode' not in body:
            continue
        if b'/Type' in body and b'/Font' in body:
            tu = re.search(rb'/ToUnicode\s+(\d+)\s+\d+\s+R', body)
            mp = {}
            if tu:
                st = get_stream(objs.get(int(tu.group(1)), b''))
                if st:
                    mp = parse_tounicode(st)
            sub = re.search(rb'/Subtype\s*/(\w+)', body)
            is_t0 = bool(sub and sub.group(1) == b'Type0')
            by_obj[num] = {'map': mp, 'two_byte': is_t0 or (len(mp) > 0 and max(mp) > 255)}
    return by_obj

def extract(path):
    data = open(path, 'rb').read()
    objs = load_objects(data)
    objs = expand_objstm(objs, data)
    by_obj = build_fonts(objs)

    # Build page font-name -> font map by scanning /Font << /F1 12 0 R ... >> blocks
    name_to_obj = {}
    for num, body in objs.items():
        for fm in re.finditer(rb'/Font\s*<<(.*?)>>', body, re.S):
            for nm, on in re.findall(rb'/([A-Za-z0-9_.]+)\s+(\d+)\s+\d+\s+R', fm.group(1)):
                name_to_obj.setdefault(nm.decode(), int(on))
    fonts = {}
    for nm, on in name_to_obj.items():
        if on in by_obj:
            fonts[nm] = by_obj[on]
    if not fonts:
        # fallback: give every name the concatenated map of all fonts
        merged = {}
        for f in by_obj.values():
            merged.update(f['map'])
        fonts = {'': {'map': merged, 'two_byte': None}}

    chunks = []
    for num, body in objs.items():
        st = get_stream(body)
        if not st:
            continue
        if b'BT' in st and (b'Tj' in st or b'TJ' in st):
            chunks.append((num, content_text(st, fonts)))
    return '\n'.join(c for _, c in chunks)

if __name__ == '__main__':
    t = extract(sys.argv[1])
    t = re.sub(r'[ \t]+', ' ', t)
    t = re.sub(r'\n\s*\n+', '\n', t)
    if len(sys.argv) > 2:
        open(sys.argv[2], 'w', encoding='utf-8').write(t)
        print('WROTE', sys.argv[2], len(t))
    else:
        sys.stdout.reconfigure(encoding='utf-8')
        print(t)
