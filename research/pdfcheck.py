import pypdf, sys, os
path = r"D:\accounting\research\official\133 TT-BTC.signed.pdf"
r = pypdf.PdfReader(path)
n = len(r.pages)
print("pages:", n)
tot = 0
for i in range(min(n, 12)):
    t = r.pages[i].extract_text() or ""
    tot += len(t)
    print(i, len(t), repr(t[:200]))
print("total chars first 12 pages:", tot)
if tot > 200:
    out = []
    for i in range(n):
        out.append(f"\n===== PAGE {i+1} =====\n" + (r.pages[i].extract_text() or ""))
    open(r"D:\accounting\research\official\tt133_official.txt", "w", encoding="utf-8").write("\n".join(out))
    print("WROTE text file")
