# 22 — detailed summaries

`go run ./exp/22_summaries -db copy.db -meetings 6 -notes 1 -min 20 -out …`
on a copy of the owner's archive, 2026-10-31. Model: GitHub Copilot
`gpt-6-sol`. "previous" is the prompt that shipped in 1.5.0 (overview of two or
three sentences, one sentence per chapter); "shipping" is the detailed one.
Recordings: meetings 293, 294, 298, 299, 300, 301 (20 minutes or longer, 254
minutes in all) and note 309 (60 minutes).

## Sizes

| | previous | shipping, run 1 | shipping, run 2 |
|---|---|---|---|
| overview, characters per meeting | 288 | 748 | 759 |
| chapter write-ups, characters per meeting | 589 | 3545 | 3518 |
| the same per minute of meeting | 14 | 84 | 83 |
| chapters per meeting | 4.8 | 5.3 | 5.2 |
| decisions / actions / questions per meeting | 1.5 / 1.0 / 2.0 | 1.2 / 1.0 / 2.0 | 1.5 / 0.8 / 2.3 |
| SPEAKER labels in the prose | 0 | 0 | 0 |
| seconds per meeting | 14 | 22 | 24 |
| note: chapter write-ups, characters | 324 | 1319 | 1280 |

## Blind comparison

The same model read each transcript with both write-ups, the order
alternating, and named the one that tells a colleague who missed the meeting
more, and every statement the transcript does not support.

| | run 1 | run 2 |
|---|---|---|
| shipping judged better | 7 of 7 | 7 of 7 |
| unsupported statements, previous | 0 | 0 |
| unsupported statements, shipping | 6 | 3 |
| … of them in the prose | 2 | 0 |
| … in the note's decisions | 1 | 0 |
| … topic labels reused from the archive | 3 | 3 |

Between the runs the prompt gained three rules: never fill in what a vague
phrase leaves out, never state an inference as something said, a note's
decision is one the speaker said; and a topic from the archive's vocabulary
must cover the recording, not share a word with it. The topic labels judged
wrong are all reused from that vocabulary (reuse rose from 17 of 23 to 23 of
25 topics), the weak spot left.

## GPT-5.6 Terra, 2026-11-01

The owner mostly uses Terra. GPT-6, 6.1 and 6.2 Terra are refused by CLI
1.0.85 and by 1.0.105 for the owner's account ("Model … is not available",
`exp/23_copilot_models -try`); GPT-5.6 Terra answers, at about the same price
per token as GPT-6 Sol. Written by `gpt-5.6-terra`, judged by `gpt-6-sol` so
that the model does not mark its own work; the same seven recordings. Run 2
adds two rules: numbers as they were said, never one worked out ("5 seconds
instead of 12", not "twice as fast"), and what was said about one person stays
about that person.

| | previous | shipping, run 1 | shipping, run 2 |
|---|---|---|---|
| overview, characters per meeting | 280 | 868 | 832 |
| chapter write-ups, characters per meeting | 618 | 4220 | 4108 |
| the same per minute of meeting | 15 | 100 | 97 |
| chapters per meeting | 5.2 | 5.5 | 4.3 |
| decisions / actions / questions per meeting | 3.2 / 1.2 / 1.5 | 2.5 / 1.2 / 1.5 | 2.3 / 1.3 / 2.0 |
| seconds per meeting | 13 | 26 | 27 |
| shipping judged better | | 7 of 7 | 6 of 7 |
| unsupported statements, previous | | 6 | 9 |
| unsupported statements, shipping | | 11 | 4 |
| … of them in the prose | | 9 | 2 |
| … owners given to people who did not take the task | | 0 | 2 |
| … topic labels from the archive | | 2 | 0 |

Run 1's prose errors were ratios the model worked out itself (a "tenfold"
that the figures did not give), something said of one person widened to
three, and a misheard word taken for a surname; run 2 had none of these. The
judge is noisy: it found 6 unsupported statements in the previous summaries
once and 9 in the same texts the second time, so differences of two or three
are within it.
