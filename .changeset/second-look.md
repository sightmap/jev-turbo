---
"@sightmap/jev-turbo": patch
---

`--second-look` on `explore` and `bench`: when a pick between groups comes back under 0.6, the loop opens the three likeliest groups, lists their members flat with the group each came from, and asks again; a member picked this way carries its own probability rather than the joint one. On the IKEA variants suite without a map, unsure picks fall from 38 to 23 over five runs a goal, with fewer model calls in total because the sharper picks save steps.
