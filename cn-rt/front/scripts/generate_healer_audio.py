"""Create an original, gentle 'angel descent' chime (no sampled game audio)."""
import math
import struct
import wave
from pathlib import Path

destination = Path(__file__).resolve().parents[1] / 'public/audio/healer-angel.wav'
rate, duration = 44100, 2.4
notes = [(0, 1046.50), (.17, 1318.51), (.34, 1567.98), (.56, 2093.00)]
samples = []
for index in range(int(rate * duration)):
    t = index / rate
    value = 0.0
    for start, frequency in notes:
        local = t - start
        if local < 0:
            continue
        envelope = min(1, local / .012) * math.exp(-local * 2.8)
        value += envelope * (math.sin(2 * math.pi * frequency * local)
                             + .19 * math.sin(2 * math.pi * frequency * 2.003 * local))
    # Soft sustained major harmony underneath the ascending bell notes.
    pad = min(1, t / .42) * max(0, 1 - t / duration) ** 2
    value += .22 * pad * sum(math.sin(2 * math.pi * f * t) for f in (523.25, 659.25, 783.99))
    value *= min(1, (duration-t) / .15)
    samples.append(value)
peak = max(abs(sample) for sample in samples)
destination.parent.mkdir(parents=True, exist_ok=True)
with wave.open(str(destination), 'wb') as output:
    output.setnchannels(1)
    output.setsampwidth(2)
    output.setframerate(rate)
    output.writeframes(b''.join(struct.pack('<h', round(sample / peak * 24000)) for sample in samples))
print(destination)
