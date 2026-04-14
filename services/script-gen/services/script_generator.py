# services/script-gen/services/script_generator.py
import logging
import json
import hashlib
from datetime import datetime, timedelta
from typing import Optional, Dict, List
import redis
import httpx
from pydantic import BaseModel

from config import Config
from services.diacritizer import ArabicDiacritizer

logger = logging.getLogger(__name__)


class ScriptRequest(BaseModel):
    product_name: str
    product_description: str
    target_audience: str
    tone: str  # FORMAL, CASUAL, PODCAST
    language: str  # en, fr, ar
    campaign_id: Optional[str] = None
    brand_guidelines: Optional[str] = None


class ScriptResponse(BaseModel):
    script_text: str
    language: str
    tone: str
    word_count: int
    estimated_duration_seconds: int
    version: int = 1


class ScriptGenerator:
    """
    Generates marketing copy using GLM Flash 4.7.
    Supports English, French, and Arabic (with diacritization).
    """

    def __init__(self):
        self.redis_client = redis.from_url(Config.REDIS_URL)
        self.diacritizer = ArabicDiacritizer()
        self.glm_api_url = "https://open.bigmodel.cn/api/paas/v4/chat/completions"  # Example

    def generate(self, request: ScriptRequest) -> ScriptResponse:
        """
        Generate script with caching and error handling.
        """
        # Check cache
        cache_key = self._generate_cache_key(request)
        cached = self._get_from_cache(cache_key)
        if cached:
            logger.info(f"Cache hit for {request.language} script")
            return cached

        # Generate prompt
        prompt = self._build_prompt(request)

        # Call GLM Flash 4.7
        try:
            raw_script = self._call_glm_api(prompt, request.language)
        except Exception as e:
            logger.error(f"GLM API call failed: {e}")
            raise

        # Post-process based on language
        if request.language == "ar":
            # Apply diacritization for Arabic
            script_text = self.diacritizer.diacritize(raw_script)
        else:
            script_text = raw_script

        # Validate script
        validated_script = self._validate_script(script_text, request.language)

        # Calculate metrics
        word_count = len(validated_script.split())
        # Approximate: 180 words/min = 3 words/sec = ~27-30 sec for 90 words
        estimated_duration = max(15, min(60, int((word_count / 3) + 2)))

        response = ScriptResponse(
            script_text=validated_script,
            language=request.language,
            tone=request.tone,
            word_count=word_count,
            estimated_duration_seconds=estimated_duration,
        )

        # Cache result
        self._cache_result(cache_key, response)

        return response

    def _build_prompt(self, request: ScriptRequest) -> str:
        """
        Build language-specific prompt for GLM Flash.
        Includes Hook-Problem-Solution-CTA structure.
        """
        if request.language == "en":
            return self._prompt_english(request)
        elif request.language == "fr":
            return self._prompt_french(request)
        elif request.language == "ar":
            return self._prompt_arabic(request)
        else:
            raise ValueError(f"Unsupported language: {request.language}")

    def _prompt_english(self, request: ScriptRequest) -> str:
        """English prompt for GLM Flash."""
        tone_instruction = {
            "FORMAL": "professional, corporate, authoritative tone",
            "CASUAL": "friendly, conversational, approachable tone",
            "PODCAST": "narrative, engaging, storytelling tone",
        }.get(request.tone, "neutral tone")

        brand_context = f"\nBrand Guidelines: {request.brand_guidelines}" if request.brand_guidelines else ""

        return f"""Generate a persuasive 30-second marketing script in English with {tone_instruction}.

PRODUCT: {request.product_name}
DESCRIPTION: {request.product_description}
TARGET AUDIENCE: {request.target_audience}{brand_context}

STRUCTURE (REQUIRED):
1. HOOK (2-3 seconds): Attention-grabbing opening
2. PROBLEM (5-7 seconds): Identify pain point
3. SOLUTION (10-12 seconds): Present product benefit
4. CTA (3-5 seconds): Clear call-to-action

REQUIREMENTS:
- Total: 40-200 words (aim for 80-120 words)
- Natural, conversational language
- No hyphens in words
- Direct, compelling copy
- One clear action for audience

Output ONLY the script text, no explanations."""

    def _prompt_french(self, request: ScriptRequest) -> str:
        """French prompt for GLM Flash."""
        tone_instruction = {
            "FORMAL": "ton professionnel, corporatif, d'autorité",
            "CASUAL": "ton amical, conversationnel, abordable",
            "PODCAST": "ton narratif, engageant, narratif",
        }.get(request.tone, "ton neutre")

        brand_context = f"\nDirectives de Marque: {request.brand_guidelines}" if request.brand_guidelines else ""

        return f"""Générez un script marketing persuasif de 30 secondes en français avec {tone_instruction}.

PRODUIT: {request.product_name}
DESCRIPTION: {request.product_description}
PUBLIC CIBLE: {request.target_audience}{brand_context}

STRUCTURE (OBLIGATOIRE):
1. ACCROCHE (2-3 secondes): Ouverture captivante
2. PROBLÈME (5-7 secondes): Identifier le point faible
3. SOLUTION (10-12 secondes): Présenter l'avantage du produit
4. APPEL À L'ACTION (3-5 secondes): Action claire pour l'audience

EXIGENCES:
- Total: 40-200 mots (viser 80-120 mots)
- Langage naturel et conversationnel
- Pas de tirets dans les mots
- Copie directe et convaincante
- Une action claire pour l'audience

Veuillez UNIQUEMENT donner le texte du script, pas d'explications."""

    def _prompt_arabic(self, request: ScriptRequest) -> str:
        """Arabic prompt for GLM Flash (Modern Standard Arabic - MSA)."""
        tone_instruction = {
            "FORMAL": "نبرة مهنية وسلطوية",
            "CASUAL": "نبرة ودية وودية",
            "PODCAST": "نبرة سردية وجذابة",
        }.get(request.tone, "نبرة محايدة")

        brand_context = f"\nإرشادات العلامة التجارية: {request.brand_guidelines}" if request.brand_guidelines else ""

        return f"""اكتب نصاً إعلانياً مقنعاً مدته 30 ثانية باللغة العربية (الفصحى) مع {tone_instruction}.

المنتج: {request.product_name}
الوصف: {request.product_description}
الجمهور المستهدف: {request.target_audience}{brand_context}

الهيكل (إلزامي):
1. التعليق (2-3 ثوان): فتحة تلفت الانتباه
2. المشكلة (5-7 ثوان): تحديد نقطة الألم
3. الحل (10-12 ثانية): عرض فائدة المنتج
4. دعوة للعمل (3-5 ثوان): إجراء واضح للجمهور

المتطلبات:
- الإجمالي: 40-200 كلمة (استهدف 80-120 كلمة)
- لغة طبيعية وحوارية
- بدون شرطات في الكلمات
- نسخة مباشرة وقاطعة
- إجراء واحد واضح للجمهور

يرجى تقديم نص السيناريو فقط، بدون تفسيرات."""

    def _call_glm_api(self, prompt: str, language: str) -> str:
        """Call GLM Flash 4.7 API."""
        try:
            with httpx.Client(timeout=Config.GLM_TIMEOUT) as client:
                response = client.post(
                    self.glm_api_url,
                    headers={
                        "Authorization": f"Bearer {Config.GLM_API_KEY}",
                        "Content-Type": "application/json",
                    },
                    json={
                        "model": Config.GLM_MODEL,
                        "messages": [
                            {
                                "role": "user",
                                "content": prompt,
                            }
                        ],
                        "temperature": 0.7,
                        "top_p": 0.95,
                        "max_tokens": 500,
                    },
                )

            if response.status_code != 200:
                logger.error(f"GLM API error: {response.status_code} - {response.text}")
                raise Exception(f"GLM API returned {response.status_code}")

            data = response.json()
            script = data["choices"][0]["message"]["content"].strip()

            logger.info(f"Generated {language} script ({len(script)} chars)")
            return script

        except httpx.TimeoutException:
            logger.error("GLM API timeout")
            raise Exception("GLM API request timed out")
        except Exception as e:
            logger.error(f"GLM API error: {e}")
            raise

    def _validate_script(self, script: str, language: str) -> str:
        """Validate and clean script."""
        # Remove extra whitespace
        script = " ".join(script.split())

        # Check for basic issues
        if language == "ar":
            # For Arabic, basic validation
            if len(script.strip()) < 10:
                raise ValueError("Script too short")
        else:
            # For English/French
            if len(script.split()) < 10:
                raise ValueError("Script too short")

        return script

    def _generate_cache_key(self, request: ScriptRequest) -> str:
        """Generate cache key from request."""
        cache_str = f"{request.product_name}:{request.tone}:{request.language}"
        return f"script:{hashlib.md5(cache_str.encode()).hexdigest()}"

    def _get_from_cache(self, key: str) -> Optional[ScriptResponse]:
        """Retrieve from Redis cache."""
        try:
            cached = self.redis_client.get(key)
            if cached:
                data = json.loads(cached)
                return ScriptResponse(**data)
        except Exception as e:
            logger.warning(f"Cache retrieval failed: {e}")

        return None

    def _cache_result(self, key: str, response: ScriptResponse):
        """Store in Redis cache."""
        try:
            self.redis_client.setex(
                key,
                Config.CACHE_TTL,
                response.model_dump_json(),
            )
        except Exception as e:
            logger.warning(f"Cache storage failed: {e}")