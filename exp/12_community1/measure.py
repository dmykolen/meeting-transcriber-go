"""Experiment 12 — pyannote community-1, against the nine sherpa configurations.

    go run ./exp/12_community1/prep.go 74 <dir>
    ../.venv/bin/python exp/12_community1/measure.py <dir>/74-mix.wav <dir>/74-tap.wav

Python, not Go, and that is the point: pyannote has no ONNX export of
community-1 and no Go binding, so this is the only way to know what the app is
giving up by shipping sherpa-onnx with pyannote segmentation 3.0. The root
project already depends on pyannote 4.x and has community-1 cached, so nothing
is installed for this.

Scored exactly as experiment 11 scored sherpa — the owner's own criterion:

    the two Сергій windows must agree with each other, and
    must not share a label with the Kovalenko window.

If community-1 clears that on the mixed audio, it is worth the trouble of
getting it into the app somehow. If it only clears it on the tap, then the
channel mattered more than the model and the app is already where it needs to
be.
"""

import os
import sys
import time
from collections import defaultdict
from pathlib import Path

TRUTH = Path(__file__).resolve().parents[1] / "truth" / "74.txt"
OUT = Path(__file__).resolve().parents[1] / "out" / "G-community1-74.txt"


def windows(path):
    out = []
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        bits = line.split(None, 2)
        out.append((float(bits[0]), float(bits[1]), bits[2]))
    return sorted(out)


def mmss(s):
    return f"{int(s) // 60}:{int(s) % 60:02d}"


def score(say, truth, spans):
    """spans: list of (start, end, label)."""
    dominant = []
    for start, end, who in truth:
        held = defaultdict(float)
        for a, b, label in spans:
            overlap = min(b, end) - max(a, start)
            if overlap > 0:
                held[label] += overlap
        if not held:
            dominant.append(None)
            say(f"   {mmss(start)}–{mmss(end)}  {who:<18} → nothing there\n")
            continue
        top = max(held, key=held.get)
        dominant.append(top)
        say(f"   {mmss(start)}–{mmss(end)}  {who:<18} → {top:<12} holds {100 * held[top] / sum(held.values()):.0f}%\n")

    same = [d for (_, _, w), d in zip(truth, dominant) if w.lower() == "сергій"]
    other = [d for (_, _, w), d in zip(truth, dominant) if w.lower() != "сергій"]
    agree = len(same) > 1 and len(set(same)) == 1
    apart = not (set(same) & set(other))
    say(f"   the stranger is one speaker      {'yes' if agree else 'NO'}\n")
    say(f"   and not the enrolled one         {'yes' if apart else 'NO'}\n\n")


def read(path):
    """One of our 16 kHz mono WAVs, as pyannote wants it."""
    import wave

    import numpy as np
    import torch

    with wave.open(path) as f:
        rate, frames = f.getframerate(), f.getnframes()
        raw = np.frombuffer(f.readframes(frames), dtype=np.int16)
    audio = torch.from_numpy(raw.astype("float32") / 32768).unsqueeze(0)
    return {"waveform": audio, "sample_rate": rate}


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(2)

    # The token lives in the root project's .env, which is where the Python
    # side of this repository already keeps it.
    root = Path(__file__).resolve().parents[3]
    for line in (root / ".env").read_text().splitlines():
        if line.startswith("MT_HF_TOKEN="):
            os.environ["HF_TOKEN"] = line.split("=", 1)[1].strip()

    import torch
    from pyannote.audio import Pipeline

    OUT.parent.mkdir(exist_ok=True)
    handle = OUT.open("w")

    def say(text):
        print(text, end="")
        handle.write(text)

    say("EXPERIMENT G — pyannote community-1, against the owner's ear\n")
    say("the same two signals experiment 11 measured, and the same question\n\n")

    truth = windows(TRUTH)
    for start, end, who in truth:
        say(f"  {mmss(start)}–{mmss(end)}  {who}\n")
    say("\n")

    began = time.time()
    pipeline = Pipeline.from_pretrained("pyannote/speaker-diarization-community-1", token=os.environ["HF_TOKEN"])
    device = "mps" if torch.backends.mps.is_available() else "cuda" if torch.cuda.is_available() else "cpu"
    pipeline.to(torch.device(device))
    say(f"community-1 loaded on {device} in {time.time() - began:.0f}s\n\n")

    for path in sys.argv[1:]:
        name = Path(path).stem.split("-")[-1]
        began = time.time()
        # A waveform, not a path. pyannote 4 loads files through torchcodec,
        # which needs FFmpeg's dylibs on the rpath and does not find them here.
        # The pipeline takes a tensor directly, so the decoder is skipped.
        result = pipeline(read(path))
        took = time.time() - began

        # pyannote 4 returns both the overlapping annotation and an
        # overlap-free one. The app has to say who said each word, so the
        # exclusive answer is the comparable one.
        spans = [(t.start, t.end, label) for t, _, label in result.exclusive_speaker_diarization.itertracks(yield_label=True)]
        speakers = {label for _, _, label in spans}
        length = max((b for _, b, _ in spans), default=1)

        say(f"── {name}\n")
        say(f"   {len(speakers)} speakers found, {took:.0f}s ({length / took:.0f}x realtime)\n")
        score(say, truth, spans)

    handle.close()
    print(f"\nwritten to {OUT}")


if __name__ == "__main__":
    main()
