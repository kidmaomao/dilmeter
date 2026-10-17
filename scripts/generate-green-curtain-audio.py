"""Regenerate the shared built-in cue: python -m pip install edge-tts==7.3.1."""

import asyncio
from pathlib import Path
import tempfile

import edge_tts


async def main() -> None:
    root = Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix="dilmeter-green-curtain-") as temporary:
        output = Path(temporary) / "speech.mp3"
        await edge_tts.Communicate(
            text="绿幕即将出现", voice="zh-CN-XiaoxiaoNeural", rate="+0%",
        ).save(str(output))
        data = output.read_bytes()
        if len(data) < 1024:
            raise RuntimeError("Online service returned incomplete audio")
        for variant in ("cn-rt", "ot"):
            destination = root / variant / "front/public/audio/boss-miel-green-curtain-xiaoxiao.mp3"
            destination.write_bytes(data)
            print(destination)


if __name__ == "__main__":
    asyncio.run(main())
