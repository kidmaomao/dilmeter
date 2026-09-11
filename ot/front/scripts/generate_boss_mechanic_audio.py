"""Generate the dedicated Lei Neien mechanic timelines.

The files intentionally contain their leading silence.  Playback starts at the
mechanic packet, so every cue remains anchored to the same absolute countdown
time even while Dilmeter is running in the background.
"""

from __future__ import annotations

import argparse
import asyncio
import math
import tempfile
import wave
from pathlib import Path

import numpy as np


SAMPLE_RATE = 44_100


def _tone(
    samples: np.ndarray,
    start: float,
    duration: float,
    frequency: float,
    volume: float,
    *,
    end_frequency: float | None = None,
) -> None:
    first = max(0, round(start * SAMPLE_RATE))
    length = max(1, round(duration * SAMPLE_RATE))
    last = min(len(samples), first + length)
    if first >= last:
        return
    local = np.arange(last - first, dtype=np.float64) / SAMPLE_RATE
    attack = np.minimum(1.0, local / min(0.012, duration / 4))
    release = np.minimum(1.0, (duration - local) / min(0.055, duration / 3))
    envelope = np.maximum(0.0, attack * release)
    if end_frequency is None:
        phase = 2 * math.pi * frequency * local
    else:
        sweep = (end_frequency - frequency) / max(duration, 1e-6)
        phase = 2 * math.pi * (frequency * local + 0.5 * sweep * local**2)
    signal = (
        np.sin(phase)
        + 0.24 * np.sin(phase * 2)
        + 0.08 * np.sin(phase * 3)
    )
    samples[first:last] += signal * envelope * volume


def _accelerating_pulses(
    samples: np.ndarray,
    start: float,
    end: float,
    *,
    initial_interval: float,
    final_interval: float,
) -> None:
    at = start
    while at < end - 0.055:
        progress = min(1.0, max(0.0, (at - start) / max(0.001, end - start)))
        interval = initial_interval + (final_interval - initial_interval) * progress**1.38
        frequency = 610 + 590 * progress
        _tone(samples, at, min(0.095, interval * 0.56), frequency, 0.20 + 0.10 * progress,
              end_frequency=frequency * 1.13)
        at += interval


def _emphasis(samples: np.ndarray, at: float, *, final: bool = False) -> None:
    base = 690 if not final else 780
    _tone(samples, at, 0.16, base, 0.34, end_frequency=base * 1.38)
    _tone(samples, at + 0.16, 0.23, base * 1.26, 0.38, end_frequency=base * 1.75)
    _tone(samples, at + 0.03, 0.36, base / 2, 0.13, end_frequency=base * 0.72)


def _mix_voice(samples: np.ndarray, voice_path: Path, at: float) -> float:
    import miniaudio

    decoded = miniaudio.decode_file(
        str(voice_path),
        output_format=miniaudio.SampleFormat.SIGNED16,
        nchannels=1,
        sample_rate=SAMPLE_RATE,
    )
    voice = np.asarray(decoded.samples, dtype=np.float64) / 32768.0
    # Edge speech has a little encoder padding.  Remove only leading digital
    # silence so the spoken phrase itself begins precisely at 10.0 seconds.
    audible = np.flatnonzero(np.abs(voice) > 0.0025)
    if audible.size:
        voice = voice[max(0, int(audible[0]) - round(0.018 * SAMPLE_RATE)):]
    if voice.size:
        voice *= min(1.0, 0.68 / max(0.01, float(np.max(np.abs(voice)))))
    first = round(at * SAMPLE_RATE)
    last = min(len(samples), first + len(voice))
    samples[first:last] += voice[: last - first]
    return len(voice) / SAMPLE_RATE


def _write_wave(path: Path, samples: np.ndarray) -> None:
    peak = float(np.max(np.abs(samples))) if samples.size else 0.0
    if peak > 0.92:
        samples = samples * (0.92 / peak)
    pcm = np.round(np.clip(samples, -1.0, 1.0) * 32767).astype("<i2")
    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as output:
        output.setnchannels(1)
        output.setsampwidth(2)
        output.setframerate(SAMPLE_RATE)
        output.writeframes(pcm.tobytes())


async def _xiaoxiao_notice(path: Path) -> None:
    import edge_tts

    communicator = edge_tts.Communicate(
        text="注意球",
        voice="zh-CN-XiaoxiaoNeural",
        rate="+12%",
        volume="+0%",
        pitch="+2Hz",
    )
    await communicator.save(str(path))


async def generate(output: Path) -> None:
    with tempfile.TemporaryDirectory(prefix="dilmeter-boss-audio-") as temporary:
        voice_path = Path(temporary) / "notice-orb-xiaoxiao.mp3"
        await _xiaoxiao_notice(voice_path)

        orb = np.zeros(round(20.0 * SAMPLE_RATE), dtype=np.float64)
        voice_duration = _mix_voice(orb, voice_path, 10.0)
        electronic_start = max(10.72, 10.0 + voice_duration - 0.10)
        _accelerating_pulses(
            orb, electronic_start, 13.5,
            initial_interval=0.53, final_interval=0.13,
        )
        _emphasis(orb, 13.5)
        _accelerating_pulses(
            orb, 13.92, 18.5,
            initial_interval=0.57, final_interval=0.12,
        )
        _emphasis(orb, 18.5, final=True)
        _write_wave(output / "boss-miel-orb.wav", orb)

        sword = np.zeros(round(5.0 * SAMPLE_RATE), dtype=np.float64)
        _accelerating_pulses(
            sword, 3.5, 4.2,
            initial_interval=0.23, final_interval=0.075,
        )
        _emphasis(sword, 4.2, final=True)
        _write_wave(output / "boss-miel-laser.wav", sword)


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    await generate(args.output)


if __name__ == "__main__":
    asyncio.run(main())
