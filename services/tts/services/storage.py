# services/tts/services/storage.py
"""
MinIO storage client for audio files.

Uploads MP3 audio to MinIO (S3-compatible) and returns presigned URLs
that the frontend can use to stream/download the audio directly.

Presigned URLs expire after 7 days (configurable via MINIO_PRESIGN_EXPIRY).
The frontend audio player uses these URLs directly — no proxy needed.
"""

import datetime
import io
import logging

from minio import Minio
from minio.error import S3Error

logger = logging.getLogger(__name__)


class MinIOStorage:
    """
    Wraps the MinIO Python SDK with async-compatible methods.

    The MinIO SDK is synchronous. We run blocking calls in a thread pool
    using asyncio.to_thread so we don't block the event loop during upload.
    """

    def __init__(
        self,
        endpoint: str,
        access_key: str,
        secret_key: str,
        bucket: str,
        use_ssl: bool = False,
        presign_expiry_seconds: int = 7 * 24 * 3600,
    ):
        self.bucket = bucket
        self.presign_expiry = datetime.timedelta(seconds=presign_expiry_seconds)

        # MinIO client (synchronous)
        self._client = Minio(
            endpoint=endpoint,
            access_key=access_key,
            secret_key=secret_key,
            secure=use_ssl,
        )

    async def ensure_bucket(self):
        """Create the audio bucket if it doesn't exist."""
        import asyncio
        await asyncio.to_thread(self._ensure_bucket_sync)

    def _ensure_bucket_sync(self):
        try:
            if not self._client.bucket_exists(self.bucket):
                self._client.make_bucket(self.bucket)
                logger.info(f"Created MinIO bucket: {self.bucket}")
            else:
                logger.debug(f"MinIO bucket already exists: {self.bucket}")
        except S3Error as e:
            logger.error(f"Failed to ensure bucket {self.bucket}: {e}")
            raise

    async def upload(
        self,
        data: bytes,
        object_name: str,
        content_type: str = "audio/mpeg",
    ) -> str:
        """
        Upload bytes to MinIO and return a presigned download URL.

        Args:
            data: raw file bytes (MP3 audio)
            object_name: path within the bucket, e.g. "audio/job-uuid.mp3"
            content_type: MIME type

        Returns:
            Presigned URL valid for self.presign_expiry
        """
        import asyncio
        return await asyncio.to_thread(
            self._upload_sync, data, object_name, content_type
        )

    def _upload_sync(self, data: bytes, object_name: str, content_type: str) -> str:
        data_stream = io.BytesIO(data)

        self._client.put_object(
            bucket_name=self.bucket,
            object_name=object_name,
            data=data_stream,
            length=len(data),
            content_type=content_type,
        )

        logger.info(f"Uploaded: {object_name} ({len(data):,} bytes)")

        # Generate a presigned URL for direct browser access
        url = self._client.presigned_get_object(
            bucket_name=self.bucket,
            object_name=object_name,
            expires=self.presign_expiry,
        )

        return url

    async def delete(self, object_name: str):
        """Remove an audio file from storage."""
        import asyncio
        await asyncio.to_thread(self._delete_sync, object_name)

    def _delete_sync(self, object_name: str):
        try:
            self._client.remove_object(self.bucket, object_name)
            logger.info(f"Deleted: {object_name}")
        except S3Error as e:
            logger.warning(f"Failed to delete {object_name}: {e}")