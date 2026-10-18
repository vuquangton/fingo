import pypdf, sys, os
for path in sys.argv[1:]:
    r = pypdf.PdfReader(path)
    n = len(r.pages)
    tot = 0
    texts = []
    for i in range(n):
        t = r.pages[i].extract_text() or ""
        tot += len(t)
        texts.append(f"\n===== PAGE {i+1} =====\n" + t)
    print(f"### {os.path.basename(path)}: pages={n} chars={tot}")
    if tot:
        out = os.path.splitext(path)[0] + ".txt"
        open(out, "w", encoding="utf-8").write("\n".join(texts))
        print("   wrote", out)
