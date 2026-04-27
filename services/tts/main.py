# services/tts/main.py
"""
Vocalize TTS Service — Sprint 5
================================
Converts ad scripts into audio files using Microsoft Edge TTS (edge-tts).

Architecture:
  POST /api/tts/generate   → enqueue job → return job_id immediately
  GET  /api/tts/status/:id → poll job status + audio URL when done
  GET  /api/tts/voices     → list available voices per language

The actual synthesis runs as a FastAPI BackgroundTask so the HTTP response
returns in <100ms even though synthesis itself takes 2-10 seconds.

Swap point: To upgrade to Chatterbox-Turbo GPU:
  1. Implement ChatterboxEngine(TTSEngine) in services/tts_engine.py
  2. Change one line in this file: engine = ChatterboxEngine(config)
  3. Nothing else changes — same API, same routes, same frontend.
"""

import logging
import os
import uuid
from contextlib import asynccontextmanager

from fastapi import BackgroundTasks, FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from config import Config
from services.job_manager import JobManager, JobStatus
from services.storage import MinIOStorage
from services.tts_engine import EdgeTTSEngine, TTSEngine

# ── Logging ───────────────────────────────────────────────────────────────────
logging.basicConfig(
    level=getattr(logging, Config.LOG_LEVEL, logging.INFO),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

# ── Global singletons (initialized at startup) ────────────────────────────────
tts_engine: TTSEngine = None
storage: MinIOStorage = None
job_manager: JobManager = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize services on startup, clean up on shutdown."""
    global tts_engine, storage, job_manager

    logger.info("🔊 TTS Service starting up...")

    # ── Initialize the TTS engine ──────────────────────────────────────────
    # SWAP POINT: replace EdgeTTSEngine with ChatterboxEngine when GPU is ready
    tts_engine = EdgeTTSEngine()
    logger.info(f"TTS engine: {tts_engine.__class__.__name__}")

    # ── Initialize MinIO storage ───────────────────────────────────────────
    if Config.MINIO_ENABLED and Config.MINIO_ENDPOINT:
        try:
            storage = MinIOStorage(
                endpoint=Config.MINIO_ENDPOINT,
                access_key=Config.MINIO_USER,
                secret_key=Config.MINIO_PASSWORD,
                bucket=Config.MINIO_BUCKET_AUDIO,
                use_ssl=Config.MINIO_USE_SSL,
            )
            await storage.ensure_bucket()
            logger.info(f"✅ MinIO storage ready. Bucket: {Config.MINIO_BUCKET_AUDIO}")
        except Exception as e:
            logger.warning(f"⚠️ MinIO unavailable ({e}). Continuing without storage (local fallback needed).")
            storage = None
    else:
        logger.info("ℹ️ MinIO disabled via config.")
        storage = None

    # ── Initialize Redis job manager ───────────────────────────────────────
    job_manager = JobManager(redis_url=Config.REDIS_URL)
    await job_manager.connect()
    logger.info("Redis job manager connected")

    logger.info(f"🔊 TTS Service running on port {Config.PORT}")
    yield

    # Shutdown
    await job_manager.disconnect()
    logger.info("TTS Service shut down")


# ── FastAPI app ───────────────────────────────────────────────────────────────
app = FastAPI(
    title="Vocalize TTS Service",
    description="Text-to-Speech synthesis for Vocalize audio ads",
    version="1.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # The Go backend calls this service internally
    allow_methods=["*"],
    allow_headers=["*"],
)


# ── Request / Response models ─────────────────────────────────────────────────

class GenerateAudioRequest(BaseModel):
    """
    Request to synthesize a script into audio.
    Mirrors the Go TTSGenerateRequest struct exactly.
    """
    script_text: str
    language: str = "en"          # en | fr | ar
    voice_id: str | None = None   # use default voice for language if not specified
    speed: float = 1.0            # 0.5 – 2.0 (1.0 = normal)
    # Metadata — stored with the job for reference
    ad_id: str | None = None      # the generated_ads.id this audio is for
    user_id: str | None = None


class GenerateAudioResponse(BaseModel):
    job_id: str
    status: str = "queued"
    message: str = "Audio generation queued. Poll /api/tts/status/{job_id} for updates."


class JobStatusResponse(BaseModel):
    job_id: str
    status: str          # queued | processing | completed | failed
    audio_url: str | None = None
    duration_seconds: float | None = None
    file_size_bytes: int | None = None
    voice_id: str | None = None
    language: str | None = None
    error: str | None = None
    created_at: str | None = None
    completed_at: str | None = None


# ── Routes ────────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    """Health check — used by Docker HEALTHCHECK and Go backend."""
    storage_status = "ready" if storage else "disabled"
    return {
        "status": "ok",
        "service": "vocalize-tts",
        "engine": tts_engine.__class__.__name__ if tts_engine else "not_initialized",
        "storage": storage_status,
        "redis": "connected" if job_manager else "not_initialized",
    }


@app.post("/api/tts/generate", response_model=GenerateAudioResponse)
async def generate_audio(req: GenerateAudioRequest, background_tasks: BackgroundTasks):
    """
    Enqueue a TTS synthesis job.

    Returns immediately with a job_id. The actual synthesis runs in the
    background. Poll /api/tts/status/{job_id} to check progress.

    Typical timeline:
      - edge-tts (CPU): 2–8 seconds for a 30-second script
      - Chatterbox-Turbo (GPU): ~200ms for the same script
    """
    if not req.script_text or len(req.script_text.strip()) < 5:
        raise HTTPException(status_code=400, detail="script_text must be at least 5 characters")

    if req.language not in ("en", "fr", "ar"):
        raise HTTPException(status_code=400, detail="language must be en, fr, or ar")

    if not 0.5 <= req.speed <= 2.0:
        raise HTTPException(status_code=400, detail="speed must be between 0.5 and 2.0")

    # Resolve voice ID (use default for language if not specified)
    voice_id = req.voice_id or tts_engine.default_voice(req.language)

    # Create the job record in Redis
    job_id = str(uuid.uuid4())
    await job_manager.create_job(job_id, {
        "script_text": req.script_text,
        "language": req.language,
        "voice_id": voice_id,
        "speed": req.speed,
        "ad_id": req.ad_id,
        "user_id": req.user_id,
    })

    # Schedule background synthesis
    # FastAPI BackgroundTasks runs AFTER the response is sent
    background_tasks.add_task(
        _synthesize_and_store,
        job_id=job_id,
        script_text=req.script_text,
        language=req.language,
        voice_id=voice_id,
        speed=req.speed,
    )

    return GenerateAudioResponse(job_id=job_id)


@app.get("/api/tts/status/{job_id}", response_model=JobStatusResponse)
async def get_job_status(job_id: str):
    """
    Poll a synthesis job's status.

    Status flow:  queued → processing → completed (or failed)

    When status == "completed", the response includes:
      - audio_url: presigned MinIO URL valid for 7 days
      - duration_seconds: actual audio length
      - file_size_bytes: compressed MP3 size
    """
    job = await job_manager.get_job(job_id)
    if not job:
        raise HTTPException(status_code=404, detail=f"Job {job_id} not found")

    return JobStatusResponse(**job)


@app.get("/api/tts/voices")
async def list_voices():
    """
    List all available voices grouped by language.

    The frontend uses this to populate the voice selector in the generate page.
    """
    return {
        "voices": tts_engine.list_voices(),
        "engine": tts_engine.__class__.__name__,
    }


# ── Background task ───────────────────────────────────────────────────────────

async def _synthesize_and_store(
    job_id: str,
    script_text: str,
    language: str,
    voice_id: str,
    speed: float,
):
    """
    The actual synthesis pipeline — runs after the HTTP response is sent.

    Steps:
      1. Update job status to "processing"
      2. Call TTS engine → get raw audio bytes (MP3)
      3. Upload to MinIO → get presigned URL
      4. Update job status to "completed" with URL and metadata
      5. On any error → update job status to "failed" with error message
    """
    logger.info(f"[Job {job_id[:8]}] Starting synthesis: lang={language} voice={voice_id}")

    try:
        # Mark as processing
        await job_manager.update_job(job_id, {"status": JobStatus.PROCESSING})

        # ── Step 1: Synthesize ────────────────────────────────────────────
        audio_bytes, duration_seconds = await tts_engine.synthesize(
            text=script_text,
            language=language,
            voice_id=voice_id,
            speed=speed,
        )

        logger.info(
            f"[Job {job_id[:8]}] Synthesis done: "
            f"{len(audio_bytes)} bytes, {duration_seconds:.1f}s"
        )

        # ── Step 2: Store in MinIO ────────────────────────────────────────
        object_name = f"audio/{job_id}.mp3"
        presigned_url = await storage.upload(
            data=audio_bytes,
            object_name=object_name,
            content_type="audio/mpeg",
        )

        # ── Step 3: Mark completed ────────────────────────────────────────
        import datetime
        await job_manager.update_job(job_id, {
            "status": JobStatus.COMPLETED,
            "audio_url": presigned_url,
            "duration_seconds": duration_seconds,
            "file_size_bytes": len(audio_bytes),
            "completed_at": datetime.datetime.utcnow().isoformat(),
        })

        logger.info(f"[Job {job_id[:8]}] Completed. URL: {presigned_url[:60]}...")

    except Exception as e:
        logger.error(f"[Job {job_id[:8]}] Synthesis failed: {e}", exc_info=True)
        await job_manager.update_job(job_id, {
            "status": JobStatus.FAILED,
            "error": str(e),
        })
