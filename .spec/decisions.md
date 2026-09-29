# Engineering decisions

These are measured or product-critical choices that are easy to accidentally
“simplify” into a regression. Re-test before changing them.

## Local-first boundary

- Transcription, diarization, playback, analytics, keyword search, and storage
  are local.
- AI is optional — OpenAI, GitHub Copilot, or models on this Mac — and
  isolated in `internal/insights`.
- Models are downloaded once into persistent app storage.
- The app window opens before model setup completes.
- Invalid or legacy persisted settings fall back independently. One bad setting
  must not prevent startup.

## One application process

- The frontend is embedded by Wails.
- The queue and HTTP MCP server share the same service and SQLite database.
- Claude uses the same executable through `--mcp-stdio`.
- Do not introduce a second daemon or MCP binary without a concrete need.

## Audio channels are semantic

- Left is microphone; right is system audio.
- System-channel speech is strong evidence of a meeting.
- The microphone side identifies the laptop owner.
- Diarization prefers the cleaner system channel.
- Do not average or discard channels before ownership-dependent logic.

## Echo and alignment

- Capture clocks drift. The system backlog is corrected one sample at a time,
  not in large blocks.
- Playback folding switches ownership between microphone and system audio in
  short windows, with a crossfade when ownership changes.
- A simple average duplicates far-side speech.
- Experimental NLMS and Speex cancellation were rejected after they failed on
  real unaligned capture. No echo-cancellation filter ships.
- Playback cache names are versioned so algorithm changes invalidate old files.

Evidence is under `exp/01_echo` through `exp/05_aligned` and corresponding
`exp/out/` files.

## Whisper context and timestamps

- Rolling text context was disabled because it caused repeated phrases and was
  slower on measured meetings.
- A glossary prompt was tested and reduced accuracy while requiring the context
  mode that caused repetition. Do not re-enable it as the vocabulary feature.
- Whisper VAD removes silence. Segment timestamps are mapped back to the
  original recording; raw token timestamps from the Go binding are not.
- Two VAD assets are intentional. `vad.bin` trims silence inside completed-file
  Whisper transcription; `silero_vad.onnx` runs continuously through sherpa to
  detect speech and start/stop live recording. Neither replaces the other.
- Transcript rows therefore use segment timing. Word-level navigation needs a
  verified mapped-token implementation, not the raw token fields.
- `large-v3` weights were much larger and slower for only marginal measured
  accuracy gain over the shipping turbo Q5 model.

Evidence: `exp/06_whisper_loops`, `exp/14_timeline`, `exp/15_rows`,
`exp/17_models`, and `exp/18_glossary`.

## Speaker strategy

- Split speakers freely, then rejoin clusters using enrolled identity.
- Diarizing the system channel prevents the laptop owner's room echo from
  contaminating far-side clustering.
- The shipping threshold is intentionally loose because identity rejoining is
  part of the full pipeline. Do not tune it against one file.
- A learned name requires at least 30 seconds of speech evidence.
- The microphone owner is assigned from settings, not matched as a stranger.
- Community-1 was measured and did not justify adding Python and PyTorch.

Evidence: `exp/11_speakers`, `exp/12_community1`, `exp/13_company`, and
`exp/16_speakers2`.

## Queue and live behavior

- The processing queue is single-worker.
- Active recording prevents new historical backlog jobs from starting. An
  already-running job is not preempted.
- Processing is heavy: two back-to-back meetings (1 h and 48 min) kept the Mac
  fully busy for 11.5 minutes right after the second one (2026-09-28). The
  settings therefore choose when the queue runs: after each recording (the
  default), at a time of day, or when the Mac is free.
- "At a time" is stateless: a recording is due at the first hh:mm on or after
  its start, so one made later waits for the next day, and one whose hour
  passed while the Mac slept runs as soon as it wakes.
- "Free" means nobody has touched the keyboard, mouse or trackpad for five
  minutes (IOKit `HIDIdleTime`), the one-minute load average is under half the
  cores, and the GPU's `Device Utilization %` is under 50. None of these needs
  a privacy permission. The GPU figure is a moment, so a spike only costs one
  30-second pass. Load in the two minutes after a job is the app's own and does
  not stop a batch while the Mac stays unattended.
- A recording asked for by hand ("Розшифрувати зараз", or a retry) goes next,
  whatever the schedule, but still after a job in flight and after a live
  recording. The request lives in memory; after a restart the recording follows
  the schedule again.
- Every queued recording shows why it waits: models, a recording, the hour,
  somebody at the Mac, other work, or its turn.
- Silence or a meeting with no words is a valid finished result.
- A transcription result remains useful when diarization fails.
- One failed job must not stop future jobs.

## Project state

- A project is a living state, not concatenated summaries.
- The model returns operations against stable IDs instead of a replacement
  document.
- Every durable item keeps source provenance.
- Repeated commitments update/restate one item rather than creating duplicates.
- Rebuild replays meetings from the beginning.
- Human edits pin content. The model may close a pinned item but may not
  silently reword or reassign it.

## UI behavior

- The app is a workspace, not a dashboard.
- The timeline, recording list, reader, and project dock stay spatially related.
- Library opens a meeting in place; other screens return to their origin.
- Destructive actions use bin + undo and reserve their layout space.
- Every wait has a visible state.
- Motion explains navigation or state and respects reduced motion.

