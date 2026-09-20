---
"@sightmap/jev-turbo": patch
---

`--distill-check` on `explore` and `bench`: after a goal is reached, Jev is asked which part of the final URL and which text on the final page prove it, the answers are composed into a `done_when`, and the proposal is scored against every page the run passed through, since a check that also holds on an earlier page would have stopped the run there. When it still holds on one, Jev is asked for a text that page does not show, up to twice. Over the saucedemo suite and the IKEA variants, 9 of 13 proposals held at the end and failed on every earlier page; the run file carries the proposal under `distilled`.
