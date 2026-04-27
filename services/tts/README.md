# Vocalize TTS Service
FastAPI service for text-to-speech synthesis using Edge TTS.

## Quick Start (Docker - Recommended)

From project root:
```bash
docker compose up -d minio redis tts
```

- TTS: http://localhost:8000/health
- MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
- Redis: localhost:6379

## Dev Direct Run (uvicorn)

1. Start deps:
```bash
docker compose up -d minio redis
```

2. From `services/tts/`:
```bash
uvicorn main:app --reload
```

Uses `.env` to connect to Docker services via `host.docker.internal:9000`.

## API

```
POST /api/tts/generate - enqueue synthesis job
GET /api/tts/status/{job_id} - poll status + audio URL
GET /api/tts/voices - list voices by language
GET /health - service status
```

## Config

- `.env` or Docker env vars
- `MINIO_ENABLED=false` to disable storage
- Works without MinIO (local fallback stub needed for production)

## Troubleshooting

**MinIO connection failed**: Ensure Docker minio running, use Docker Compose.

**No Redis**: `docker compose up redis`

