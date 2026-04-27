# services/tts/services/tts_engine.py
"""
TTS Engine abstraction layer.

CURRENT:  EdgeTTSEngine  — Microsoft Edge TTS via the `edge-tts` library.
                           Free, no API key, CPU-only, ~2–8s per 30s script.
                           Supports Arabic (Gulf dialect), French, English.

FUTURE:   ChatterboxEngine — Chatterbox-Turbo on NVIDIA GPU.
                           ~200ms per 30s script.
                           Higher voice quality, voice cloning support.

Swap procedure (Sprint 7):
  1. Uncomment ChatterboxEngine below
  2. In main.py, change:  engine = EdgeTTSEngine()
                   to:    engine = ChatterboxEngine()
  3. Add CHATTERBOX_MODEL_PATH to docker-compose.yml
  4. Nothing else changes — same API, same routes, same frontend.
"""

import asyncio
import io
import logging
import re
from abc import ABC, abstractmethod

import edge_tts

logger = logging.getLogger(__name__)


# ── Abstract base class ───────────────────────────────────────────────────────

class TTSEngine(ABC):
    """
    Interface that all TTS engines must implement.
    Keeping this interface stable is what makes the swap cost-free.
    """

    @abstractmethod
    async def synthesize(
        self,
        text: str,
        language: str,
        voice_id: str,
        speed: float = 1.0,
    ) -> tuple[bytes, float]:
        """
        Convert text to audio.

        Returns:
            (audio_bytes, duration_seconds)
            audio_bytes: raw MP3 data
            duration_seconds: actual audio length
        """
        ...

    @abstractmethod
    def default_voice(self, language: str) -> str:
        """Return the default voice ID for a language."""
        ...

    @abstractmethod
    def list_voices(self) -> dict[str, list[dict]]:
        """Return all voices grouped by language code."""
        ...


# ── Edge TTS Engine (current) ─────────────────────────────────────────────────

