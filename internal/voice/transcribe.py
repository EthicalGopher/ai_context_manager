import sys
import wave

import numpy as np
from faster_whisper import WhisperModel

audio, model = sys.argv[1], sys.argv[2]
lang = sys.argv[3] if len(sys.argv) > 3 and sys.argv[3] not in ("", "auto") else None


def load(path):
    # AICW records 16 kHz mono 16-bit WAV. Read it directly so we do not depend on
    # PyAV (some PyAV/faster-whisper version pairs are incompatible).
    try:
        with wave.open(path, "rb") as w:
            if w.getframerate() == 16000 and w.getnchannels() == 1 and w.getsampwidth() == 2:
                return np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16).astype(np.float32) / 32768.0
    except Exception:
        pass
    return path  # other formats: let faster-whisper decode them


m = WhisperModel(model, device="cpu", compute_type="int8")
segments, _ = m.transcribe(load(audio), language=lang, vad_filter=True, beam_size=5)
print(" ".join(s.text.strip() for s in segments))
