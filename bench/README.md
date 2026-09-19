# explore benchmarks

Goal suites for `jev-turbo bench`. Each suite is a set of goals on one site
with a reset between goals: ten short ones for saucedemo and books, two long
ones for journeys, and the single jev-ultrafast task for flights. Runs are
reproducible from this directory with a running session and the keys in the
environment (`TYPESAFE_API_KEY`; `ANTHROPIC_API_KEY` only for
`--picker anthropic`).

| suite | site | corpus | what it tests |
|---|---|---|---|
| `saucedemo.json` | saucedemo.com | `saucedemo/.sightmap` (39 components, from the sightkick example) | a mapped site: login, cart, a 12-step checkout, a select, a menu, an error state; finish checks are URL and text so `--no-map` can pass them |
| `saucedemo-tools.json` | saucedemo.com | `saucedemo/.sightmap` + `saucedemo/.sightkick` (17 tools, the sightkick example) | the same ten goals as `saucedemo.json`, plus a `name` value on the four product goals, run over the tool layer named by the suite's `tools` field so the picker sees tools like `log_in` and `add_to_cart` ahead of raw elements |
| `books.json` | books.toscrape.com | `books/.sightmap` (empty) | an unmapped site with 114 raw links on the home page: categories, pagination, detail pages |
| `flights.json` | Google Flights | `flights/.sightmap` (14 components, 6 memory lines) | the jev-ultrafast task: one-way Zürich to London on September 20, 2026; suggestion dialogs, a select, a typed date, a results page that renders late |
| `journeys.json` | saucedemo.com | `saucedemo/.sightmap` | two long goals (30–45 steps): three separate orders; sort and buy the two cheapest |
| `ikea.json` | ikea.com | `ikea/.sightmap` (20 components, 4 views, 14 memory lines) + `ikea/.sightkick` (6 tools) | a large public retail site, about 420 candidates a step: one goal, the KALLAX shelf unit in white into the shopping bag. A consent banner that floats over the lower viewport, a survey modal that can open on any page, a search box whose submit button is hidden until it has focus, an add-to-bag confirmation sheet that is modal, and a bag page built on hashed CSS-module class names. Run it with `--tools ikea` for the tool condition; the suite file carries no `tools` key |

## Run

```bash
cd bench

sightmap browser start --detach --url https://www.saucedemo.com/ --sightmap-dir saucedemo/.sightmap \
  --profile ~/.sightmap/profiles/explore-saucedemo --port 7901 --cdp-port 7902
jev-turbo bench saucedemo.json                       # Jev
jev-turbo bench saucedemo.json --picker anthropic    # Claude in the same seat
jev-turbo bench saucedemo-tools.json                 # same goals, over the sightkick tool layer

sightmap browser start --detach --url https://books.toscrape.com/ --sightmap-dir books/.sightmap \
  --profile ~/.sightmap/profiles/explore-books --port 7911 --cdp-port 7912
jev-turbo bench books.json

sightmap browser start --detach --url 'https://www.google.com/travel/flights?hl=en&curr=USD' --sightmap-dir flights/.sightmap \
  --profile ~/.sightmap/profiles/explore-flights --port 7931 --cdp-port 7932
jev-turbo bench flights.json --repeat 5
jev-turbo bench flights.json --record ../out/rec && python3 ../scripts/render-demo.py ../out/rec ../out/demo   # the README video

sightmap browser start --detach --headless --url https://www.ikea.com/us/en/ --sightmap-dir ikea/.sightmap \
  --profile ~/.sightmap/profiles/explore-ikea --port 7957 --cdp-port 7958
jev-turbo bench ikea.json --repeat 2
jev-turbo bench ikea.json --tools ikea                 # same goal, over the sightkick tool layer
```

Use a fresh profile for Google Flights. After many automated searches from one
profile the page pre-fills places from recent searches and the run drifts.

Saucedemo's published password trips Chrome's breach warning, a native dialog
the page cannot see. Launch the profile once, stop, set
`profile.password_manager_leak_detection: false` in the profile's
`Default/Preferences`, and start again.

