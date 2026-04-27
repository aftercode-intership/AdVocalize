# services/tts/services/job_manager.py
"""
Redis-backed job manager for TTS synthesis jobs.

Each job is stored as a Redis hash at key "tts:job:{job_id}".
Fields: status, audio_url, duration_seconds, file_size_bytes,
        voice_id, language, error, created_at, completed_at, ...

Jobs expire automatically after JOB_TTL_SECONDS (default: 7 days).
This prevents unbounded Redis memory growth without needing a separate
cleanup process.
"""

import asyncio
import datetime
import json
import logging
from enum import Enum

import redis.asyncio as aioredis

logger = logging.getLogger(__name__)

# How long job records survive in Redis after creation (seconds)
DEFAULT_TTL = 7 * 24 * 3600  # 7 days


class JobStatus:
    QUEUED     = "queued"
    PROCESSING = "processing"
    COMPLETED  = "completed"
    FAILED     = "failed"


class JobManager:
    """
    Manages TTS job lifecycle in Redis.

    We use Redis hashes (HSET/HGETALL) rather than JSON strings for
    two reasons:
      1. Individual fields can be updated atomically without reading the whole job
      2. Redis memory is slightly more efficient for structured data
    """

    def __init__(self, redis_url: str, job_ttl: int = DEFAULT_TTL):
        self.redis_url = redis_url
        self.job_ttl = job_ttl
        self._redis: aioredis.Redis | None = None

    async def connect(self):
        """Open the Redis connection pool."""
        self._redis = await aioredis.from_url(
            self.redis_url,
            encoding="utf-8",
            decode_responses=True,
        )
        # Verify connection
        await self._redis.ping()
        logger.info(f"Connected to Redis: {self.redis_url}")

    async def disconnect(self):
        """Close the Redis connection pool."""
        if self._redis:
            await self._redis.aclose()
            self._redis = None

    def _key(self, job_id: str) -> str:
        return f"tts:job:{job_id}"

    async def create_job(self, job_id: str, metadata: dict) -> str:
        """
        Create a new job record with status "queued".

        Stores the job metadata (script_text, language, voice_id, etc.)
        along with the initial status and creation timestamp.

        Returns the job_id.
        """
        now = datetime.datetime.utcnow().isoformat()

        job_data = {
            "job_id":     job_id,
            "status":     JobStatus.QUEUED,
            "created_at": now,
            # Metadata fields
            "language":   metadata.get("language", "en"),
            "voice_id":   metadata.get("voice_id", ""),
            "ad_id":      metadata.get("ad_id", "") or "",
            "user_id":    metadata.get("user_id", "") or "",
            # Result fields (populated when completed)
            "audio_url":         "",
            "duration_seconds":  "",
            "file_size_bytes":   "",
            "completed_at":      "",
            "error":             "",
        }

        key = self._key(job_id)
        await self._redis.hset(key, mapping=job_data)
        await self._redis.expire(key, self.job_ttl)

        logger.debug(f"Job created: {job_id}")
        return job_id

    async def update_job(self, job_id: str, updates: dict):
        """
        Partially update a job's fields.

        Called by the background task to transition the job through states:
          create_job() → update(processing) → update(completed/failed)
        """
        key = self._key(job_id)

        # Convert non-string values to strings (Redis hashes store strings)
        string_updates = {}
        for k, v in updates.items():
            if v is None:
                string_updates[k] = ""
            elif isinstance(v, (int, float)):
                string_updates[k] = str(v)
            else:
                string_updates[k] = str(v)

        await self._redis.hset(key, mapping=string_updates)
        # Refresh TTL on each update so active jobs don't expire
        await self._redis.expire(key, self.job_ttl)

        logger.debug(f"Job {job_id[:8]} updated: {list(updates.keys())}")

    async def get_job(self, job_id: str) -> dict | None:
        """
        Retrieve a job's current state.

        Returns None if the job doesn't exist (never created or expired).
        Returns a dict with all job fields otherwise.
        """
        key = self._key(job_id)
        data = await self._redis.hgetall(key)

        if not data:
            return None

        # Type-cast numeric fields from Redis string format
        result = dict(data)

        if result.get("duration_seconds"):
            try:
                result["duration_seconds"] = float(result["duration_seconds"])
            except (ValueError, TypeError):
                result["duration_seconds"] = None

        if result.get("file_size_bytes"):
            try:
                result["file_size_bytes"] = int(result["file_size_bytes"])
            except (ValueError, TypeError):
                result["file_size_bytes"] = None

        # Replace empty strings with None for cleaner JSON responses
        for field in ("audio_url", "error", "completed_at", "ad_id", "user_id"):
            if result.get(field) == "":
                result[field] = None

        return result

    async def list_jobs(self, user_id: str, limit: int = 20) -> list[dict]:
        """
        List recent jobs for a user.
        Currently does a pattern scan — fine for dev, add an index for production.
        """
        pattern = "tts:job:*"
        keys = []
        async for key in self._redis.scan_iter(match=pattern, count=100):
            keys.append(key)
            if len(keys) >= limit * 5:  # over-fetch to filter by user
                break

        jobs = []
        for key in keys:
            data = await self._redis.hgetall(key)
            if data and data.get("user_id") == user_id:
                jobs.append(data)

        # Sort by created_at descending
        jobs.sort(key=lambda j: j.get("created_at", ""), reverse=True)
        return jobs[:limit]