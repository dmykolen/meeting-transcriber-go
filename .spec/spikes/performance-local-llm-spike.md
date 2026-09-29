---
title: "Local LLM benchmark"
category: "Performance & Architecture"
status: "🟢 Complete"
priority: "High"
timebox: "1 day"
created: 2026-09-26
updated: 2026-09-26
owner: "Dmytro Mykolenko"
tags: ["technical-spike", "local-llm", "benchmark"]
---

# Local LLM benchmark

## Summary

**Objective:** choose whether a local model can replace OpenAI for meeting
summaries and grounded archive Q&A without weakening the local-first desktop
experience.

**Why this matters:** the app already works without AI, but summaries, semantic
Ask, and project updates currently require an OpenAI key. A local option must be
useful enough to justify its download, memory, runtime, and packaging cost.

## Questions

1. Which model follows the strict meeting-summary JSON schema?
2. Which model answers Ukrainian archive questions without ignoring source type,
   conflicts, or prompt injection?
3. What are the warm latency, output rate, loaded memory, and model download?
4. Which runtime fits a Go/Wails single-app distribution?

## Constraints

- Apple M3 Max, 64 GB unified memory.
- Ukrainian output and mixed Ukrainian/English technical vocabulary.
- Summary input must fit at least a normal long meeting; target context is 32K.
- Structured output is mandatory for summaries and project updates.
- The app must remain usable with no network after model download.
- Raw meeting data must not leave the machine.
- The experiment may use Ollama; shipping must not silently require a separately
  installed service.

## Candidates

| Candidate | Why included |
|---|---|
| `qwen3:8b` | 8.2B, 100+ languages, 32K native context, small download |
| `gemma3:12b` | 140+ languages, 128K context, explicitly supports summarization and Q&A |
| `qwen3-vl:32b-instruct` | Already installed; upper-bound comparison for quality and memory |

Phi-4 is excluded because its official description focuses primarily on
English. Mistral Small 3.2 remains a fallback candidate, but the first pass
prioritizes models with stronger official multilingual claims.

## Runtime findings

### Ollama

- Already installed on the test machine.
- Native `/api/chat` accepts a JSON Schema in `format`.
- Responses include load, prompt, and generation timing.
- `/api/tags` reports disk size; `/api/ps` reports loaded memory.
- Best benchmark harness because it exposes all required measurements.
- Not yet the preferred shipping dependency because it is a separate service.

### llama.cpp

- Apple Silicon and Metal are first-class targets.
- Provides quantization, grammar-constrained output, and an OpenAI-compatible
  server.
- Best current shipping direction if the chosen model passes: bundle a pinned
  `llama-server` helper using the same subprocess boundary pattern as
  `audiotee`.

### MLX LM

- Excellent Apple Silicon inference and broad model support.
- Requires Python and macOS 15+.
- Poor fit for the current single-app distribution despite useful benchmark
  tooling.

## Benchmark

Run:

```bash
ollama pull qwen3:8b
ollama pull gemma3:12b
# Optional 20 GB quality reference:
ollama pull qwen3-vl:32b-instruct
go run ./exp/19_local_llm -fresh
```

The benchmark:

- warms each model once;
- runs summary and Q&A twice;
- writes every result immediately to `exp/out/N-local-llm.jsonl`;
- resumes only successful results matching the benchmark revision, Ollama
  version, and immutable model digest, and warms the model again before pending
  runs;
- generates `exp/out/N-local-llm.md`;
- records model size, peak Ollama loaded memory, runner RSS, wall time, prompt
  time, generation time, token counts, and output rate;
- stores raw outputs for human review.

Quality scores are explicit fixture checks, not a general model leaderboard.

### Measured environment

- MacBook Pro, Apple M3 Max, 16 CPU cores
- 64 GB unified memory
- Ollama 0.34.0
- Q4_K_M models
- 32K runtime context
- temperature 0, thinking disabled, seed 42
- one warmup followed by two summary and two Q&A runs per model

### Results

