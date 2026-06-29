from PIL import Image, ImageDraw, ImageFont

size = 1024
img = Image.new('RGBA', (size, size), (24, 24, 27, 255))
draw = ImageDraw.Draw(img)

emerald = (16, 185, 129, 255)
emerald_dark = (6, 95, 70, 255)
bg = (39, 39, 42, 255)

corner = 220
draw.rounded_rectangle([80, 80, size-80, size-80], radius=corner, fill=bg)

try:
    font = ImageFont.truetype("C:/Windows/Fonts/arialbd.ttf", 520)
except Exception:
    font = ImageFont.load_default()

text = "A"
bbox = draw.textbbox((0, 0), text, font=font)
text_w = bbox[2] - bbox[0]
text_h = bbox[3] - bbox[1]
x = (size - text_w) // 2 - 10
y = (size - text_h) // 2 - 40

draw.text((x+6, y+6), text, font=font, fill=emerald_dark)
draw.text((x, y), text, font=font, fill=emerald)

cx, cy = 720, 300
for r, c in [(60, emerald_dark), (35, emerald)]:
    draw.ellipse([cx-r, cy-r, cx+r, cy+r], fill=c)

img.save("k:/go_projects/AsyncStarterAgent/web/src-tauri/icons/icon.png")
print("icon.png saved")
