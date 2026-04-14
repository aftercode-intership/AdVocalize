# services/script-gen/config.py
import os
from dotenv import load_dotenv

load_dotenv()


class Config:
    # ── GLM API ──────────────────────────────────────────────────────────────
    GLM_API_KEY: str = os.getenv("GLM_API_KEY", "")

    # ✅ FIXED: reads from env first, then falls back to the correct model name.
    # The previous hardcoded value was "glm-flash-4.7" (wrong order).
    # ZhipuAI API uses "glm-4-flash" or "glm-4.7-flash" — check your API docs.
    # Your .env has GLM_MODEL=glm-4.7-flash, so that's what will be used.
    GLM_MODEL: str = os.getenv("GLM_MODEL", "glm-4-flash")

    # ✅ FIXED: base URL is now configurable via env
    GLM_BASE_URL: str = os.getenv(
        "GLM_BASE_URL",
        "https://open.bigmodel.cn/api/paas/v4"
    )

    # Full chat completions endpoint (constructed from base URL)
    @classmethod
    def glm_chat_url(cls) -> str:
        return f"{cls.GLM_BASE_URL}/chat/completions"

    # ── Redis ─────────────────────────────────────────────────────────────────
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379")
    CACHE_TTL: int = 300          # seconds — cached scripts expire after 5 min
    CACHE_MAX_VERSIONS: int = 10  # max stored versions per script

    # ── Timeouts ──────────────────────────────────────────────────────────────
    GLM_TIMEOUT: int = 30         # max seconds to wait for GLM API response
    GLM_INFERENCE_TIMEOUT: int = 3  # target (monitoring only, not enforced yet)

    # ── Arabic diacritization ─────────────────────────────────────────────────
    # CAMeL-Lab BERT model for Arabic dialect identification + diacritization
    DIACRITIZER_MODEL: str = os.getenv(
        "DIACRITIZER_MODEL",
        "CAMeL-Lab/bert-base-arabic-camelbert-msa-did-madar"
    )
    DIACRITIZER_CONFIDENCE_THRESHOLD: float = float(
        os.getenv("DIACRITIZER_CONFIDENCE_THRESHOLD", "0.7")
    )

    # ── Environment ───────────────────────────────────────────────────────────
    LOG_LEVEL: str = os.getenv("LOG_LEVEL", "INFO").upper()
    ENVIRONMENT: str = os.getenv("ENVIRONMENT", "development")
    PORT: int = int(os.getenv("PORT", "8001"))

    # ── Validation ────────────────────────────────────────────────────────────
    @classmethod
    def validate(cls) -> None:
        """Call at startup to fail fast on missing required config."""
        if not cls.GLM_API_KEY:
            raise ValueError(
                "GLM_API_KEY environment variable is required. "
                "Add it to your .env file."
            )