"""Generate the two redistributable Buff alert sounds used by the test build."""

from __future__ import annotations

import argparse
import asyncio
import math
import struct
import wave
from pathlib import Path


def synthesize_electronic(path: Path) -> None:
    sample_rate = 44_100
    duration = 1.5
    notes = (
        (0.00, 0.30, 659.25),
        (0.26, 0.32, 783.99),
        (0.54, 0.36, 987.77),
        (0.86, 0.58, 1318.51),
    )
    frames: list[bytes] = []
    for index in range(int(sample_rate * duration)):
        time = index / sample_rate
        value = 0.0
        for start, length, frequency in notes:
            local = time - start
            if not 0 <= local < length:
                continue
            attack = min(1.0, local / 0.018)
            release = min(1.0, (length - local) / 0.12)
            envelope = attack * release * math.exp(-local * 1.25)
            value += envelope * (
                math.sin(2 * math.pi * frequency * local)
                + 0.30 * math.sin(2 * math.pi * frequency * 2 * local)
                + 0.12 * math.sin(2 * math.pi * frequency * 3 * local)
            )
        shimmer = math.sin(2 * math.pi * 7.5 * time) * 0.04
        sample = max(-1.0, min(1.0, value * 0.32 + shimmer * max(0, 1 - time / duration)))
        frames.append(struct.pack("<h", int(sample * 32767)))

    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as output:
        output.setnchannels(1)
        output.setsampwidth(2)
        output.setframerate(sample_rate)
        output.writeframes(b"".join(frames))


async def synthesize_voice(path: Path) -> None:
    import edge_tts

    path.parent.mkdir(parents=True, exist_ok=True)
    communicator = edge_tts.Communicate(
        text="音乐要结束了",
        voice="zh-CN-XiaoxiaoNeural",
        rate="+8%",
        volume="+0%",
        pitch="+0Hz",
    )
    await communicator.save(str(path))


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    parser.add_argument("--skip-voice", action="store_true")
    args = parser.parse_args()
    synthesize_electronic(args.output / "buff-ending-electronic.wav")
    if not args.skip_voice:
        await synthesize_voice(args.output / "buff-ending-xiaoxiao.mp3")


if __name__ == "__main__":
    asyncio.run(main())
