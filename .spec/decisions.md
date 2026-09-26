# Engineering decisions

These are measured or product-critical choices that are easy to accidentally
“simplify” into a regression. Re-test before changing them.

## Local-first boundary

- Transcription, diarization, playback, analytics, keyword search, and storage
  are local.
- OpenAI is optional and isolated in `internal/insights`.
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

## Packaging

- Run and distribute the `.app`, not the bare Go executable.
- macOS privacy grants belong to the application identity.
- The bundle includes `audiotee` and sherpa dynamic libraries.
- Local signing stabilizes developer permissions; public distribution still
  requires Developer ID signing and notarization.
