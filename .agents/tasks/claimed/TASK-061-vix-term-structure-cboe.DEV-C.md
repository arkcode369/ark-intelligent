# TASK-061: VIX Term Structure Engine (CBOE CSV)
**Claimed by:** Dev-C
**Status:** in-progress
**Branch:** feat/TASK-061-vix-term-structure
**Started:** 2026-04-01T18:55+08:00

## Work
- Wire VIX engine (already in internal/service/vix/) into sentiment Fetch() pipeline
- Populate VIX fields in SentimentData from vix.Cache
- Add VIX Term Structure section to FormatSentiment output
- Build, vet, test
