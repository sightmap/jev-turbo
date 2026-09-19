# @sightmap/jev-turbo

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