Each run writes a JSON file (`--out`) with every step's pick, probabilities,
timings, and the observed transitions.

## Results (2026-09-17, jev-turbo at this commit)

Same loop for both pickers. The Claude rows are `claude-sonnet-5` asked for the
same pick as a JSON reply, a bare picker rather than a full agent, so read them
as "a big model in the same seat", not as the best a big model can do.

| suite | picker | goals | steps | wall / step | model / call | Anthropic bill |
|---|---|---|---|---|---|---|
| saucedemo | jev-latest | 10/10 | 59 | 0.24 s | 160 ms | $0 |
| saucedemo | claude-sonnet-5 | 10/10 | 59 | 1.16 s | 1,230 ms | $0.25 |
| books (empty corpus) | jev-latest | 10/10 | 23 | 0.30 s | 184 ms | $0 |
| books (empty corpus) | claude-sonnet-5 | 9/10 | 44 | 2.54 s | 1,802 ms | $0.81 |
| flights, Zürich→London ×5 | jev-latest | 5/5, runs of 7.9 / 8.3 / 8.4 / 8.6 / 10.9 s | 53 | 0.83 s | 185 ms | $0 |
| flights, Zürich→London | claude-sonnet-5 | 1/1, 61.6 s | 17 | 3.62 s | 1,769 ms | $0.29 |

The full run files are in `results/`. The flights runs are nine actions each
(origin, suggestion, destination, suggestion, date, trip type, One way, Search,
wait); jev-ultrafast reports a 7.1 s median over three runs of the same task.
Where the seconds go on Google Flights: the snapshot of a 4,000-node tree costs
250 to 350 ms a step, Jev 150 to 300 ms, and the results take about 1.5 s to
render after Search. An Atlanta→Zürich variant was tried and is not reliable
yet: with the origin pre-filled Google shows "Explore" instead of "Search" and
opens a calendar on the date field. The 12-step saucedemo checkout (login,
add to cart, cart, checkout, three fields, continue, finish) took 3.2 s with
Jev. The one Claude miss was a books pagination goal that looped between the
logo and category links until the step cap.

Where a Jev step's time goes on saucedemo: snapshot 7 to 30 ms, Jev 130 to 250
ms, the DOM click or native-setter fill under 50 ms, settle about 120 ms (two
quiet samples 40 ms apart). The model is now most of the step.

Jev token use for whoever prices it: about 1.2k input tokens per call on
saucedemo and 3.7k on the books home page; output under 200.

## Map vs no map (2026-09-18)

Same loop, same site, same model, run once with a sightmap and once with
`--no-map`, which points the loop at an empty corpus so every candidate is an
unnamed node from the raw HTML and ARIA tree. Jev `jev-latest`, on this branch
(`map-vs-no-map`): saucedemo and journeys at commit `8bda9cd`, Google Flights
rerun at commit `111c91c`, once a grouped step started recording the joint
probability of its two answers rather than the second one alone. Saucedemo and
journeys ran headless in one session; Google Flights ran headless in a second
session with a fresh Chrome profile, because a reused profile pre-fills places
from earlier searches and the run drifts.

| suite | condition | goals reached | steps per reached goal (median) | wasted steps | low-confidence picks | candidates a step (median) | seconds |
|---|---|---|---|---|---|---|---|
| saucedemo, 10 goals ×3 | map | 30/30 | 4.5 | 3 | 0 | 3 | 48.4 |
| saucedemo, 10 goals ×3 | no map | 30/30 | 4.5 | 3 | 2 | 3 | 45.6 |
| journeys, 2 long goals ×3 | map | 3/6 | 13 | 63 | 34 | 9 | 53.8 |
| journeys, 2 long goals ×3 | no map | 3/6 | 13 | 53 | 49 | 10 | 53.4 |
| Google Flights, Zürich to London ×10 | map | 10/10 | 10 | 7 | 4 | 78 | 116.3 |
| Google Flights, Zürich to London ×10 | no map | 10/10 | 15 | 12 | 47 | 78 | 143.2 |

