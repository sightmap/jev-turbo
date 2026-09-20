---
"@sightmap/jev-turbo": patch
---

`--judge-effects` on `explore` and `bench` asks Jev what each action did, from the difference between the page before and after (URL, the controls that appeared and disappeared, the typed field's value, any notice), and records the verdict beside the rule's in the run file with the evidence it read. The rule that compares candidate lists is fooled by a recommendation rail rendering at the same moment as a removed bag row, and cannot see a radio's checked state or a status notice; the judge reads both. Only the rule's "changed" and "none" verdicts are judged, since a moved URL and a landed value need no second opinion, which keeps the cost to about 200 ms on one step in five.