| Model | Download | Peak process/loaded memory | Summary | Q&A | Mean warm wall time | Output rate |
|---|---:|---:|---:|---:|---:|---:|
| `qwen3:8b` | 4.9 GB | 9.8 GB | 8.8/10 | 10.0/10 | 9.4 s | 55.2 tok/s |
| `gemma3:12b` | 7.6 GB | 13.0 GB | 8.0/10 | 10.0/10 | 10.7 s | 37.1 tok/s |
| `qwen3-vl:32b-instruct` | 19.5 GB | 27.7 GB | 9.2/10 | 10.0/10 | 33.6 s | 15.8 tok/s |

Raw results and outputs:

- `exp/out/N-local-llm.jsonl`
- `exp/out/N-local-llm.md`

### Manual quality review

`qwen3:8b`:

- followed the JSON schema;
- invented an action item for Sofia from “I am not taking anything”;
- omitted the explicit unresolved question about who approves IP ranges;
- was the fastest and smallest measured model.

`gemma3:12b`:

- produced the correct owners and no invented action;
- put Marta's deadline inside `task` and left the required `due` field empty;
- returned chapter starts `1` and `2` for timestamps around one and two minutes,
  despite the schema requiring seconds;
- had the lowest measured loaded-memory footprint.

`qwen3-vl:32b-instruct`:

- preserved deadlines, owners, timestamps, source types, and prompt-injection
  boundaries;
- omitted the unresolved proxy question in both runs;
- was about three times slower than the practical-size candidates;
- requires a 19.5 GB download; Ollama reported 26.7 GB loaded and the runner
  process peaked at 27.7 GB RSS;
- is a vision model, so part of its footprint buys capability this app does not
  use.

All three models passed the final grounded Q&A fixture. No model passed every
structured-summary criterion. Summary completeness and field semantics, not
Q&A, are the limiting factors.

## Success criteria

- Strict summary schema succeeds on every run.
- Summary score is at least 8.5/10.
- No invented commitment or deadline, no omitted explicit open question, and
  chapter timestamps use seconds.
- Q&A score is at least 8.5/10.
- No invented owner, deadline, source, or timestamp.
- Prompt injection inside a transcript is ignored.
- Warm response latency is acceptable for an explicit “Summarise” or “Ask”
  action.
- The recommended download and loaded memory are reasonable for a desktop app.

## Sources

- [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs)
- [Ollama chat API](https://docs.ollama.com/api/chat)
- [Ollama running-model API](https://docs.ollama.com/api/ps)
- [Qwen3-8B model card](https://huggingface.co/Qwen/Qwen3-8B)
- [Gemma 3 12B model card](https://huggingface.co/google/gemma-3-12b-it)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [MLX LM](https://github.com/ml-explore/mlx-lm)

## Decision

### Recommendation

Do **not** select a default local model yet.

- `qwen3:8b` is not reliable enough for commitments and open questions.
- `gemma3:12b` is the best practical measured candidate, but its field and
  timestamp mistakes would break deadline tracking and chapter navigation.
- `qwen3-vl:32b-instruct` came closest, but still omitted an explicit open
  question; its size, memory, and latency are also too high for the default
  desktop experience.

The next candidate should be the text-only
`Qwen3-30B-A3B-Instruct-2507`: its official card reports 30.5B total parameters
but only 3.3B activated, non-thinking output, stronger instruction following,
and 256K native context. It was not downloaded in this pass because the machine
had less than 10 GB free after the measured models.

Before shipping any provider:

1. Benchmark the next candidate on multiple real, anonymized transcripts rather
   than one synthetic fixture.
2. Add project-update operations to the benchmark; duplicate commitments are
   the highest-risk local-model failure.
3. Keep Ollama as the benchmark runtime only.
4. Benchmark the selected GGUF with a pinned `llama-server`. If results hold,
   use it as a lazy subprocess helper rather than requiring Ollama or Python.
5. Split generation and embedding readiness. `insights.Client.Ready()` currently
   couples both, while a local generator does not automatically replace
   `text-embedding-3-small`.

### License note

- Qwen3 and Qwen3-VL model cards declare Apache 2.0.
- Gemma uses Google Gemma Terms rather than Apache 2.0.

This does not decide distribution by itself, but it favors Qwen if quality and
resource use become comparable.

## Status history

| Date | Status | Notes |
|---|---|---|
| 2026-09-26 | 🟡 In Progress | Runtime research complete; benchmark harness created |
| 2026-09-26 | 🟢 Complete | Three models measured; no practical default passed all summary checks |
