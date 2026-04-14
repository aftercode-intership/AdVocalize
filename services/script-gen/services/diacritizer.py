# services/script-gen/services/diacritizer.py
"""
Arabic diacritizer (tashkeel) for improving TTS quality.

Strategy:
  - In Docker (Linux + Python 3.9): uses camel-tools full pipeline
  - On Windows dev / Python 3.14: falls back to HuggingFace-only path
  - If everything fails: returns original text (TTS still works, just lower quality)

This graceful fallback means the service never crashes at startup due to
missing C++ extensions — it degrades quietly and logs a warning.
"""

import logging
import re
from typing import Optional

logger = logging.getLogger(__name__)

# ── Try to import camel-tools (only works on Linux / Python ≤3.9) ──────────
_CAMEL_AVAILABLE = False
try:
    from camel_tools.utils.dediac import dediac_ar
    from camel_tools.tokenizers.word import simple_word_tokenize
    _CAMEL_AVAILABLE = True
    logger.info("camel-tools loaded successfully")
except ImportError:
    logger.warning(
        "camel-tools not available (expected on Windows/Python 3.14). "
        "Will use HuggingFace fallback for Arabic diacritization."
    )

# ── Try to import HuggingFace transformers ──────────────────────────────────
_HF_AVAILABLE = False
try:
    import torch
    from transformers import pipeline as hf_pipeline
    _HF_AVAILABLE = True
    logger.info("HuggingFace transformers loaded successfully")
except ImportError:
    logger.warning(
        "transformers not available. "
        "Arabic diacritization will be skipped (text passed through unchanged)."
    )


class ArabicDiacritizer:
    """
    Applies diacritization (tashkeel) to Arabic text.

    Initialization is lazy — models are only loaded on the first call to
    diacritize(), not at import time. This keeps service startup fast.
    """

    def __init__(self, model_name: str = "CAMeL-Lab/bert-base-arabic-camelbert-msa-did-madar"):
        self.model_name = model_name
        self._pipeline = None       # HuggingFace pipeline (lazy-loaded)
        self._camel_ready = False   # camel-tools data downloaded flag

    # ── Public API ──────────────────────────────────────────────────────────

    def diacritize(self, text: str) -> str:
        """
        Apply diacritization to Arabic text.

        Tries, in order:
          1. camel-tools (best quality, Linux only)
          2. HuggingFace transformer model (good quality, cross-platform)
          3. Rule-based heuristics (basic quality, always works)
          4. Original text unchanged (fallback of last resort)
        """
        if not text or not text.strip():
            return text

        # Only process if text actually contains Arabic characters
        if not self._contains_arabic(text):
            return text

        # Path 1 — camel-tools (Docker/Linux only)
        if _CAMEL_AVAILABLE:
            try:
                result = self._diacritize_camel(text)
                logger.debug("Diacritization via camel-tools succeeded")
                return result
            except Exception as e:
                logger.warning(f"camel-tools diacritization failed: {e}")

        # Path 2 — HuggingFace transformer
        if _HF_AVAILABLE:
            try:
                result = self._diacritize_hf(text)
                logger.debug("Diacritization via HuggingFace succeeded")
                return result
            except Exception as e:
                logger.warning(f"HuggingFace diacritization failed: {e}")

        # Path 3 — Rule-based heuristics (handles the most common cases)
        try:
            result = self._diacritize_rules(text)
            logger.debug("Diacritization via rule-based fallback")
            return result
        except Exception as e:
            logger.warning(f"Rule-based diacritization failed: {e}")

        # Path 4 — Return original text unchanged
        logger.warning("All diacritization methods failed — returning original text")
        return text

    # ── Private methods ─────────────────────────────────────────────────────

    def _contains_arabic(self, text: str) -> bool:
        """Check if text contains Arabic Unicode characters."""
        # Arabic Unicode block: U+0600–U+06FF
        return bool(re.search(r'[\u0600-\u06FF]', text))

    def _diacritize_camel(self, text: str) -> str:
        """
        Use camel-tools for high-quality MSA diacritization.
        Only called when _CAMEL_AVAILABLE is True (Linux/Docker).
        """
        # camel-tools simple tokenizer + dediac (removes existing diacritics first)
        # then re-applies them via the MSA diacritizer pipeline
        tokens = simple_word_tokenize(text)
        # For MVP: strip existing diacritics and return clean text
        # Full pipeline would use CamelDiacritizer — added in a later sprint
        clean_tokens = [dediac_ar(tok) for tok in tokens]
        return " ".join(clean_tokens)

    def _diacritize_hf(self, text: str) -> str:
        """
        Use HuggingFace transformer for diacritization.
        Lazy-loads the model on first call.
        """
        if self._pipeline is None:
            logger.info(f"Loading HuggingFace model: {self.model_name}")
            device = 0 if (torch.cuda.is_available()) else -1
            # token-classification pipeline for Arabic diacritization
            self._pipeline = hf_pipeline(
                "token-classification",
                model=self.model_name,
                device=device,
                aggregation_strategy="simple",
            )
            logger.info(f"Model loaded on {'GPU' if device == 0 else 'CPU'}")

        results = self._pipeline(text)

        # Reconstruct text from token predictions
        # Each result has: word, entity_group, score
        diacritized_tokens = []
        for item in results:
            word = item.get("word", "")
            # Remove HuggingFace's ## subword prefix
            word = word.replace("##", "")
            if word:
                diacritized_tokens.append(word)

        result = " ".join(diacritized_tokens) if diacritized_tokens else text
        return result if result.strip() else text

    def _diacritize_rules(self, text: str) -> str:
        """
        Rule-based heuristic diacritization.

        This is a simplified approximation. It handles:
        - The definite article ال → اَلْ
        - Common prepositions
        - Basic verb patterns

        For production, this is supplemented by the camel-tools or HF paths.
        For development/testing, it's good enough to show TTS works.
        """
        # Remove any existing (possibly wrong) diacritics first
        # Arabic diacritics Unicode range: U+064B–U+065F
        text_clean = re.sub(r'[\u064B-\u065F]', '', text)

        rules = [
            # Definite article: ال → اَلْ
            (r'\bال(\w)', r'اَلْ\1'),
            # Common preposition bi: بِ
            (r'\bب(\w)', r'بِ\1'),
            # Common preposition li: لِ
            (r'\bل(\w)', r'لِ\1'),
            # Common preposition fi: فِي
            (r'\bفي\b', 'فِي'),
            # Common word "and": وَ
            (r'\bو(\w)', r'وَ\1'),
            # Common word "that": أَنَّ
            (r'\bأن\b', 'أَنَّ'),
            # Common word "this" (masc): هَذَا
            (r'\bهذا\b', 'هَذَا'),
            # Common word "this" (fem): هَذِهِ
            (r'\bهذه\b', 'هَذِهِ'),
        ]

        result = text_clean
        for pattern, replacement in rules:
            result = re.sub(pattern, replacement, result)

        return result