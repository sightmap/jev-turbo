# @sightmap/jev-turbo

## 0.2.1

### Patch Changes

- da9a4c3: A finish check can now say what must not be on the page. `text_absent` holds when the page does not show a text, and `text_count` bounds how often a text shows: exactly N times, at least N, or between N and M (`--done-when text_count=1:BILLY`, `1+:BILLY`, `1-2:BILLY`). A presence check could only assert that the right item was in the bag; a run that added it twice passed `text_contains` and scored as a success. A count sees the second row. Occurrences are counted on the innermost nodes that show the text, since a node's rendered text is its whole subtree's, so a product name is one per bag row, not one per level of nesting.
- da9a4c3: A step's history line names the component that owns the control it acted on ("clicked [AddToCartButton] in [ProductCard title=...]"), so a pick among many same-named controls can be told apart afterwards, and component property values show up to 60 characters instead of 40. New `bench/ikea-variants.json`: three KALLAX goals that ask for one exact variant from a listing where every add button carries the same name.
- b69d5ae: A fill verifies that the value landed in the field it was asked to fill, and sets it there when it did not. The combobox path typed wherever focus was, and a synthetic click does not move focus, so a page whose consent banner holds keyboard focus (ikea.com) took the keystrokes into the banner and left the field empty. Enter is now pressed in the field the last fill typed into, not in whatever holds focus. The suggestion wait also counts the entries of the list a combobox names through `aria-controls`, so a dropdown of plain links is seen as one. The IKEA map gains a Category view, the header menu (NavEntry, MenuTab, MenuLink), and a shared ProductCard with an AddToCartButton; the memory that steered Jev away from the category route is gone. The IKEA bench reruns at 4 steps a goal in both conditions, from 8 without the map and 15 with it, with every map pick a named component.
- da9a4c3: Each step records what it did, judged from the observation that followed it: `navigated`, `value` (the filled field holds a value), `changed` (the page offers different controls) or `none`; a tool step is judged from its own outcome, and the last step of a run that hit its limit is left blank. The score gains a `no-effect steps` row, so a fill that never landed or an Enter that submitted nothing shows up as a driver fault instead of reading as a map problem, which is how the ikea.com focus bug hid. Each step also counts the candidates whose description is identical to another's on the page (`Step.Ambiguous`), and the score shows the median as `same-name candidates`: on IKEA's listing sixteen "Add to cart" buttons share one description, which is exactly where a map that scopes each button to its card pays. Files written before these fields existed print `-` in both rows.
- da9a4c3: `--no-memory` on `explore` and `bench` keeps the map's components and views and withholds every memory note, corpus, view, and component level, from what the picker sees, so a run against a plain map run says whether a site's prose helped or hurt; `--no-map` still drops the whole map, and the two do not combine. A bench run without memory records its condition as `map-no-memory` (`tools-no-memory` with a tool layer), and the score keeps counting fallback picks for it, since the map is still there to fall back from. `jev-turbo memory-lint DIR` reads a corpus and prints every memory note that prescribes a route, forbids an action, or rules a page out, with the reason, and exits 1 when it flags any, so a note written for one goal is caught when the map is written rather than when it steers the next goal wrong, as the IKEA map's did.

## 0.2.0

### Minor Changes

- a0ad56f: `--tools DIR` offers the tools of a sightkick layer to the picker ahead of the page's elements. A picked tool runs through `sightkick call --via cli`; its guidance orders the next options; elements remain the fallback when no tool fits or a call fails. Suites can name a `tools` directory. `bench/saucedemo-tools.json` runs the saucedemo goals over the example tool layer.

### Patch Changes

- b7ca543: New demo: the KALLAX shelf unit into the bag on ikea.com three times with the same model and loop: without a map, with a map, and with the map's sightkick tools. `scripts/render-demo.py --compare LABEL=DIR ...` renders recordings as acts and shows the choices offered per step. `bench/ikea.json`, its map, and its tool layer are new.
- 9099d11: grow decides button, link, input and select from roles and tags, and asks Jev only about repeated containers (nav, card, or noise). `grow.Namer` is an interface with `TemplateNamer` as the default, the place where a curator that proposes names can plug in. `bench --grow` writes the grown components and model calls into the run file. grow is coverage scaffolding: it scopes each new component to the nearest stable ancestor the page already has, verifies the selector, names it from a template, reads off `label`, `href` and `placeholder` as properties, and names views from their routes. Any property beyond those three, a memory line, or a name a person would choose still comes from the authoring skill or a curator.
- da984e4: `jev-turbo score RUN.json ...` prints one column per run file: goals reached, steps per goal, wasted steps, fallback picks, low-confidence picks, candidates offered, low-coverage pages, seconds. `bench` prints the same block after its table. It is a measure of the map: how a 200 ms driver does over it.

## 0.1.3

### Patch Changes

- b2bbbbc: `--no-map` on `explore` and `bench` runs the same loop with no components, views, or memory, so a map's effect can be measured against the same site and model. Every step records the candidates offered, how many were named, the pick's confidence, whether the pick fell back to an unnamed node, and whether the step was wasted (stale, back, or a repeat). Runs and suites summarise these. `history_count` is a new finish check for long goals (`--done-when history_count=3:Finish`), and `bench/journeys.json` holds two 30–45 step saucedemo goals. Two loop fixes: a button inside a same-named form is a candidate again, and a suggestion list that reaches the tree late is observed twice, which also makes Google Flights work headless. jev-turbo no longer brings the browser tab to the front, and `--start` takes `--headless`. The README reports what the map changed: the same goals reached with and without it on three suites, and on Google Flights 9 steps a goal instead of 16, 1 unsure pick instead of 36, 101 seconds instead of 142 over ten runs.

## 0.1.2

### Patch Changes

- 1eb0e23: The "back" action is offered only after the run has navigated at least once. From the first page, `history.back()` leaves the site for the tab's blank page and the loop cannot recover. The demo renderer puts the page in a browser frame with the run's actions, sightmap components, and finish check beside it.

## 0.1.1

### Patch Changes

- 37be544: Google Flights benchmark and demo video. `bench/flights.json` runs the same
  task as browser-use's jev-ultrafast demo (one-way Zürich to London on September
  20, 2026) over a 14-component corpus with site notes; five runs pass at a median
  of 8.4 s. `--record DIR` on `explore` and `bench` captures the driven tab as
  frames, and `scripts/render-demo.py` turns a recording into an MP4 and GIF with
  step captions. The loop learned what that site needed: real keystrokes into
  suggestion fields (typed into whichever element takes focus, so dialogs work),
  a wait for the typed city's suggestions rather than the recent-search entries,
  "pick an option next" after a field or select opens a list, a "wait" action
  that polls the finish check, an "enter" action after typing, no container or
  presentational nodes as candidates, and corpus memory lines passed to the
  picker as site notes. The README no longer reports runs against a third-party
  SaaS account.

## 0.1.0

### Minor Changes

- 53ad7f5: First release. `jev-turbo explore` drives a running sightmap browser session
  toward a goal with Jev picking every step over the annotated component tree;
  `jev-turbo bench` runs the goal suites under `bench/`; `--grow` builds the
  corpus while exploring.