Commands:

```bash
cd bench

sightmap browser start --detach --headless --url https://www.saucedemo.com/ --sightmap-dir saucedemo/.sightmap \
  --profile ~/.sightmap/profiles/explore-saucedemo --port 7951 --cdp-port 7952
jev-turbo bench saucedemo.json --repeat 3 --out results/saucedemo-map.json
jev-turbo bench saucedemo.json --repeat 3 --no-map --out results/saucedemo-no-map.json
jev-turbo bench journeys.json --repeat 3 --out results/journeys-map.json
jev-turbo bench journeys.json --repeat 3 --no-map --out results/journeys-no-map.json
cd saucedemo && sightmap browser stop && cd ..

sightmap browser start --detach --headless --url 'https://www.google.com/travel/flights?hl=en&curr=USD' --sightmap-dir flights/.sightmap \
  --profile ~/.sightmap/profiles/explore-flights-7 --port 7953 --cdp-port 7954
jev-turbo bench flights.json --repeat 10 --out results/flights-map.json
jev-turbo bench flights.json --repeat 10 --no-map --out results/flights-no-map.json
cd flights && sightmap browser stop && cd ..
```

Steps per reached goal is the median of the acted steps of the runs that
reached their goal; the closing `done` record is not an action and is not
counted (`IsAction`, `explore/metrics.go`). Wasted counts a step that repeats work: a stale pick, a back, or a control
already acted on at this URL (`Step.Wasted`, `explore/explore.go`). Fallback
counts a step where a map exists but Jev's pick carries no sightmap
component (`Step.Fallback`); it can only fire in the map condition, so the
row prints `-` under `--no-map` because a fallback needs a map.
Low-confidence, called unsure picks in the
README's table, counts a step where Jev's own probability for the option it
picked is above 0 and below 0.6, the `LowConfidence` constant in
`explore/metrics.go`. On a grouped step, where a page too large for one
question is answered as a group and then an option inside it, that
probability is the group's times the option's within the group; the group
half of it is kept on its own in `group_confidence`. Candidates counts what is left of the page after the
guards filter it, before Jev sees any names (`Step.Candidates`,
`explore/explore.go`); on Google Flights its median is 78 in both
conditions, because the guards keep the same elements either way and only
the label attached to each one changes.

Two earlier no-map runs failed for reasons that were bugs in this loop, not
properties of the site. An earlier saucedemo no-map run reached 0 of 30
because a button that shared its name with the form around it was dropped
from the candidate list; commit `0d894a2` fixes this by keeping a control a
candidate even when its container carries the same name. An earlier Google
Flights no-map run reached 1 of 10 because a suggestion list was read before
its entries had rendered into the tree; commit `abfadba` fixes this by
looking twice for suggestion options before falling back to the whole page.
Both fixes are on this branch, ahead of the six runs in the table above.

## Map score

`jev-turbo score` reads one or more files, each a bench result or a single
run file, and prints one column per file, so any number of runs sit side by
side. It is the table above, built by `explore.ScoreResult` instead of by
hand, and it runs on any run file, not only a map-versus-no-map pair.

The score reads on the map only when the picker is held fixed. Run the same
picker over two maps, or over a map and `--no-map`, and the difference
between the columns is what the map changed. A different picker moves every
row too, as the Jev-versus-Claude rows above show. For a curator holding the
picker fixed, the rows to watch are wasted steps, fallback picks,
low-confidence picks, and candidates offered. A page that scores badly on
those is a page where a name, a property, or a memory line is missing.

- `reached`: goals reached out of goals attempted (`Score.Reached` /
  `Score.Goals`).
- `steps / goal`: median acted steps per reached goal. The closing `done`,
  `done(judged)` and `no-candidates` records are not actions and are not
  counted (`IsAction`, `explore/metrics.go`).
