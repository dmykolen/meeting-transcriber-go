# The project page

A plan, written before any code, because the owner asked for one and because
the hard parts here are decisions rather than typing.

## What this is, and what it is not

A per-meeting summary is a **snapshot**. A project is not the concatenation of
its snapshots: things get decided and then reversed, a task is promised in one
meeting and finished in another, the same commitment is worded three different
ways in three meetings, and a question asked in March is answered in May by
somebody who never saw it asked.

So the project page is not "all the summaries, one after another". It is a
**single living document that every meeting updates**, and the model's job is to
apply each meeting to it — not to re-read the pile.

The owner's words: *see the current state of the project, built and changed
dynamically from meeting to meeting, with no duplicates in the topics or the
to-do list.*

## The shape of it

### State, not text

Free prose cannot be deduplicated, diffed or edited. The project's state is
structured, and every part of it carries where it came from.

```
Project
  status      one paragraph, rewritten as things change
  work        Item[]      what anybody committed to
  decisions   Decision[]  what was settled, and what overturned it
  questions   Question[]  raised, and answered or not
  people      who is actually in this, from the voices
  updated     which meeting last changed it
```

```
Item                                  Decision
  id        stable, never reused        id
  what      one line                    what
  owner     a person, or nobody         when      the meeting that settled it
  due       as it was said              supersedes  an earlier decision, or none
  state     open | done | dropped
  blocked   what it is waiting on, in the words it was said in
  first     the meeting it was promised in
  last      the meeting that last touched it
  history   Touch[]  meeting, what changed, when
  pinned    a person edited this; the model may close it, never reword it
```

**Provenance is not optional.** Every item points at a meeting and a timestamp.
Without it the page is an unfalsifiable blob, and the first time it is wrong
about something nobody will trust it again. With it, every line is one click
from the moment it was said — which the app can already play.

### The update is a set of operations, not a new list

This is the crux, and it is what makes deduplication work.

The model is **not** asked "summarise this project". It is given the current
state — with the ids — and one meeting's summary, and asked for **operations**:

```
add item        what/owner/due
update item     id, what changed
close item      id, done or dropped, and why
decide          what, supersedes?
answer question id, the answer
restate         id — this meeting said the same thing again, in other words
status          the new paragraph
```

Because the model sees the existing items and answers with their ids, "узгодити
ролі з безпекою" said again in the next meeting comes back as `restate 14`
rather than as a fifteenth item. Asking for a fresh list every time is what
produces duplicates, and no amount of prompting fixes it.

`restate` is worth having on its own: an item restated three times is one nobody
is doing, and that is visible on the page without any extra machinery.

### Incremental, and rebuildable

`state(n) = apply(state(n-1), meeting(n))`

- One model call per meeting, per project, on the meeting's **summary** rather
  than its transcript — a few hundred tokens, not tens of thousands.
- The answer is stable: yesterday's page does not change because the model felt
  different today.
- It yields **what this meeting changed**, free, which is the most interesting
  thing on the page.

The risk is drift: an error early on is carried for ever. Two answers, both
needed:

1. **Rebuild** replays every meeting from the start against an empty state.
   Deterministic, costs one call per meeting, and is the answer to "this has
   gone wrong".
2. **Provenance makes drift visible.** An item whose last touch was in March,
   in a project that met yesterday, is either forgotten or wrong, and the page
   says which meeting to go and check.

### Human edits win

The owner is getting the ability to edit and delete to-do items. So the state
has human writing in it, and the model must not overwrite it.

The rule: **an item a person has edited is pinned.** The model may close it,
reference it, or mark it restated. It may not reword it, change its owner, or
delete it. Anything else and the app quietly undoes the user's work, which is
the fastest way to lose their trust in the whole page.

### When it runs

After a meeting is summarised **and** filed into a project. Filing later applies
it then — so dragging a meeting onto a project updates that project's state,
which is a satisfying thing to watch happen.

A project with no key gets everything except the model's part: meetings, people,
colour, the raw action items from each summary. The page must be useful without
a key, in the same way the rest of the app is.

## The interface

The owner was explicit: this page must not reuse the rest of the app's pattern,
and it must not look plain. So the first decision is what it *is*, and the
answer is not a dashboard.

**A dashboard is the trivial answer** — four stat tiles and three lists. Today
already is that, and doing it again here would be the failure the owner named.

**This page is a document with a time machine.**

