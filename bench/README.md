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
| `books.json` | books.toscrape.com | `books/.sightmap` (empty) | an unmapped site with 114 raw links on the home page: categories, pagination, detail pages |
| `flights.json` | Google Flights | `flights/.sightmap` (14 components, 6 memory lines) | the jev-ultrafast task: one-way Zürich to London on September 20, 2026; suggestion dialogs, a select, a typed date, a results page that renders late |
| `journeys.json` | saucedemo.com | `saucedemo/.sightmap` | two long goals (30–45 steps): three separate orders; sort and buy the two cheapest |

## Run

```bash
cd bench

sightmap browser start --detach --url https://www.saucedemo.com/ --sightmap-dir saucedemo/.sightmap \
  --profile ~/.sightmap/profiles/explore-saucedemo --port 7901 --cdp-port 7902
jev-turbo bench saucedemo.json                       # Jev
jev-turbo bench saucedemo.json --picker anthropic    # Claude in the same seat

sightmap browser start --detach --url https://books.toscrape.com/ --sightmap-dir books/.sightmap \
  --profile ~/.sightmap/profiles/explore-books --port 7911 --cdp-port 7912
jev-turbo bench books.json

sightmap browser start --detach --url 'https://www.google.com/travel/flights?hl=en&curr=USD' --sightmap-dir flights/.sightmap \
  --profile ~/.sightmap/profiles/explore-flights --port 7931 --cdp-port 7932
jev-turbo bench flights.json --repeat 5
jev-turbo bench flights.json --record ../out/rec && python3 ../scripts/render-demo.py ../out/rec ../out/demo   # the README video
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
component (`Step.Fallback`); it can only fire in the map condition, which is
why every no-map row reads 0. Low-confidence, called unsure picks in the
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

## Growing the corpus (`--grow`)

`jev-turbo bench books.json --grow --repeat 3`, starting from
the empty `books/.sightmap` (reset it with `rm -rf books/.sightmap/views` and a
fresh `components.yaml` of `version: 1`, `components: []`):

| repeat | goals | wall / step | coverage of visited pages | corpus after |
|---|---|---|---|---|
| 1 | 10/10 | 0.32 s | 0% → 89% | 4 views, 6 components, 4 promoted to global |
| 2 | 10/10 | 0.29 s | 100% | unchanged |
| 3 | 10/10 | 0.28 s | 100% | unchanged |

Grow work cost 1.2 s in total across the four page types, with seven Jev
classification calls. `sightmap validate` and `sightmap lint --warn-only` are
clean on the result. The run file is `results/books-jev-grow.json`.

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
