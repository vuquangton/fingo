import pymupdf, sys, os
src = r"D:\accounting\research\official\133 TT-BTC.signed.pdf"
out = r"D:\accounting\research\official\pages"
os.makedirs(out, exist_ok=True)
d = pymupdf.open(src)
for arg in sys.argv[1:]:
    if "-" in arg:
        a, b = arg.split("-"); rng = range(int(a), int(b) + 1)
    else:
        rng = [int(arg)]
    for n in rng:
        p = d.load_page(n - 1)
        zoom = 180 / 72
        pix = p.get_pixmap(matrix=pymupdf.Matrix(zoom, zoom))
        fn = os.path.join(out, f"p{n:03d}.png")
        pix.save(fn)
        print(fn, pix.width, pix.height)