```
┌──────────────────────────────────────────────────────────────────────┐
│  ▎Northwind                                    ● 4 people   ⌘K  ⋯     │  colour spine
│   Where it stands                                                    │
│   The migration is agreed and waiting on the security review.        │  status, prose,
│   Access stays behind the VPN. Two things are late.                   │  large, readable
│                                                                       │
│  ───────────────────────────────────────────────────────────────────  │
│                                                                       │
│   TIMELINE            │  WORK                                        │
│   ▏                   │  ○ Узгодити перелік ролей       Dmytro  ⚑3   │
│   ▏● 12 Sep  +2 ~1    │    said again three times · last 12 Sep      │
│   ▏                   │  ○ Матеріали для Northwind       Marta  late  │
│   ▏● 5 Sep   +1 ✓2    │  ✓ Закрити доступ ззовні        done 5 Sep   │
│   ▏                   │                                              │
│   ▏● 29 Aug  +4       │  DECIDED                                     │
│   ▏                   │  • Доступ лишається через VPN     5 Sep      │
│   ▏● 22 Aug  start    │  • ~~Відкрити назовні~~ overturned 5 Sep     │
│                       │                                              │
│                       │  STILL OPEN                                  │
│                       │  ? Які IP-діапазони               ×3 · 22 Aug│
└──────────────────────────────────────────────────────────────────────┘
```

What makes it worth building rather than another list of things:

**The timeline is a scrubber, not decoration.** Click 29 August and the whole
right-hand side becomes the project *as it was that day*. Nothing else in the
app can do that, and no per-meeting summary ever will. Drag along it and watch
tasks appear, get owners, and close.

**Every meeting shows what it changed.** `+2 ~1` — two things started, one
closed. That is the diff the incremental update gives away for free, and it
answers "what actually came out of that hour".

**The status is prose and it is the largest thing on the page.** Not a metric
grid. Somebody who opens this wants a sentence they can repeat to their manager.

**Everything is traceable in one click.** Any item, decision or question opens
the meeting at the second it was said, and plays it. The app already has the
player, the waveform and word-level timing.

**The restate count is the honest metric.** Not burndown, not velocity — how
many times a thing has been promised again. It requires no estimation and it
cannot be gamed.

**Editing is inline and immediate.** Click the text of a task and type. It pins.
No dialog, no form, no save button.

**The colour is the project's, and it runs down the page** as a spine, in the
tab, on every card in the Library. One glance says which project you are in.

### What is deliberately absent

- No burndown chart, no velocity, no completion percentage. Nothing here is
  estimated, so any of those would be invented.
- No Gantt. There are no dependencies in the data, and inventing them would be
  the app pretending to know something.
- No "health score". The page shows what is late and what keeps being restated,
  which is the same information without a number nobody can audit.

## Order of work

1. **Colour and rename.** A project gets a colour and can be renamed; a meeting
   can be renamed. Small, visible, and the colour is needed by everything below.
2. **Editing.** Items become editable and deletable, with `pinned`. This has to
   exist before the model starts writing them, or the first edit gets eaten.
3. **The state, without a model.** Store the structure, and fill it by simply
   collecting each meeting's summary into it. Deduplication is absent at this
   stage and the page is already useful — this is the version that works with
   no key.
4. **The updater.** One call per meeting, operations against ids, provenance,
   `restate`, rebuild.
5. **The page.** Status, work, decisions, questions, timeline with diffs.
6. **The time machine.** Scrubbing the timeline to see the state as of a date.
   Needs nothing new — the operations are already an event log.

Steps 1 and 2 are days. Steps 3 to 6 are the feature.

## What could go wrong, and what is done about it

| Risk | What is done |
|---|---|
| The model invents a task nobody promised | Every item carries the meeting and moment; a wrong one is one click from being disproved and deleted |
| Errors accumulate over months | Rebuild replays from the start; provenance shows stale items |
| The same thing appears twice anyway | Operations reference ids, and `restate` is a first-class answer. If duplicates still appear, the fix is the prompt seeing the existing items, not a similarity threshold |
| The model rewrites what a person typed | `pinned`, enforced in code and not by asking the model nicely |
| A long project costs a lot | One call per meeting on a summary, not a transcript. A two-year project is a few hundred small calls over two years |
| No key | Steps 1 to 3 work without one; the page degrades to collected summaries rather than disappearing |
| A meeting belongs to two projects | Not supported at first. A recording has one `folder`, and changing that is a schema decision worth making on evidence rather than in advance |
