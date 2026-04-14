# services/script-gen/main.py
import logging
from fastapi import FastAPI, HTTPException, status
from fastapi.responses import JSONResponse
import uvicorn

from config import Config
from services.script_generator import ScriptGenerator, ScriptRequest, ScriptResponse

# Setup logging
logging.basicConfig(level=Config.LOG_LEVEL)
logger = logging.getLogger(__name__)

# Create app
app = FastAPI(title="Vocalize Script Generation Service")

# Initialize service
script_generator = ScriptGenerator()


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "ok", "service": "script-gen"}


@app.post("/api/generate/script", response_model=ScriptResponse)
async def generate_script(request: ScriptRequest):
    """
    Generate marketing script via GLM Flash 4.7.

    Supports:
    - English (en): Native speaker quality
    - French (fr): Native speaker quality
    - Arabic (ar): Modern Standard Arabic (MSA) with diacritization

    Request body:
    {
      "product_name": "Premium Headphones",
      "product_description": "Noise-canceling wireless headphones with 30-hour battery",
      "target_audience": "Tech-savvy professionals aged 25-40",
      "tone": "FORMAL",
      "language": "en"
    }
    """
    try:
        # Validate language
        if request.language not in ["en", "fr", "ar"]:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Language must be: en, fr, or ar",
            )

        # Validate tone
        if request.tone not in ["FORMAL", "CASUAL", "PODCAST"]:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Tone must be: FORMAL, CASUAL, or PODCAST",
            )

        # Generate script
        response = script_generator.generate(request)

        return response

    except HTTPException:
        raise
    except ValueError as e:
        logger.error(f"Validation error: {e}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(e),
        )
    except Exception as e:
        logger.error(f"Script generation error: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to generate script. Please try again.",
        )


@app.post("/api/generate/script/regenerate")
async def regenerate_script(request: ScriptRequest):
    """
    Regenerate script with different tone/audience.
    Returns new version (increments version counter).
    """
    return await generate_script(request)


if __name__ == "__main__":
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=8001,
        log_level=Config.LOG_LEVEL.lower(),
    )