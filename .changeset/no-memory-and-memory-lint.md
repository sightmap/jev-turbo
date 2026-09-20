---
"@sightmap/jev-turbo": patch
---

`--no-memory` on `explore` and `bench` keeps the map's components and views and withholds every memory note, corpus, view, and component level, from what the picker sees, so a run against a plain map run says whether a site's prose helped or hurt; `--no-map` still drops the whole map, and the two do not combine. A bench run without memory records its condition as `map-no-memory` (`tools-no-memory` with a tool layer), and the score keeps counting fallback picks for it, since the map is still there to fall back from. `jev-turbo memory-lint DIR` reads a corpus and prints every memory note that prescribes a route, forbids an action, or rules a page out, with the reason, and exits 1 when it flags any, so a note written for one goal is caught when the map is written rather than when it steers the next goal wrong, as the IKEA map's did.