- `wasted steps`: total across the runs in the file of steps that repeat
  work: a stale pick, a back, or a control already acted on at this URL
  (`Step.Wasted`).
- `fallback picks`: total across the runs in the file of picks where a map
  exists but Jev's pick carries no sightmap component (`Step.Fallback`). A
  page with many of these needs more named components. A fallback needs a map
  to fire, so this row prints `-` under `--no-map`.
- `low-confidence picks`: total across the runs in the file of steps where
  Jev's own probability for the picked option is above 0 and below 0.6, the
  `LowConfidence` constant in `explore/metrics.go`. A component whose name is
  close to a neighbor's tends to show up here.
- `candidates offered`: median candidates left after the guards, before Jev
  sees any names (`Step.Candidates`). A high count next to a low reach rate
  points at a page that needs to be split into more specific components.
- `low-coverage pages`: distinct URLs where named controls are under half of
  the interactive ones (`Score.LowCoveragePages`). These are the pages to map
  next. Coverage is measured against the corpus, so this row prints `-` under
  `--no-map`.
- `seconds`: total wall time across the runs in the file.

A run file written before these metrics existed carries no counts. On such a
file `wasted steps`, `fallback picks`, `low-confidence picks` and
`candidates offered` all print `-`.

Command:

```bash
jev-turbo score results/saucedemo-map.json results/saucedemo-no-map.json \
  results/journeys-map.json results/journeys-no-map.json \
  results/flights-map.json results/flights-no-map.json
```

Output:

```
                      saucedemo/map/jev:jev-latest  saucedemo/no-map/jev:jev-latest  journeys/map/jev:jev-latest  journeys/no-map/jev:jev-latest  flights/map/jev:jev-latest  flights/no-map/jev:jev-latest
reached               30/30                         30/30                            3/6                          3/6                             10/10                       10/10
steps / goal          4.5                           4.5                              13                           13                              10                          15
wasted steps          3                             3                                63                           53                              7                           12
fallback picks        3                             -                                12                           -                               0                           -
low-confidence picks  0                             2                                34                           49                              4                           47
candidates offered    3                             3                                9                            10                              78                          78
low-coverage pages    0                             -                                0                            -                               5                           -
seconds               48.4                          45.6                             53.8                         53.4                            116.3                       143.2
```

## Tools mode

Same ten saucedemo goals as `saucedemo.json`, over the same
`saucedemo/.sightmap` corpus, with the sightkick tool layer named by the
suite's `tools` field switched on, so Jev sees its 17 tools, such as `log_in`,
`add_to_cart`, and `go_to_cart`, offered ahead of the raw sightmap elements.
The goals are not quite identical: `saucedemo-tools.json` adds a `name` value
to the four product goals, so that `add_to_cart(name)` and `open_item(name)`
have the argument they require and can be offered at all. When Jev picks a
tool, jev-turbo runs it with
`sightkick call <app dir> <tool> --param k=v --via cli`. That command
translates the tool's steps into `sightmap browser` commands (a snapshot, a
click or fill, a wait) against the same recorded session. Jev can still fall
back to a raw element on a step where no tool fits.

```
                      saucedemo/map/jev:jev-latest  saucedemo-tools/tools/jev:jev-latest
reached               30/30                         30/30
steps / goal          4.5                           2.5
wasted steps          3                             18
fallback picks        3                             0
low-confidence picks  0                             14
candidates offered    3                             30
low-coverage pages    0                             0
seconds               48.4                          79.5
```

Both conditions reach all 30 goals. Tools mode takes fewer steps per goal, 2.5
against 4.5, because one tool call folds several element actions into one
step. For example `log_in` fills two fields and clicks Login, and
`add_to_cart` clicks the product's button and waits for it to read Remove.
Jev ran 76 tool calls across the 30 goals; 67 reported ok and 9 failed.

