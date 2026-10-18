import pymupdf, os
src = r"D:\accounting\research\official\133 TT-BTC.signed.pdf"
out = r"D:\accounting\research\official\pages"
os.makedirs(out, exist_ok=True)
d = pymupdf.open(src)
print("pages:", d.page_count)
# low-res contact sheets to locate the appendix
import math
cols, rows = 8, 5
thumb_w = 190
per_sheet = cols * rows
from PIL import Image
sheets = []
for sheet_i, start in enumerate(range(0, d.page_count, per_sheet)):
    imgs = []
    for i in range(start, min(start + per_sheet, d.page_count)):
        p = d.load_page(i)
        zoom = thumb_w / p.rect.width
        pix = p.get_pixmap(matrix=pymupdf.Matrix(zoom, zoom))
        imgs.append((i + 1, Image.frombytes("RGB", (pix.width, pix.height), pix.samples)))
    if not imgs:
        break
    w, h = imgs[0][1].size
    sheet = Image.new("RGB", (cols * (w + 6), rows * (h + 18)), "white")
    from PIL import ImageDraw
    dr = ImageDraw.Draw(sheet)
    for k, (num, im) in enumerate(imgs):
        r, c = divmod(k, cols)
        x, y = c * (w + 6), r * (h + 18)
        sheet.paste(im, (x, y))
        dr.text((x + 3, y + h + 3), str(num), fill="black")
    fn = os.path.join(out, f"sheet_{sheet_i+1:02d}_{start+1}-{imgs[-1][0]}.png")
    sheet.save(fn)
    print(fn, sheet.size)
