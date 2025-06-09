#!/usr/bin/env python3
import argparse, random, string
from PIL import Image, ImageDraw, ImageFont, ImageFilter
from pathlib import Path


def gnar_captcha(text: str, w: int = 180, h: int = 70):
    """Вернёт (text, Image) с «уродской» капчей."""
    img = Image.new("RGB", (w, h), (255, 255, 255))
    d   = ImageDraw.Draw(img)
    from pathlib import Path
    FONT_PATH = Path(__file__).with_name("arial.ttf")
    f = ImageFont.truetype(FONT_PATH, 48)

    # шум
    for _ in range(800):
        d.point((random.randint(0, w), random.randint(0, h)),
                fill=(random.randint(0, 255),) * 3)

    # символы
    x = 35
    for ch in text:
        ang = random.uniform(-40, 40)
        y   = random.randint(-10, 10)
        col = tuple(random.randint(0, 120) for _ in range(3))
        ch_img = Image.new("RGBA", (60, 60), (0, 0, 0, 0))
        ImageDraw.Draw(ch_img).text((0, 0), ch, font=f, fill=col)
        ch_img = ch_img.rotate(ang, expand=True)
        img.paste(ch_img, (x, y), ch_img)
        x += 30

    # линии
    for _ in range(5):
        d.line([(random.randint(0, w), random.randint(0, h)),
                (random.randint(0, w), random.randint(0, h))],
               width=4, fill=(0, 0, 0))

    # размытие + пикселизация
    img = img.filter(ImageFilter.GaussianBlur(1.3)) \
             .resize((w // 2, h // 2)).resize((w, h), Image.NEAREST)
    return text, img

def main():
    parser = argparse.ArgumentParser(
        description="Генератор шумной капчи в PNG.")
    parser.add_argument("-o", "--outfile", default="captcha.png",
                        help="Имя выходного файла (PNG)")
    parser.add_argument("-t", "--text",
                        help="Текст капчи (если не задан — генерируется)")
    parser.add_argument("--width",  type=int, default=360, help="Ширина")
    parser.add_argument("--height", type=int, default=60,  help="Высота")
    args = parser.parse_args()

    # если текста нет — генерируем
    alphabet = string.ascii_uppercase + string.digits
    text = args.text or ''.join(random.choices(alphabet, k=5))

    _, img = gnar_captcha(text, args.width, args.height)
    img.save(args.outfile)

if __name__ == "__main__":
    main()
