# ─────────────────────────────────────────────────────────────────────────────
# Vocalize local development stack.
#
# Services:
#   Infrastructure (always running):
#     postgres    → main database
#     redis       → job queue + caching
#     minio       → S3-compatible file storage
#     loki        → log aggregation
#     promtail    → log shipper (reads Docker logs → sends to Loki)
#     prometheus  → metrics collection
#     grafana     → metrics + logs dashboard  http://localhost:3001
#     traefik     → API gateway / reverse proxy
#
#   Application (uncomment as you build each sprint):
#     backend     → Go/Fiber API              http://localhost:8081
#     script-gen  → Python script generation  http://localhost:8001  ← NOW ACTIVE
#     tts         → Python TTS service        http://localhost:8002  (Sprint 5)
#     voice-clone → Python voice cloning      http://localhost:8003  (Sprint 7)
#     video-gen   → Python video generation   http://localhost:8004  (Sprint 9)
#     frontend    → Next.js UI                http://localhost:3000  (Sprint 2+)
#
# Usage:
#   Start infrastructure:  docker compose up -d postgres redis minio
#   Start everything:      docker compose up -d
#   View logs:             docker compose logs -f script-gen
#   Rebuild after change:  docker compose up -d --build script-gen
# ─────────────────────────────────────────────────────────────────────────────