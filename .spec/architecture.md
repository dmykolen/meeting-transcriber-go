# Architecture

This is the compact map of the production application. Code and tests remain
the source of truth.

## Runtime

Meeting Transcriber is one macOS application:

- Wails embeds the React frontend into the Go executable.
- The same process owns capture, processing, SQLite, and the MCP server.
- `audiotee` is bundled as a helper for macOS system-audio capture.
- sherpa-onnx dynamic libraries are included in the `.app`.
- Models and FFmpeg are downloaded on first run into the app home directory.

Default app data:

```text
~/MeetingTranscriber/
├── config.toml
├── meetings.db
├── recordings/
├── models/
├── bin/
├── cache/
└── logs/mt.log
```

`MT_HOME` replaces this root.

## Composition

`main.go`:

1. opens logging;
2. loads config;
3. opens SQLite;
4. creates setup, library, and service objects;
5. starts the loopback MCP server;
6. opens the Wails window immediately;
7. downloads and loads models in the background;
8. starts the listener, retention, and processing queue when models are ready.

The window must not wait for model imports, downloads, or initialization.

## Packages

| Package | Ownership |
|---|---|
| `internal/home` | App paths, config, defaults |
| `internal/models` | Downloadable model/tool manifest and first-run fetching |
| `internal/audio` | Microphone and system-audio devices, stereo stream |
| `internal/listen` | VAD, preroll, meeting detection, recording, live transcript |
| `internal/media` | Decode, channel alignment, fold, playback cache, waveform |
| `internal/engine` | ASR, diarization, embeddings |
| `internal/store` | SQLite schema, queries, FTS, people, notes, projects, knowledge |
| `internal/insights` | Optional OpenAI summaries, embeddings, Q&A, project updates |
| `internal/library` | Processing queue and transcript-derived artifacts |
| `internal/service` | Wails methods and MCP tools |
| `frontend/src` | Product UI |

Do not duplicate package ownership. `internal/insights` is the only outbound AI
boundary.

## Audio and processing flow

```text
microphone ─┐
            ├─ stereo WAV ─ decode/align/fold ─ ASR ─ transcript
system tap ─┘                            └────── diarization/voices
                                                    │
SQLite ← transcript + voice evidence + analytics ←─┘
  │
  ├─ keyword passages
  ├─ optional embeddings
  ├─ optional summary
  └─ optional project-state update
```

The left channel is the laptop microphone. The right channel is system audio.
Channel ownership is used for note/meeting classification, local-speaker
attribution, echo handling, and far-side diarization.

The queue is single-worker. Active recording prevents the queue from starting a
new historical job; a job already in flight is not preempted.

## Storage

SQLite stores:

- recording metadata and processing status;
- transcript turns and summaries;
- action items and manual notes;
- full-text and semantic-search data;
- people, voiceprints, and voiceprint provenance;
- projects, deterministic standing, and model-maintained kept state;
- deletion and retention metadata.

Audio remains on disk. Retention deletes audio only; the meeting record remains.

## Projects

A project has two complementary views:

- `Standing`: deterministic aggregation of meetings, actions, decisions,
  questions, and people;
- `Kept`: model-maintained state updated by operations against stable IDs.

Project updates are incremental. `Rebuild` discards the model document and
replays meetings in order. Human-edited items are pinned and cannot be silently
rewritten by the model.

## Search and Ask

- Exact search is local.
- Semantic search uses optional embeddings cached in SQLite.
- Ask receives typed sources from transcripts, summaries, notes, actions, and
  projects.
- Source UI links back to the owning document or transcript moment.

OpenAI is optional. Exact search and the stored archive remain usable without
it.

## MCP

The desktop process serves Streamable HTTP on
`http://127.0.0.1:8765/mcp`. The same executable supports `--mcp-stdio` for
Claude Desktop.

MCP reads through the same `service.Meetings` and SQLite database as the UI.
Tools are read-only and never return secrets or raw voiceprint vectors.

## Frontend

The main rail owns Today, Meetings, Search, To do, Ask, and Settings.

The Meetings workspace combines:

- timeline;
- chronological recording list;
- transcript/summary reader;
- speaker and meeting analytics;
- notes;
- project dock and project document.

Sample-data mode is explicit:

```bash
cd frontend
VITE_DESIGN=1 npm run dev
```

Production builds do not include the sample adapter branch.

## Build and packaging

```bash
make          # build/mt
make bundle   # build/Meeting Transcriber.app
make dmg      # build/MeetingTranscriber.dmg
make test     # whisper.cpp + go vet + go test
```

`whisper.cpp` and `audiotee` are pinned and built under `build/`. The directory
is ignored and must be reproducible from a clean checkout.
