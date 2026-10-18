import sys, docx

def extract(path, out):
    d = docx.Document(path)
    lines = []
    for p in d.paragraphs:
        t = p.text.strip()
        if t:
            lines.append(t)
    for ti, tbl in enumerate(d.tables):
        lines.append(f"\n=== TABLE {ti+1} ({len(tbl.rows)}x{len(tbl.columns)}) ===")
        for r in tbl.rows:
            cells = []
            seen = set()
            for c in r.cells:
                if id(c._tc) in seen:
                    continue
                seen.add(id(c._tc))
                cells.append(" ".join(c.text.split()))
            lines.append(" | ".join(cells))
    with open(out, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print(f"{path} -> {out} : {len(lines)} lines")

extract(sys.argv[1], sys.argv[2])
