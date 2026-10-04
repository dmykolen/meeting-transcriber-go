# AGENTS.md

This file applies to the entire repository. It is the operating contract for
coding agents working on Meeting Transcriber.

## Product

Meeting Transcriber is a local-first macOS desktop app. It records microphone
and system audio, transcribes locally, separates and recognises speakers, keeps
meeting and project knowledge in SQLite, and exposes the archive through a
read-only MCP server.

Stack:

- Go 1.26 and Wails v3
- React 19, TypeScript, Vite, Tailwind CSS, Motion, and Lucide
- SQLite
- whisper.cpp and sherpa-onnx
- optional OpenAI, GitHub Copilot, or local `llama-server` calls behind
  `internal/insights`

The owner communicates in Ukrainian. Reply in Ukrainian. Keep code, comments,
documentation, logs, and technical identifiers in English. The product UI is
Ukrainian or English, chosen in Settings. Ukrainian is the source: write
interface text in Ukrainian through `t()` and add its English to
`frontend/src/en.ts`, which the compiler checks.

## Source of truth

Use this order when information conflicts:

1. The current user request.
2. Running code and tests.
3. This file.
4. The focused documents in `.spec/`.
5. Current upstream documentation and installed library source.

Do not treat old chat history, generated prose, experiment output, or a stale
comment as truth. Verify behavior in code and, when practical, by running it.

Useful references:

- [Architecture](.spec/architecture.md)
- [Engineering decisions](.spec/decisions.md)
- [Experiments](.spec/experiments.md)
- [Roadmap](.spec/roadmap.md)

## The overriding code rule

Write the smallest amount of code that completely solves the current problem.

- Prefer direct, idiomatic Go and TypeScript.
- Prefer one obvious function over a framework of helpers.
- Do not add factories, registries, base classes, wrappers, adapters, or
  interfaces for requirements that do not exist.
- A helper used once usually belongs inline.
- Do not create a new package merely to move a few lines elsewhere.
- Avoid duplication, but do not invent an abstraction to remove three simple
  repeated lines.
- Use a mature library directly when it already solves the problem.
- Every new dependency must earn its place.
- Optimize for the engineer opening the file tomorrow, not for architectural
  symmetry.

Do not fix unrelated issues or reformat unrelated files. Preserve user changes
in a dirty worktree.

## Research before implementation

Library and platform knowledge becomes stale quickly.

- Check current official documentation, source, and tests before using an
  unfamiliar or version-sensitive API.
- Verify the installed version with `go doc`, source inspection, or TypeScript
  types. Trust the installed runtime over a blog post.
- Search the repository for prior art before adding a new pattern.
- When a technology choice matters, compare realistic alternatives and explain
  the chosen tradeoff briefly.
- Do not claim a performance or quality improvement without measuring it.

## Architecture boundaries

- `main.go` is the composition root.
- `internal/audio` captures microphone and system audio.
- `internal/listen` owns detection, preroll, recording, and live text.
- `internal/media` decodes, aligns, folds, and prepares playback.
- `internal/engine` owns transcription, diarization, and voice embeddings.
- `internal/store` owns SQLite and deterministic derived data.
- `internal/library` owns the processing queue and transcript-derived artifacts.
- `internal/service` is the Wails and MCP application surface.
- `internal/insights` is the only outbound AI boundary.
- `frontend/src` is the product interface.

Share the existing database, library, engine, and service objects. Do not create
parallel stores, background coordinators, or a second source of truth.

The frontend is embedded into the Go executable. MCP is part of that same
executable. Do not introduce a separate daemon or companion binary without an
explicit requirement.

## Non-negotiable runtime contracts

- The app window appears before models are downloaded or loaded. Heavy model
  work stays in the background.
- Transcription, diarization, playback, analytics, and keyword search work
  locally. AI remains optional, and a local model keeps it on the Mac.
- Downloaded models and tools such as FFmpeg and the Copilot CLI live in the
  app-managed home directory. The macOS `audiotee` and `llama-server` helpers
  are deliberately bundled inside the `.app`.
- The microphone is the left channel and system audio is the right channel.
  Do not collapse them before logic that depends on channel ownership.
- Live capture has priority over queued historical processing.
- Audio retention must never remove transcripts, summaries, notes, or project
  state.
- A human edit wins over model output. Pinned project items must not be
  silently rewritten.
- The MCP HTTP endpoint remains loopback-only. MCP tools remain explicit about
  read-only and destructive behavior. Never expose API keys or raw voiceprint
  vectors.
- Run the packaged app through the `.app` bundle. Launching a bare executable
  changes macOS privacy attribution.

Read `.spec/decisions.md` before changing audio alignment, VAD, timestamps,
speaker thresholds, voice matching, model settings, project-state updates, or
startup behavior.

## Error and state handling

Nothing important fails silently.

- Return useful errors at boundaries.
- Do not add broad catches, empty fallbacks, or success-shaped failure values.
- A background worker must survive one failed job and record where it failed.
- Long operations expose a named state and progress where available.
- Startup, downloads, model loading, recording phases, processing phases, and
  MCP lifecycle remain observable in logs or UI.
- Invalid external/request input is rejected explicitly.
- Persisted config is different: malformed or legacy values fall back per
  setting and must not prevent the app window from opening.

## Testing

Test the real path around the expensive boundary.

- Stub the ASR, diarizer, network model, or operating-system boundary—not the
  application method being tested.
- Fakes must reject inputs the real dependency rejects.
- Use realistic configuration, including blank optional values.
- Add regression coverage for bugs tightly coupled to a change.
- Run the narrowest relevant test first, then the broader suite when risk
  warrants it.
