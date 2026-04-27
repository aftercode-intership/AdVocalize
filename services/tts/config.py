# services/tts/config.py
import os
import logging
from dotenv import load_dotenv

load_dotenv()

logger = logging.getLogger(__name__)


class Config:
    # ── Server ────────────────────────────────────────────────────────────────
    PORT: int = int(os.getenv("PORT", "8000"))
    LOG_LEVEL: str = os.getenv("LOG_LEVEL", "INFO").upper()
    ENVIRONMENT: str = os.getenv("ENVIRONMENT", "development")

    # ── Redis (job queue) ─────────────────────────────────────────────────────
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379")
    # How long job records stay in Redis after completion (7 days)
    JOB_TTL_SECONDS: int = int(os.getenv("JOB_TTL_SECONDS", str(7 * 24 * 3600)))

    # ── MinIO (audio storage) ─────────────────────────────────────────────────


    # ── MinIO (audio storage) ─────────────────────────────────────────────────
    MINIO_ENDPOINT: str = os.getenv("MINIO_ENDPOINT", "")
    MINIO_USER: str = os.getenv("MINIO_USER", "minioadmin")
    MINIO_PASSWORD: str = os.getenv("MINIO_PASSWORD", "minioadmin")
    MINIO_BUCKET_AUDIO: str = os.getenv("MINIO_BUCKET_AUDIO", "vocalize-audio")
    MINIO_USE_SSL: bool = os.getenv("MINIO_USE_SSL", "false").lower() == "true"
    MINIO_ENABLED: bool = os.getenv("MINIO_ENABLED", "true").lower() == "true"
    # Presigned URL expiry (7 days)
    MINIO_PRESIGN_EXPIRY: int = int(os.getenv("MINIO_PRESIGN_EXPIRY", str(7 * 24 * 3600)))

    @classmethod
    def validate(cls):
        if cls.MINIO_ENABLED and not cls.MINIO_ENDPOINT:
            logger.warning("MINIO_ENABLED=true but no MINIO_ENDPOINT set. Storage will be disabled.")
        # Nothing strictly required — all have sensible defaults for dev
        pass

    # ── TTS engine config ─────────────────────────────────────────────────────
    # When CHATTERBOX_MODEL_PATH is set and GPU is available, the service
    # will automatically use ChatterboxEngine instead of EdgeTTSEngine.
    # Leave empty to always use edge-tts (CPU fallback).
    CHATTERBOX_MODEL_PATH: str = os.getenv("CHATTERBOX_MODEL_PATH", "")
