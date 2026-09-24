"""Generate the bundled healer prompts with Microsoft Xiaoxiao Neural."""
import asyncio
import argparse
from pathlib import Path
import edge_tts

async def main():
    output = Path(__file__).resolve().parents[1] / 'public/audio'
    prompts = {
        'healer-health-xiaoxiao.mp3': '队友血量过低，请及时治疗',
        'healer-music-xiaoxiao.mp3': '队友音乐时间到了',
        'healer-buff-xiaoxiao.mp3': '队友增益效果即将结束',
        'healer-death-xiaoxiao.mp3': '队友死亡，增益已消失，请及时补充',
    }
    parser = argparse.ArgumentParser()
    parser.add_argument('--only', choices=list(prompts))
    args = parser.parse_args()
    for filename, text in prompts.items():
        if args.only and filename != args.only:
            continue
        await edge_tts.Communicate(text=text, voice='zh-CN-XiaoxiaoNeural', rate='+0%', volume='+0%', pitch='+0Hz').save(str(output / filename))
        print(filename, flush=True)

if __name__ == '__main__':
    asyncio.run(main())
