# services/tts/Dockerfile
# ─────────────────────────────────────────────────────────────────────────────
# TTS Service — edge-tts CPU build
#
# Intentionally lightweight: Python 3.11-slim + edge-tts + MinIO SDK + Redis.
# No heavy ML dependencies (PyTorch, transformers) — edge-tts calls
# Microsoft's servers over HTTPS, so synthesis runs anywhere without GPU.
#
# When upgrading to Chatterbox-Turbo:
#   FROM python:3.11-slim  →  FROM nvidia/cuda:11.8-cudnn8-runtime-ubuntu22.04
#   Add:  RUN pip install torch torchaudio chatterbox-tts
# ─────────────────────────────────────────────────────────────────────────────
FROM python:3.11-slim

WORKDIR /app

# System dependencies for audio processing
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for Docker layer caching
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application source
COPY . .

# Non-root user
RUN useradd -m -u 1000 ttsuser
USER ttsuser

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]