class EdgeTTSEngine(TTSEngine):
    """
    Microsoft Edge TTS via the `edge-tts` Python library.

    This library calls Microsoft's Text-to-Speech service (the same engine
    used in Edge browser read-aloud) over HTTPS. It's free, requires no API
    key, and produces broadcast-quality audio.

    Supported languages: 400+ voices in 100+ languages.
    Latency: typically 2–8 seconds for a 30-second script on CPU.

    Voice selection for Vocalize:
    ─────────────────────────────
    Arabic (ar):
      ar-SA-ZariyahNeural — Saudi female, clear MSA
      ar-SA-HamedNeural   — Saudi male, clear MSA
      ar-EG-SalmaNeural   — Egyptian female
      ar-EG-ShakirNeural  — Egyptian male

    French (fr):
      fr-FR-DeniseNeural  — French female, natural
      fr-FR-HenriNeural   — French male, warm
      fr-CA-SylvieNeural  — Canadian French female

    English (en):
      en-US-AriaNeural    — US female, expressive
      en-US-GuyNeural     — US male, newsreader quality
      en-GB-SoniaNeural   — British female
    """

    # Default voices per language — chosen for broadcast/ad quality
    _DEFAULT_VOICES = {
        "ar": "ar-SA-ZariyahNeural",   # Gulf Arabic, female
        "fr": "fr-FR-DeniseNeural",    # French, female
        "en": "en-US-AriaNeural",      # US English, female
    }

    # Curated voice list exposed to the frontend
    _VOICES = {
        "ar": [
            {"id": "ar-SA-ZariyahNeural", "name": "Zariyah (Saudi, Female)", "gender": "Female", "locale": "ar-SA"},
            {"id": "ar-SA-HamedNeural",   "name": "Hamed (Saudi, Male)",    "gender": "Male",   "locale": "ar-SA"},
            {"id": "ar-EG-SalmaNeural",   "name": "Salma (Egyptian, Female)", "gender": "Female", "locale": "ar-EG"},
            {"id": "ar-EG-ShakirNeural",  "name": "Shakir (Egyptian, Male)",  "gender": "Male",   "locale": "ar-EG"},
        ],
        "fr": [
            {"id": "fr-FR-DeniseNeural", "name": "Denise (French, Female)",  "gender": "Female", "locale": "fr-FR"},
            {"id": "fr-FR-HenriNeural",  "name": "Henri (French, Male)",     "gender": "Male",   "locale": "fr-FR"},
            {"id": "fr-CA-SylvieNeural", "name": "Sylvie (Canadian, Female)","gender": "Female", "locale": "fr-CA"},
        ],
        "en": [
            {"id": "en-US-AriaNeural",   "name": "Aria (US, Female)",        "gender": "Female", "locale": "en-US"},
            {"id": "en-US-GuyNeural",    "name": "Guy (US, Male)",           "gender": "Male",   "locale": "en-US"},
            {"id": "en-GB-SoniaNeural",  "name": "Sonia (British, Female)",  "gender": "Female", "locale": "en-GB"},
            {"id": "en-GB-RyanNeural",   "name": "Ryan (British, Male)",     "gender": "Male",   "locale": "en-GB"},
        ],
    }

    def default_voice(self, language: str) -> str:
        return self._DEFAULT_VOICES.get(language, "en-US-AriaNeural")

    def list_voices(self) -> dict:
        return self._VOICES

    async def synthesize(
        self,
        text: str,
        language: str,
        voice_id: str,
        speed: float = 1.0,
    ) -> tuple[bytes, float]:
        """
        Synthesize text using edge-tts.

        edge-tts uses SSML under the hood. We set the rate (speed) via the
        +/-X% format that SSML rate attribute accepts.

        Speed conversion:
            1.0  → +0%    (normal)
            1.2  → +20%   (20% faster)
            0.8  → -20%   (20% slower)
        """
        # Convert float speed to SSML rate string
        rate_percent = int((speed - 1.0) * 100)
        rate_str = f"+{rate_percent}%" if rate_percent >= 0 else f"{rate_percent}%"

        # Clean the text for TTS (remove markdown, excessive punctuation)
        clean_text = self._clean_text(text)

        logger.debug(f"Synthesizing: voice={voice_id}, rate={rate_str}, chars={len(clean_text)}")

        # Collect audio chunks from the streaming edge-tts response
        audio_chunks = []
        communicate = edge_tts.Communicate(clean_text, voice_id, rate=rate_str)

        async for chunk in communicate.stream():
            if chunk["type"] == "audio":
                audio_chunks.append(chunk["data"])

        if not audio_chunks:
            raise RuntimeError(
                f"edge-tts returned no audio for voice={voice_id}. "
                "The voice may be unavailable or the text may be empty."
            )

        audio_bytes = b"".join(audio_chunks)

        # Estimate duration from MP3 bitrate
        # edge-tts outputs ~24kbps MP3 (192kbit/s is the max, 24kbps is typical for speech)
        # Duration = file_size_bytes / (bitrate_kbps * 125)
        # We use 24kbps as a conservative estimate
        estimated_duration = len(audio_bytes) / (24 * 125)

        logger.info(
            f"Synthesized: {len(audio_bytes):,} bytes, "
            f"~{estimated_duration:.1f}s, voice={voice_id}"
        )

        return audio_bytes, estimated_duration

    def _clean_text(self, text: str) -> str:
        """
        Prepare text for TTS synthesis.

        TTS engines work best with clean prose — no markdown formatting,
        no excessive ellipses, and normalized punctuation.
        """
        # Remove markdown bold/italic
        text = re.sub(r'\*{1,3}(.*?)\*{1,3}', r'\1', text)
        text = re.sub(r'_{1,3}(.*?)_{1,3}', r'\1', text)

        # Remove markdown headers
        text = re.sub(r'^#{1,6}\s+', '', text, flags=re.MULTILINE)

        # Remove bullet points and numbered list markers
        text = re.sub(r'^\s*[-*•]\s+', '', text, flags=re.MULTILINE)
        text = re.sub(r'^\s*\d+\.\s+', '', text, flags=re.MULTILINE)

        # Normalize multiple whitespace/newlines to single spaces
        text = re.sub(r'\n+', ' ', text)
        text = re.sub(r' {2,}', ' ', text)

        # Remove URLs
        text = re.sub(r'https?://\S+', '', text)

        return text.strip()


# ── Chatterbox Engine (placeholder for Sprint 7) ─────────────────────────────

# class ChatterboxEngine(TTSEngine):
#     """
#     Chatterbox-Turbo GPU engine — Sprint 7.
#
#     Requirements:
#       - NVIDIA GPU with CUDA 11.8+
#       - 4GB+ VRAM
#       - CHATTERBOX_MODEL_PATH env var pointing to model weights
#
#     To enable:
#       1. pip install chatterbox-tts torch torchaudio
#       2. Download model: python -c "from chatterbox.tts import ChatterboxTTS; ChatterboxTTS.from_pretrained('resemble-ai/chatterbox')"
#       3. Set CHATTERBOX_MODEL_PATH in .env
#       4. In main.py: engine = ChatterboxEngine()
#     """
#
#     def __init__(self, model_path: str = None):
#         import torch
#         from chatterbox.tts import ChatterboxTTS
#
#         device = "cuda" if torch.cuda.is_available() else "cpu"
#         logger.info(f"Loading Chatterbox on {device}")
#         self.model = ChatterboxTTS.from_pretrained(
#             model_path or "resemble-ai/chatterbox",
#             device=device
#         )
#         self.device = device
#
#     def default_voice(self, language: str) -> str:
#         return "default"
#
#     def list_voices(self) -> dict:
#         return {"en": [...], "ar": [...], "fr": [...]}
#
#     async def synthesize(self, text, language, voice_id, speed=1.0):
#         import soundfile as sf
#         import io
#         wav = self.model.generate(text, speed=speed)
#         buf = io.BytesIO()
#         sf.write(buf, wav, self.model.sr, format='mp3')
#         return buf.getvalue(), len(wav) / self.model.sr