Wasted steps go up, 18 against 3. The map run's 3 are one repeated click, the
same Add to cart pick taken twice on the same page, once in each of the three
repeats. The tools run has that click as well, plus 6 failed `log_out` calls,
8 repeated tool calls, and one repeated menu click. All 8 repeats are in a
single logout run that cycled `log_out` and `log_in` four times before falling
back to the menu link; the other two logout runs finished in six steps. Three
more calls failed and are not counted: the locked-out logins, whose `wait_for`
times out but whose error banner is what their goal asks for, so the next
observation finishes the goal and the step keeps its work.

Low-confidence picks go up too, 14 against 0, and that is not the failures: 8
of the 14 are in goals where no tool failed. What moves them is the longer
option list. Switching tools on takes the median options a step from 5 to 37,
and Jev spreads the same probability mass over more entries, so more picks
land under the 0.6 line.

Candidates offered rises to 30 from 3, but not because tools count as
candidates: `Step.Candidates` counts page elements only, and tools are counted
separately, in `options`. With tools, login is a single `log_in` step, so fewer
of the run's steps land on the Login page, which offers only 3 candidates. More
of them land on the Inventory page instead, where the median is 34, and that
pulls the overall median up. Cart steps carry about 10.

Seconds go up in tools mode, 79.5 s against 48.4 s, even with fewer steps,
because a tool call is not one browser action. Each `sightkick call --via cli`
run is several `sightmap browser` commands in sequence, and some of those,
mainly click and fill, stall on saucedemo's own elements before returning.

Commands:

```bash
cd bench

sightmap browser start --detach --headless --url https://www.saucedemo.com/ --sightmap-dir saucedemo/.sightmap \
  --profile ~/.sightmap/profiles/explore-saucedemo --port 7961 --cdp-port 7962
jev-turbo bench saucedemo-tools.json --repeat 3 --out results/saucedemo-tools.json
cd saucedemo && sightmap browser stop && cd ..

jev-turbo score results/saucedemo-map.json results/saucedemo-tools.json
```

## IKEA (2026-09-18)

One goal on ikea.com, the KALLAX shelf unit in white into the shopping bag,
run five times in each of three conditions: `--no-map`, the map in
`ikea/.sightmap`, and that map's sightkick tools. Jev `jev-latest`, headless,
in one session on a fresh profile with a 1200×900 window. Three more single
runs, one per condition, were recorded with `--record`; they are the three
acts of the README video.

```
condition=no-map jev:jev-latest: 5/5 ok · 44 steps · 71.6s total · 1.63s/step · model 62 calls, 210 ms/call, 489182+25536 tok · wasted 5 · fallback 0 · low-conf 18 · candidates ~423
condition=map jev:jev-latest: 5/5 ok · 83 steps · 130.2s total · 1.57s/step · model 133 calls, 208 ms/call, 1072209+50221 tok · wasted 16 · fallback 44 · low-conf 50 · candidates ~423
condition=tools jev:jev-latest: 5/5 ok · 20 steps · 34.7s total · 1.73s/step · model 15 calls, 267 ms/call, 229193+12353 tok · wasted 0 · fallback 5 · low-conf 2 · candidates ~563
```

```
                      ikea/no-map/jev:jev-latest  ikea/map/jev:jev-latest  ikea/tools/jev:jev-latest
reached               5/5                         5/5                      5/5
steps / goal          8                           15                       3
wasted steps          5                           16                       0
fallback picks        -                           44                       5
low-confidence picks  18                          50                       2
candidates offered    423                         423                      563
low-coverage pages    -                           7                        1
seconds               71.6                        130.2                    34.7
```

On ikea.com the map as authored did not help Jev. It took more steps than the
raw tree: 15 a goal with the map, 8 without it. The run files show why. Enter
after typing does not submit IKEA's search box, and the visual-search button
and the Products menu carry no name in the map, which the score counts as 44
fallback picks and 7 pages where named controls are under half the interactive
ones. The map's tools cut the goal to 3 steps, because the `search` tool
submits the search with its own keypress and wait. Improving the map is the
next curation step, and the score is what points at it.

