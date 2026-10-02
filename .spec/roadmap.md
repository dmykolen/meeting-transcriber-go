# Roadmap

This file contains only unresolved product work and decisions that should not be
re-proposed.

## Next

The voice-recognition fix comes next (owner, 2026-09-29).

1. **Voice recognition after the speaker-model switch** (found by
   `exp/20_voices`, 2026-09-29)
   - Voices enrolled before the switch keep only 512-wide prints.
     `store.Cosine` returns -1 across widths, so those people are never
     recognised again, and nothing says so. On the owner's archive this hit
     two regular colleagues. The fix could be as simple as flagging them in
     Settings for one new naming.
   - The recogniser enrols its own matches (`Remember` after `Recognise`) at
     `Match` 0.55. 13% of different-people pairs clear that bar across
     meetings, and the centres of three enrolled people are now 0.78–0.84
     alike, so names may drift. Measure against names the owner confirms
     before changing the threshold.
   - `mine()` can leave the owner's print identical to a far-side label's
     print (10 current-model meetings).

2. **Vocabulary and transcript correction**
   - Let a person correct names, products, and technical terms.
   - Reuse corrections on later meetings.
   - The old prompt-only glossary experiment failed; the implementation needs a
     new measured approach that does not restore looping context.

3. **Dictation mode**
   - One button and hotkey start and stop.
   - After stopping, choose a project or leave the note unfiled.
   - Optionally clean dictated text without making AI mandatory for capture.

4. **Local model quality**
   - The local option ships with a small, unmeasured default (Gemma 4 E2B).
     Its first run chaptered everything at 0 s and dropped a spoken deadline.
   - Measure current candidates on several anonymized real transcripts,
     including project-update operations. The spike's list predates Gemma 4,
     Qwen3.5+ and the Ukrainian MamayLM v2.
   - See [the measured spike](spikes/performance-local-llm-spike.md).

5. **GitHub Copilot end to end**
   - Start, sign-in state, the model list and errors are verified; a
     successful summary is not, because the test account was over its monthly
     quota.

## Proposed, measured — owner to decide

Measured on a copy of the owner's archive on 2026-09-28 (109 meetings, 42.8 h,
1–25 September; read-only aggregates).

1. **Name recurring voices once, across the archive — blocked by evidence**
   - The pain is real: unnamed voices speak 53% of the time, and 114 action
     items are owned by `SPEAKER_xx`.
   - `exp/20_voices` (`exp/out/20-voices.md`) found the premise wrong. The
     first estimate compared 512-wide prints of the old speaker model with
     192-wide ones of the current model. With the current model alone, the
     same person across meetings scores a median 0.45 against 0.40 for
     different people. Within one meeting the model separates voices well:
     0.83 against 0.36.
   - Applying one name to a whole cluster is unsafe at every tested
     threshold: two of the five largest groups hold other people's named
     voices. It could work only as confirm-by-ear, then apply at 0.70 or
     above, with the rest shown as suggestions.
   - Fix the recognition defects below first.

2. **The strip reacts to the owner's name**
   - Others say the owner's name in 47 of 109 meetings: 185 turns, 35 of them
     questions (an upper bound: namesakes count too).
   - The strip highlights the sentence with the name and the line before it.
   - Risk: the live scribe drops an utterance while the previous one is still
     being transcribed. First step: replay recorded meetings through the live
     path and measure recall and delay.

3. **"What did I miss?" in the strip**
   - Three lines on the last five minutes of the live transcript, through the
     chosen AI. With 2, it gives the context of the question.
   - Risk: rough live text and a weak small local model; measure on the same
     replayed meetings.

4. **Memory during a meeting (a bet)**
   - When the conversation returns to a topic with a past decision, the strip
     shows that decision with a link.
   - No evidence yet: replay past meetings in order and count useful against
     noisy hints before building anything.

## Later, after evidence

- Decision version history and a true project time machine.
- Structured source provenance for each generated summary claim.
- Word-level playback after mapped token timestamps are proven.
- Names in the live transcript.
- Durable markers created during a live recording.
- Better mic-only recognition of in-person meetings.
- Search benchmarks, pagination, and virtualization for very large archives.
- Meeting-efficiency insights based on measured participation and assignments.

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
- ~~AI from OpenAI, GitHub Copilot, or a local model, and local search vectors.~~
- ~~Always-on-top recording strip: tell the others, pause, stop.~~
- ~~Background semantic-index progress with named phases.~~
- ~~Interface in Ukrainian and English, switched in Settings.~~
- ~~Transcription schedule: right after a recording, at a set time, or when the Mac is free.~~

## Do not propose again

- Two microphones in one room.
- Redaction before the model.
- A generated follow-up message that duplicates the existing summary/Markdown
  export.
- SRT, VTT, or DOCX export.
- A warning that a decision affects someone who was not told.
- A dashboard that scores who a project is waiting on.
- Invented project health, velocity, burndown, or Gantt data.
- Developer ID signing and notarization.
- Keeping the API keys in the Keychain.
- Guessing speaker names from how people address each other: first names are
  in 2% of turns, and 2 of 445 unnamed voices get a consistent candidate.
- Detecting "off the record" phrases: none in 109 meetings.
- Deadline reminders for action items: 22 of 369 items have a due date, 4 of
  them the owner's.
- Closing action items from what later meetings say: the project fold already
  can, and it closed 1 of 132.