## MCP safety

- HTTP is loopback-only.
- Tools are read-only, non-destructive, idempotent, and closed-world.
- The OpenAI key and raw voiceprint vectors are never exposed.

## Local LLM

- Local grounded Q&A is viable on all three measured models.
- No measured model matched every structured-summary contract; even the 32B
  model omitted one explicit open question.
- Measured 8B and 12B models made structured-summary errors that would
  corrupt action/deadline or chapter data.
- The owner chose to ship the option before quality is settled (2026-09-27):
  the default is the small Gemma 4 E2B Q4_0 (Apache-2.0, 2.8 GB), picked for
  size and licence, not measured against OpenAI. A bigger model is a Hugging
  Face `.gguf` link in the settings. Its first real run chaptered everything
  at 0 s and left a spoken deadline out of `due`.
- Ollama is the benchmark runtime, not a shipping dependency.
- The runtime is `llama-server` v0.5.0, built from a pinned commit and bundled
  like `audiotee`: one lazy helper process per model file, loopback only, a
  per-start key in its environment, and `--sleep-idle-seconds 300` so an idle
  model leaves memory. Linked in, its ggml would clash with whisper.cpp's; as a
  process, running out of memory cannot stop a recording.
- Local generation uses Chat Completions. llama-server's Responses endpoint
  ignores `text.format`; its chat endpoint turns the schema into a grammar.
- Generation and embeddings are separate choices. The local embedder is
  Qwen3-Embedding-0.6B Q8_0 with the query-side instruction; its 1024-wide
  vectors are cut to 512 and renormalised so the archive keeps one width.
  The `meta` table names the model behind the stored vectors; a different
  one clears them and reindexes, since vectors from two models do not compare.
- Meetings, notes and projects are re-embedded in the background as soon as
  the embedder changes, with a named progress line in the settings. Left to the
  first question, that answer waited about 2.5 minutes on a 100-meeting
  archive. Transcript passages are reindexed from the settings button.
- Helpers stop in Wails' `OnShutdown`, not a `defer`: quitting ends the process
  before `app.Run` returns. A helper left by a crash is killed when the next
  one for the same file starts.

Evidence: `exp/19_local_llm`, `exp/out/N-local-llm.jsonl`,
`.spec/spikes/performance-local-llm-spike.md`, and
`TestLocalModelsSummariseAndFindOnThisMac` (`MT_TEST_LLAMA=1`).

## GitHub Copilot

- GitHub Models, the OpenAI-compatible inference API, was retired on
  2026-07-30. The supported route is the Copilot SDK, which drives the Copilot
  CLI. Reverse-engineered Copilot API proxies are not an option.
- The CLI is downloaded when Copilot is chosen, pinned to 1.0.85 — the version
  SDK v1.0.14 was released against — and always run with `--no-auto-update`;
  without it the binary hands over to whatever newer copy the person has.
- Sign-in is the CLI's own `copilot login`, which opens the browser. Its state
  lives in `~/MeetingTranscriber/copilot`, apart from the person's own Copilot
  history; without it the CLI falls back to the `gh` login.
- Sessions run in `ModeEmpty` with no tools, every permission refused, and a
  replaced system prompt. The SDK has no schema option, so the schema goes in
  the prompt and the JSON is taken from the reply. Each session is deleted
  after its answer.
- No model chosen means Copilot's own `auto`; left empty, the CLI would use its
  default model instead. The account's model list leaves Auto out for that
  reason.
- `SendAndWait` keeps only the English text of a failure. A spent quota is
  recognised by the session error event's `errorType` and reported with what to
  do. Once the monthly quota is spent every model is refused, GPT-5 mini
  included (checked 2026-09-28).
- Copilot bills AI Credits per token. Business and Enterprise need the Copilot
  CLI policy enabled. The SDK has no embeddings; search uses OpenAI or local.

## Recording strip

- While a meeting (or a recording started by hand) is in progress, a capsule
  hangs under the menu bar, centred on the primary display: a reminder to tell
  the others, then the latest live line, with Pause and Stop. It shows over
  every Space and full-screen app and is excluded from screen capture.
- It is a non-activating panel at status level. Wails' notch window was
  rejected: it makes itself key on hover and takes the keyboard from the call.
- WebKit swallows the first click on an inactive panel, then makes the panel
  key. `strip_darwin.go` answers `acceptsFirstMouse:` YES and
  `needsPanelToBecomeKey` NO for the strip's web view alone, through a
  class-level swizzle gated by an associated object. Giving the view a class of
  its own crashed: WebKit observes it with KVO, whose generated class must stay
  its class.
- It is placed by plain X/Y from the primary display's work area. Wails beta.16
  divides `Options.Screen` coordinates by the Retina scale a second time.
- It is created when the recording starts and closed when it ends, so the main
  window stays the last one to close.
- Pause holds the file open. Nothing heard while held is written, transcribed
  or kept for a later preroll, and the detector stands still so a pause cannot
  end the meeting. Stop while held still stops.

## Packaging

- Run and distribute the `.app`, not the bare Go executable.
- macOS privacy grants belong to the application identity.
- The bundle includes `audiotee`, `llama-server`, and sherpa dynamic libraries.
- Local signing stabilizes developer permissions; public distribution still
  requires Developer ID signing and notarization.
