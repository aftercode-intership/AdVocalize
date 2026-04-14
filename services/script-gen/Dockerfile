# services/script-gen/Dockerfile
# ─────────────────────────────────────────────────────────────────────────────
# Multi-stage build for the Script Generation microservice.
#
# Stage 1 (builder): installs all dependencies including C++ build tools
#                    needed for camel-tools and editdistance.
# Stage 2 (runtime): copies only the installed packages — no build tools
#                    in the final image, keeping it smaller and more secure.
#
# Why Python 3.9?
#   camel-tools requires Python ≤3.9. The service runs in Docker (Linux)
#   so this has no impact on your Windows dev environment.
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: Builder ────────────────────────────────────────────────────────
FROM python:3.9-slim AS build_env

WORKDIR /build

# Install system dependencies needed to compile camel-kenlm and editdistance
# cmake       → required by camel-kenlm
# build-essential, g++ → C++ compiler for both packages
# git         → some HuggingFace models fetch via git during download
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    build-essential \
    g++ \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy only requirements first — Docker caches this layer.
# If your code changes but requirements.txt doesn't, pip install is skipped.
COPY requirements.txt .

# Install all Python dependencies into a separate directory
# We use --extra-index-url for the PyTorch CPU-only build
RUN pip install --no-cache-dir --prefix=/install \
    --extra-index-url https://download.pytorch.org/whl/cpu \
    -r requirements.txt

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM python:3.9-slim AS run_env

WORKDIR /app

# Copy installed Python packages from builder (no cmake/g++ in this stage)
COPY --from=build_env /install /usr/local

# Download camel-tools Arabic data files
# These are needed for the MSA diacritizer to work
# --no-confirm skips the interactive prompt
# || true means: if this fails (e.g., no internet at build time), continue anyway
# The service will fall back to HuggingFace or rule-based diacritization
RUN python -m camel_tools.data download --no-confirm || \
    echo "Warning: camel-tools data download failed — will use HuggingFace fallback"

# Copy application source code
# (done last so code changes don't invalidate the pip install cache)
COPY . .

# Create a non-root user for security
# Running as root inside containers is a security risk
RUN useradd -m -u 1000 scriptgen
USER scriptgen

# Port the service listens on
EXPOSE 8001

# Health check — Docker will restart the container if this fails 3 times
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD python -c "import httpx; httpx.get('http://localhost:8001/health').raise_for_status()" \
    || exit 1

# Start the service
# --workers 1: single worker (model is loaded once, shared)
# --host 0.0.0.0: listen on all interfaces inside the container
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001", "--workers", "1"]