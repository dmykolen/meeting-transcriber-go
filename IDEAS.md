# Ideas

Candidate features for the standalone app, and — just as important — the ones
that were put to the owner and turned down. Read the rejected list before
proposing anything: an idea that was refused once and comes back is worse than
no idea at all.

`PLAN.md` is what is being built and what is done. This file is what has not
been decided yet.

Ratings are ●●●●● out of five. **Value** is whether it changes how the app is
used; **cost** is the work in this codebase specifically.

## Refused

Do not propose these again, in any wording.

| Idea | Why it was refused |
|---|---|
| Two microphones in one room | Not interesting to the owner |
| Redaction before the model | Not interesting to the owner |
| A follow-up message to send after a meeting | Redundant. The summary already holds the decisions, the owners and the open questions, and Copy as Markdown already puts them on the clipboard. The team is on Teams, whose API is not available, so it would be copied by hand either way — which is what the app already allows. Proposed out of product-pattern reflex, which is the failure mode the owner warned about |
| Export to SRT, VTT or DOCX | Asked for repeatedly and refused repeatedly |
| A decision nobody told the person it affects | Refused outright — "єрунда" |
| Who the project is waiting on, and for how long | Refused outright — "єрунда". Note the distinction: *blocked on* as a field of one task was not refused, and lives inside the project state. Do not bring back a screen that lists people the project is waiting on |

## Agreed, not yet built

| Idea | Shape the owner asked for |
|---|---|
| **Dictation mode** | One button and a hotkey for both start and stop; after stopping, choose a project or leave it loose; the model tidies what was dictated into clean text |
| **MCP server** | The owner likes it and considers it obvious, so it is scheduled rather than pitched. Meetings become context for Claude Code and anything else that speaks MCP |
| **The project page** | The big one. A living, model-maintained state of a project across all of its meetings — deduplicated, current, and changing as meetings happen. Design written up in [PROJECTS.md](PROJECTS.md); the owner asked for the plan before any code, and said the interface matters as much as the machinery and must not reuse the rest of the app's pattern |
| **Edit an action item** | Reword, reassign, re-date, delete. A model's guess at an owner is a guess |
| **The strip above every window during a call** | Approved as proposed |
| **The week as a shape, not a list** | Approved, with the owner saying he does not fully picture it yet and expects it to be made functional and good-looking |

## Waiting on a decision

### The app knows things nothing else does

| # | Idea | Validation — why this is not something the app already does | Value | Cost |
|---|---|---|---|---|
| 1 | **Your own vocabulary instead of the same mistake** | The transcript cannot be edited at all, and `SetInitialPrompt` — which whisper.cpp's Go binding does expose — is unused. Corrections accumulate into a vocabulary of your people, products and systems and are given to the model *before* the next recording, so the mistake stops happening rather than being repaired | ●●●●● | ●●○○○ |
| 3 | **The history of a decision, including its reversal** | Summaries hold decisions inside one meeting. Nothing links them across time or notices that one was undone. "Why did we do it this way" is the most expensive question in an organisation | ●●●●● | ●●●○○ |
| 4 | **"This meeting could have done without you"** | Analytics measures share within one meeting. This aggregates: hours a month in meetings where you said under a minute and nothing was assigned to you. Not a vanity metric — grounds for declining the next one | ●●●●● | ●●○○○ |
| 5 | **"You have not spoken to Marta since 12 August, and she owes you this"** | Today shows overdue by date. This is a different axis: when you were last in a meeting with a person at all | ●●●●○ | ●●○○○ |
| 6 | **In-person meetings stop being invisible** | Notes are deliberately never diarized, so a meeting held in a room looks like a monologue. If the microphone confidently carries two voices, it is not a note. Closes a limitation the app currently just admits to | ●●●●○ | ●●●○○ |
| 7 | **Names in the live transcript** | Live shows only "you" and "them". Voiceprints are already computed and nobody asks them during the call | ●●●●○ | ●●●○○ |
| 8 | **Click a word, hear that word** | `SetTokenTimestamps` is available and only the bounds of a turn are kept. When an argument is about one word, forty seconds around it is not the answer | ●●●●○ | ●●○○○ |

### Running a project

Everything that was here has been folded into one thing — see
[PROJECTS.md](PROJECTS.md). The owner's reading was that these were not seven
features but one: a project page that keeps the project's actual state.


### Interface

| # | Idea | Validation | Value | Cost |
|---|---|---|---|---|
| 16 | **Skim mode** | The density toggle changes type size inside the same layout. This changes what is shown: questions, commitments and numbers, with everything else folded and expandable around them. Nine hundred rows are unreadable as they are | ●●●●● | ●●○○○ |
| 17 | **⌘K as a real command palette** | ⌘K currently opens the Search screen, which searches *inside transcripts*. A palette searches the app: a meeting, a person, a project, an action, a command | ●●●●○ | ●●○○○ |
| 19 | **A speaker ribbon down the side of a transcript** | The analytics card draws word density across time; there is no map of *who* was speaking when, and that is what navigation is actually done by | ●●●●○ | ●●○○○ |
| 20 | **Reading mode** | Compact and roomy are both scanning layouts. Reading a meeting properly wants a wider measure, larger type and names in the margin, the way a play is printed | ●●●○○ | ●●○○○ |
| 22 | **Keyboard through a transcript** | Only ⌘F exists. j and k between turns, space to play the turn under the cursor, n and p between speakers | ●●●○○ | ●●○○○ |
| 26 | **The card becomes the meeting** | Opening a meeting fades one screen into another. Motion's `layoutId` can carry the title from the card into the header, so the transition explains where you went | ●●●○○ | ●●○○○ |