- Frontend work is not done after `npm run build`: exercise it in a real
  browser and, for native behavior, in the Wails app.
- Any JavaScript console error is a failure.
- Media-path verification must use formats supported by the test browser.

Core commands:

```bash
make test
cd frontend && npm run build
make bundle
make run
```

## Experiments

Performance, audio quality, ASR behavior, diarization thresholds, and model
choices are decided by experiments, not intuition.

- All experimental programs live in `exp/NN_name/`.
- Ground truth lives in `exp/truth/`, which git ignores: it is made from
  private meetings.
- Reproducible text results live in `exp/out/`. Commit the measured numbers,
  never a transcript dump; `.gitignore` names the dumps.
- Do not commit private meeting audio, transcripts, titles, or the names of
  the people in them. Print recording IDs and speaker labels instead.
- Start with a baseline, vary one thing, run against a real recording and the
  edge case that broke the previous attempt, and record the numbers.
- An experiment may print aggressively and be ugly. Production code may not.
- Move an idea into `internal/` only after the result beats the current behavior
  on explicit acceptance criteria.
- Update `.spec/decisions.md` when an experiment changes a shipping choice.
- Keep failed experiments when they prevent the same dead end from being tried
  again.

See `.spec/experiments.md` before changing the model or audio pipeline.

## UI character

The app is a quiet, exact, unhurried instrument—not a cheerful SaaS dashboard.

- Dark, compact, and native to macOS.
- Violet is the identity color. Cool near-neutral surfaces make it vivid.
- Bright green and coral communicate state; they are not decoration.
- Use Geologica and preserve good Ukrainian typography.
- State facts without judging people.
- No celebratory copy, exclamation marks, “Oops”, or chatty helper text.
- Every wait has a visible name. A spinner-free unexplained pause is a bug.
- Errors explain what happened and what the user can do.
- Prefer fewer boxes, borders, labels, and words.
- Do not create a dashboard of generic metric cards when a document, timeline,
  or direct manipulation communicates the information better.
- A tab does its own job. Do not navigate on the user's behalf.
- Preserve the current workspace model: timeline, recordings, reader, project
  dock, and direct links back to evidence.

## UI interaction rules

- Motion explains a state or spatial relationship; it is not decoration.
- Prefer transform and opacity with short, deliberate easing.
- Avoid bounce, elastic motion, generic fade-up sequences, random particles,
  and gratuitous glass.
- Infinite animations must not block transition completion.
- Respect `prefers-reduced-motion`.
- Hover is an enhancement, never the only way to act.
- Reserve space for contextual actions so content does not jump.
- Destructive actions are quiet and reversible. Prefer bin + undo over a
  confirmation dialog.
- Icon-only controls require accessible names and useful tooltips.
- Preserve keyboard navigation and visible focus.
- Use `<template x-if>` rather than hidden content when absent data would still
  be dereferenced.

For repeated visual patterns, extract a shared primitive only after the pattern
is genuinely repeated. Keep the DOM shallow and the content dominant.

## Frontend workflow

Run the real frontend with sample data:

```bash
cd frontend
VITE_DESIGN=1 npm run dev
```

Use sample data only for visual development. Before finishing:

1. Build the production frontend.
2. Exercise the affected flow at desktop and narrow widths.
3. Check hover, focus, keyboard, loading, empty, success, and error states.
4. Record console errors.
5. Run the packaged app when the change depends on Wails, permissions, audio,
   filesystem behavior, or MCP.

## Documentation

- `README.md` is public product and onboarding documentation.
- `.spec/` contains concise internal architecture, decisions, experiments, and
  roadmap documents.
- `docs/` contains user-facing supporting artifacts.
- Do not create new planning or status Markdown files in the repository root.
- Update an existing canonical document instead.
- `.spec/roadmap.md` is a set of tables. Read all of it before adding a row,
  update a similar row instead of adding a duplicate, and strike through a row
  once it is done.
- Delete historical implementation plans after their durable decisions are
  captured.
- Keep documentation factual, concise, and linked to current paths.

## Open-source hygiene

- Never commit secrets, private recordings, local databases, model files,
  caches, or machine-specific build output.
- Keep the repository buildable from a clean checkout.
- Use pinned native dependencies when reproducibility depends on them.
- Preserve license notices for third-party components.
- Do not add screenshots or binaries larger than necessary.
- Before finishing, inspect `git status`, `git diff --check`, and the final diff.

## Releases

- Bump `CFBundleShortVersionString` and `CFBundleVersion` in
  `packaging/darwin/Info.plist` and the server version in
  `internal/service/mcp.go`, then commit.
- Build from that commit with `make dmg` and copy the image to
  `build/MeetingTranscriber-X.Y.Z.dmg`. Check the version and
  `codesign --verify --deep`; `go version -m` must show the release commit,
  not modified.
- Tag `vX.Y.Z` on that commit, push the branch and the tag, and create the
  release "Meeting Transcriber X.Y.Z" with the DMG attached.
- Notes are short, plain English in three sections: New, Improved, Fixed. Add
  "Known issue" when there is one, and end with the line that the app is not
  notarized. No AI attribution in commits, tags, or notes.

## Definition of done

A task is complete only when:

- the requested behavior is implemented end to end;
- existing behavior is preserved unless intentionally changed;
- the relevant tests and build pass;
- user-visible states and errors are coherent;
- the UI was inspected when UI changed;
- documentation and generated bindings are updated where required;
- temporary files and processes are cleaned up;
- the diff contains no unrelated work.
