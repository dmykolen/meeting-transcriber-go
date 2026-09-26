# Roadmap

This file contains only unresolved product work and decisions that should not be
re-proposed.

## Next

1. **Vocabulary and transcript correction**
   - Let a person correct names, products, and technical terms.
   - Reuse corrections on later meetings.
   - The old prompt-only glossary experiment failed; the implementation needs a
     new measured approach that does not restore looping context.

2. **Dictation mode**
   - One button and hotkey start and stop.
   - After stopping, choose a project or leave the note unfiled.
   - Optionally clean dictated text without making AI mandatory for capture.

3. **Local LLM option**
   - Add a local alternative behind the existing `internal/insights` boundary.
   - Preserve OpenAI support and local-only core behavior.
   - Measure model quality, memory use, download size, and latency before
     choosing a runtime or default model.

## Later, after evidence

- Decision version history and a true project time machine.
- Structured source provenance for each generated summary claim.
- Word-level playback after mapped token timestamps are proven.
- Names in the live transcript.
- Durable markers created during a live recording.
- Better mic-only recognition of in-person meetings.
- Search benchmarks, pagination, and virtualization for very large archives.
- Background semantic-index progress.
- Meeting-efficiency insights based on measured participation and assignments.
- A compact always-on-top strip above other windows during a call. It was
  approved, but needs a native multi-window design and interaction pass.

## Completed from the original backlog

- ~~Built-in read-only MCP server and client setup UI.~~
- ~~Living project state with stable IDs, rebuild, provenance, and pinned edits.~~
- ~~Editable action items.~~
- ~~Combined Meetings workspace with timeline, reader, voice map, and project dock.~~
- ~~Real command palette.~~
- ~~Exact and semantic knowledge search across meetings, projects, and notes.~~
- ~~Ask with typed sources and direct navigation to evidence.~~
- ~~Manual note deck and agenda-note workflow.~~
- ~~Summary preview, comparison, acceptance, and undo.~~
- ~~Trash, restore, and empty-bin flows.~~
- ~~Responsive workspace drawers, focus mode, and reduced-motion support.~~

## Do not propose again

- Two microphones in one room.
- Redaction before the model.
- A generated follow-up message that duplicates the existing summary/Markdown
  export.
- SRT, VTT, or DOCX export.
- A warning that a decision affects someone who was not told.
- A dashboard that scores who a project is waiting on.
- Invented project health, velocity, burndown, or Gantt data.
