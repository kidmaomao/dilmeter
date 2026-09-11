from pathlib import Path

from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parent
SOURCE = ROOT / "app-icon.png"


def contain(image: Image.Image, size: int, padding: int = 0) -> Image.Image:
    usable = size - padding * 2
    scaled = image.copy()
    scaled.thumbnail((usable, usable), Image.Resampling.LANCZOS)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    canvas.alpha_composite(scaled, ((size - scaled.width) // 2, (size - scaled.height) // 2))
    return canvas


icon = Image.open(SOURCE).convert("RGBA")
master = contain(icon, 1024, 20)
master.save(ROOT / "app-icon-master.png", optimize=True)

icon_256 = contain(icon, 256, 4)
icon_256.save(ROOT / "app-icon-256.png", optimize=True)
icon_256.save(
    ROOT / "app-icon.ico",
    format="ICO",
    sizes=[(16, 16), (20, 20), (24, 24), (32, 32), (40, 40), (48, 48), (64, 64), (128, 128), (256, 256)],
)

preview = Image.new("RGB", (740, 640), "#202020")
draw = ImageDraw.Draw(preview)
draw.text((24, 18), "DilmeterCN icon size check - dark and light backgrounds", fill="white")
for row, (background, label, text_color) in enumerate(
    (("#2d2d2d", "Dark taskbar", "#dddddd"), ("#f0f0f0", "Light taskbar", "#222222"))
):
    y = 48 + row * 288
    draw.text((24, y), label, fill=text_color)
    x = 24
    for size in (256, 64, 32, 16):
        tile_size = max(size, 80)
        tile = Image.new("RGBA", (tile_size, tile_size), background)
        sample = contain(icon, size, max(1, size // 64))
        tile.alpha_composite(sample, ((tile_size - size) // 2, (tile_size - size) // 2))
        preview.paste(tile.convert("RGB"), (x, y + 24))
        draw.text((x, y + 24 + tile_size + 8), f"{size}x{size}", fill=text_color)
        x += tile_size + 28

preview.save(ROOT / "app-icon-size-preview.png", optimize=True)
