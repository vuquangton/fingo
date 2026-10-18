import pymupdf, sys, os
src = sys.argv[1]
outdir = sys.argv[2]
os.makedirs(outdir, exist_ok=True)
d = pymupdf.open(src)
for arg in sys.argv[3:]:
    if "-" in arg:
        a, b = arg.split("-"); rng = range(int(a), int(b) + 1)
    else:
        rng = [int(arg)]
    for n in rng:
        p = d.load_page(n - 1)
        zoom = 200 / 72
        pix = p.get_pixmap(matrix=pymupdf.Matrix(zoom, zoom))
        fn = os.path.join(outdir, f"p{n:03d}.png")
        pix.save(fn)
        print(fn, pix.width, pix.height)
