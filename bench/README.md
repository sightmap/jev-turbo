# explore benchmarks

Goal suites for `jev-turbo bench`. Each suite is ten goals on
one site with a reset between goals. Runs are reproducible from this directory
with a running session and the keys in the environment (`TYPESAFE_API_KEY`;
`ANTHROPIC_API_KEY` only for `--picker anthropic`).

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
