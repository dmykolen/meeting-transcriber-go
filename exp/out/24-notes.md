# 24 — voice notes

`go run ./exp/24_notes -db copy.db -out … -judge gpt-5.6-terra` on a copy of
the owner's archive, 2026-11-01. Written by GitHub Copilot `gpt-6.1-sol`, the
model in the owner's settings, and judged by `gpt-5.6-terra`, so that a model
does not mark its own work. "previous" is the note prompt that shipped in
1.6.0; "shipping" is `insights.Polish`.

## Made-up dictations

Six dictations in `exp/24_notes/cases.json`, nothing private: deadlines said
relative to the day ("tomorrow", "on Wednesday", "by the 15th"), corrections
made while speaking ("tomorrow, no, Friday"), an idea and a thought that hold
no to-do, and a vague time that must stay words. Checked by code: each to-do's
date, the number of to-dos, words that must stay and a corrected word that
must go.

| | first wording | shipping wording |
|---|---|---|
| runs | 3 | 2 |
| checks passed | 18 of 18 | 12 of 12 |

## The archive's notes

The 29 notes with at least 80 characters of speech; seven of them are
hour-long parts of one long recording of a conversation.

| | previous | shipping, run 1 | shipping, run 2 |
|---|---|---|---|
| text, characters per note | 884 | 2,921 | 952 |
| longest text, characters | 3,161 | 13,732 | 2,859 |
| to-dos (with a date) | 29 (0) | 27 (2) | 27 (2) |
| meeting words in the text ("discussed", "speaker", …) | 19 | 61 | 22 |
| `SPEAKER_` labels in the text | 0 | not counted; two notes came out as dialogues | 0 |
| seconds per note | 14 | 23 | 12 |
| judged better: shipping / previous / tie | | 19 / 6 / 4 | 21 / 7 / 1 |
| unsupported statements: previous / shipping | | 3 / 1 | 5 / 4 |
| dated to-dos judged wrong | | 0 of 2 | 0 of 2 |

Between the runs the prompt gained three rules: a note reads in two minutes,
about 400 words at most however long the recording; one voice split into
several labels is still one person, and a note is never a label or a dialogue;
whatever can be understood of a garbled recording is kept rather than the
whole declared unusable. Run 2 lost five of the seven hour-long conversation
parts, where the judge preferred the previous format's separate decisions and
questions, and two short notes. The judge varies by two or three statements
between runs on identical texts.

## In the packaged app

A sandbox copy of the app, with its microphone playing a synthesized
dictation, on the "when the Mac is free" schedule: the queue took the note
11 ms after it stopped, transcribed it in 1 s, and the model wrote it in 9.4 s.
Whisper heard the synthesized "п'ятницю" as "татницю", and that to-do stayed
without a date rather than get a guessed one.
