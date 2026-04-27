# Fix Plan — Chat Sessions, Audio Generation, GLM Timeouts

## Issues Found
1. **Chat history 500 error**: `GetSessionHistory` can't scan NULL `user_id` (AI messages) into Go `string`
2. **TTS audio generation fails**: `_synthesize_and_store` crashes when `storage` is `None` (MinIO not configured)
3. **GLM timeouts**: NVIDIA API times out (25s × 3 retries), causing chat 500 errors
4. **Frontend silent failures**: `useChat.ts` silently ignores HTTP errors, leaving spinners stuck

## Steps
- [ ] Step 1: Fix TTS null-storage crash (`services/tts/main.py`)
- [ ] Step 2: Fix TTS config defaults (`services/tts/config.py`)
- [ ] Step 3: Fix chat NULL `user_id` scanning (`backend/internal/services/chat_service.go`)
- [ ] Step 4: Improve GLM timeout handling (`backend/internal/services/glm_service.go`)
- [ ] Step 5: Return 503 for AI timeouts (`backend/internal/handlers/chat_handler.go`)
- [ ] Step 6: Fix frontend silent failures (`frontend/src/hooks/useChat.ts`)
- [ ] Step 7: Fix frontend TTS error display (`frontend/src/hooks/useAudioGeneration.ts`)