The suite's earlier goal was the BILLY bookcase. The home page carries a BILLY
rail with an Add button, so a raw tree reached that goal in three actions, and
the goal was replaced with the KALLAX shelf unit, which has to be searched for.

The recorded map run reached the goal in 17 steps over 29.0 s, all of it
captured; the video plays the full run at 1x with no held frames.

Commands:

```bash
cd bench

rm -rf ~/.sightmap/profiles/explore-ikea-2
sightmap browser start --detach --headless --url https://www.ikea.com/us/en/ --sightmap-dir ikea/.sightmap \
  --profile ~/.sightmap/profiles/explore-ikea-2 --port 7957 --cdp-port 7958 --chrome-flag=--window-size=1200,900
jev-turbo bench ikea.json --repeat 5 --no-map --out results/ikea-no-map.json
jev-turbo bench ikea.json --repeat 5 --out results/ikea-map.json
jev-turbo bench ikea.json --repeat 5 --tools ikea --out results/ikea-tools.json
jev-turbo bench ikea.json --no-map --record ../out/ikea-no-map --out results/ikea-demo-no-map.json
jev-turbo bench ikea.json --record ../out/ikea-map --out results/ikea-demo-map.json
jev-turbo bench ikea.json --tools ikea --record ../out/ikea-tools --out results/ikea-demo-tools.json
cd ikea && sightmap browser stop --port 7957 && cd ..

jev-turbo score results/ikea-no-map.json results/ikea-map.json results/ikea-tools.json
python3 ../scripts/render-demo.py --compare "without a map=../out/ikea-no-map" \
  "with a map=../out/ikea-map" "with sightkick tools=../out/ikea-tools" \
  ../out/demo --subtitle "One goal, one model, three ways in."   # the README video
```

## Growing the corpus (`--grow`)

`jev-turbo bench books.json --grow --repeat 3`, starting from
the empty `books/.sightmap` (reset it with `rm -rf books/.sightmap/views` and a
fresh `components.yaml` of `version: 1`, `components: []`), jev-turbo at
this commit, where grow decides button, link, input and select from role and
tag and asks Jev only about repeated containers (`grow/kinds.go`):

| repeat | goals | corpus after |
|---|---|---|
| 1 | 10/10 | 4 views, 6 components, 4 promoted to global |
| 2 | 10/10 | unchanged |
| 3 | 10/10 | unchanged |

Grow work cost 213 ms in total across the four page types, with one Jev
classification call. The earlier version of grow, which asked Jev about
every container, printed seven model calls in the stderr report at the end
of its run; no run file was kept for it, so that seven comes from the report
line and not from `results/`. `sightmap validate` and `sightmap lint
--warn-only` are clean on the result. The run file for the numbers above is
`results/books-jev-grow.json`.

Grow is coverage scaffolding, nothing more. It scopes each new component to
the nearest stable ancestor the page already has, verifies the selector,
names the component from a template, reads off `label`, `href`, and
`placeholder` as properties, and names views from their routes. It does not
add meaning. Any property beyond those three, a memory line, or a name a
person would actually choose still has to come from the authoring skill or a
curator. The naming step is where a curator plugs in: it sits behind the
`grow.Namer` interface. The shipped default, `TemplateNamer` in
`grow/namer.go`, names from a template; a curator that proposes names
implements the same interface.

## What the numbers do and do not show

- A typed model at about 200 ms is enough to pick every step on a mapped site,
  an unmapped site, and a real SaaS app when the page is offered as named
  actions with properties.
- The goals are short (2 to 12 steps) on cooperative sites. Nothing here needed
  backtracking, a modal that eats clicks, or an infinite-scroll feed.
- Jev never types free text. Every value came from the suite's spec.
- One run per configuration. Treat single-goal differences as noise.

Related work: [browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast)
runs the same idea over a raw element table with an operation head plus target
heads in one request, and a small text model for typed input. This loop
differs in offering sightmap component names and properties as the options,
and in taking typed values from a spec instead of generating them.
