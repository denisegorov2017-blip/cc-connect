"""CommandSTT adapter using an existing Hermes local STT installation.

Audio arrives on stdin; stdout is only {"text": "..."}. No dependency installs,
model downloads, HTTP listener, credentials, or cloud-provider fallback.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import sys
import tempfile
import time

MAX_AUDIO_BYTES = 25 * 1024 * 1024
FORMATS = {"mp3", "mp4", "m4a", "wav", "webm", "ogg", "opus", "mpeg", "mpga"}


def recognize(args: argparse.Namespace, audio: bytes, audio_format: str) -> dict[str, object]:
    model_dir = args.model_dir.resolve()
    if not (model_dir / "model.bin").is_file():
        raise ValueError("The specified local model is not cached")
    if audio_format not in FORMATS or not 0 < len(audio) <= MAX_AUDIO_BYTES:
        raise ValueError("Invalid audio format or size")
    # These overrides apply only to this subprocess, never the Hermes gateway.
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"
    os.environ["OMP_NUM_THREADS"] = "2"
    if args.hermes_home is not None:
        os.environ["HERMES_HOME"] = str(args.hermes_home.resolve())
    sys.path.insert(0, str(args.hermes_dir.resolve()))
    from tools.transcription_tools import transcribe_audio_local_fallback

    digest = hashlib.sha256(audio).hexdigest()
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="cc-connect-stt-") as directory:
        path = pathlib.Path(directory) / ("audio." + audio_format)
        path.write_bytes(audio)
        result = transcribe_audio_local_fallback(str(path), model=str(model_dir))
    if result.get("success") is not True or not isinstance(result.get("transcript"), str):
        raise ValueError("Hermes local recognition failed")
    text = result["transcript"].strip()
    evidence: dict[str, object] = {
        "text": text,
        "provider": "Hermes local fallback",
        "audio_sha256": digest,
        "audio_bytes": len(audio),
        "format": audio_format,
        "elapsed_seconds": time.monotonic() - started,
    }
    if args.evidence_dir is not None:
        root = args.evidence_dir.resolve()
        root.mkdir(parents=True, exist_ok=True)
        path = root / (digest + "." + audio_format)
        if path.exists() and hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise ValueError("Existing evidence file identity mismatch")
        path.write_bytes(audio)
        (root / (digest + ".json")).write_text(json.dumps(evidence, ensure_ascii=False), encoding="utf-8")
    return evidence


def main() -> int:
    sys.stdout.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hermes-dir", type=pathlib.Path, required=True)
    parser.add_argument("--model-dir", type=pathlib.Path, required=True)
    parser.add_argument("--hermes-home", type=pathlib.Path)
    parser.add_argument("--evidence-dir", type=pathlib.Path)
    args = parser.parse_args()
    audio = sys.stdin.buffer.read(MAX_AUDIO_BYTES + 1)
    audio_format = os.environ.get("CC_CONNECT_STT_FORMAT", "").lower()
    try:
        result = recognize(args, audio, audio_format)
    except Exception:
        # Do not reflect provider diagnostics, transcript fragments, or input paths.
        print("Local speech recognition failed", file=sys.stderr)
        return 1
    print(json.dumps({"text": result["text"]}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
