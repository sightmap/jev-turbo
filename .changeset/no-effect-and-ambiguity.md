---
"@sightmap/jev-turbo": patch
---

Each step records what it did, judged from the observation that followed it: `navigated`, `value` (the filled field holds a value), `changed` (the page offers different controls) or `none`; a tool step is judged from its own outcome, and the last step of a run that hit its limit is left blank. The score gains a `no-effect steps` row, so a fill that never landed or an Enter that submitted nothing shows up as a driver fault instead of reading as a map problem, which is how the ikea.com focus bug hid. Each step also counts the candidates whose description is identical to another's on the page (`Step.Ambiguous`), and the score shows the median as `same-name candidates`: on IKEA's listing sixteen "Add to cart" buttons share one description, which is exactly where a map that scopes each button to its card pays. Files written before these fields existed print `-` in both rows.
