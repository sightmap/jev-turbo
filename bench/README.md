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
| `ikea.json` | ikea.com | `ikea/.sightmap` (24 components, 5 views, 16 memory lines) + `ikea/.sightkick` (6 tools) | a large public retail site, about 600 candidates a step: one goal, the KALLAX shelf unit in white into the shopping bag. A consent banner that floats over the lower viewport and holds keyboard focus, a survey modal that can open on any page, a search box whose submit button is hidden until it has focus, an add-to-bag confirmation sheet that is modal, and a bag page built on hashed CSS-module class names. Run it with `--tools ikea` for the tool condition; the suite file carries no `tools` key |
| `ikea-variants.json` | ikea.com | `ikea/.sightmap` | three goals that each ask for one exact KALLAX variant from a listing where every add button reads `Add "KALLAX Shelf unit" to cart`: the white 2x2, the black-brown 2x2 (a swatch under the white card, not a card of its own), and both a white 1x4 and a black-brown 2x2 in one bag. Finish checks count bag rows with `text_count`, so an extra item fails the goal |

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
jev-turbo bench ikea.json --no-map                    # same goal, the raw tree: no components, views, or memory
jev-turbo bench ikea.json --no-memory                 # same map, its memory notes withheld from Jev
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

Without a map, the loop groups the controls of a page for the first pick by
landmark, and since 2026-09-20 by entry as well: a container whose parent
holds siblings of the same shape and whose subtree carries a title (a
heading, else the longest link) is one entry, and its controls are offered
under that title, a promoted control says which entry it is in, and the
history line does too. That is the raw tree's version of what a mapped
ProductCard does. On the variants suite it took the raw tree from 10 of 15
goals to 15 of 15 (wasted steps 34 to 15, unsure picks 79 to 38); the map
still picks at 1.00 where the raw tree picks near 0.4, because a mapped
title is the card's clean name and an entry's title is whatever its longest
link says.

`--second-look` follows an unsure group pick, one under 0.6, with a second:
the three likeliest groups are opened and their members listed flat, each
saying which group it came from, and Jev chooses again with that in front
of it. On the variants suite without a map that took unsure picks from 38
to 23 over five runs a goal, and the run made fewer model calls in total,
146 against 152, because the surer picks saved steps. A pick made on the
second look carries its own probability, not the joint one, and the step
records `second_look`.

`--distill-check` turns a reached goal into a finish check: Jev picks the
part of the final URL and the text on the final page that prove the goal,
the two become a `done_when`, and the proposal is scored against every page
the run passed through, since a check that also holds on an earlier page
would have stopped the run there. When it still holds on one, Jev is asked
for a text that page does not show, up to twice. Over saucedemo and the
IKEA variants, 9 of 13 proposals held at the end and failed on every earlier
page (`{"url_contains": "/cart.html"}` with the item's name, the
checkout-complete page with "Thank you for your order!", the bag with the
exact KALLAX row); three held on one earlier page too; the logout goal got
nothing, because its proof is an absence, which `text_absent` can say and
the distiller does not yet propose. The proposal prints after the goal and
sits in the run file under `distilled`.

A third ablation sits between the two. `--no-memory` keeps the map's
components and views and withholds only its memory notes, the free-text lines
the picker sees under `SITE NOTES`, and the run file records the condition as
`map-no-memory` (`tools-no-memory` with a tool layer). Against a plain map
run it isolates what the site's prose did, which is how a note written for
one goal, like the ones that steered the IKEA runs below away from the
category route, shows up as a cost rather than being folded into the map's
score. `jev-turbo memory-lint DIR` reads the same notes and flags the ones
that prescribe a route or forbid an action, so a map can be checked as it is
written, before a run.

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
- `no-effect steps`: total across the runs in the file of steps whose next
  observation showed nothing had happened: the URL did not move, a filled
  field holds no value, and the page offers the same controls as before
  (`Step.Effect` is `none`; the other values are `navigated`, `value` and
  `changed`). A fill whose keystrokes went elsewhere or an Enter on an empty
  field lands here. A run of these is a driver or page fault, not a map
  fault, and it is what let a fill that never landed on ikea.com read as a
  map problem.
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
- `same-name candidates`: median per step of candidates whose description is
  identical to another candidate's on the same page (`Step.Ambiguous`), so a
  listing with sixteen "Add to cart" buttons counts sixteen. These are the
  picks the raw tree cannot tell apart, and where a map that scopes each
  control to its owner and properties pays.
- `low-coverage pages`: distinct URLs where named controls are under half of
  the interactive ones (`Score.LowCoveragePages`). These are the pages to map
  next. Coverage is measured against the corpus, so this row prints `-` under
  `--no-map`.
- `seconds`: total wall time across the runs in the file.

With `--judge-effects`, the verdict comes from Jev rather than the rule: it
reads the URL before and after, the controls that appeared and disappeared
(the ones sharing words with the action first), the typed field's value and
any notice, and answers navigated, opened, value, changed, error or none.
The rule's verdict stays in the run file as `effect_rule` beside the
judge's `effect`, with `effect_confidence` and the `effect_evidence` it
read, so a disagreement can be checked. Only the rule's "changed" and
"none" are judged; a moved URL and a landed value need no second opinion.
On 204 steps across saucedemo and the IKEA variants the two agreed on 187;
every disagreement read went the judge's way: a menu click is "opened",
a locked-out login is "error", a sort select is "value", a swatch toggle
is "changed" on the strength of a "New variant selected" notice the rule
cannot see, and a "Remove" whose row was still there at the next look is
"none" even though forty recommendations rendered beside it.

A run file written before these metrics existed carries no counts. On such a
file `wasted steps`, `no-effect steps`, `fallback picks`, `low-confidence
picks`, `candidates offered` and `same-name candidates` all print `-`.

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

## IKEA variants (2026-09-20)

Three goals on ikea.com that each ask for one exact KALLAX variant, five runs
a goal in three conditions: `--no-map`, the map in `ikea/.sightmap`, and the
same map with `--no-memory`. Jev `jev-latest`, headless, one session on a
fresh profile with a 1200×900 window.

The listing for "KALLAX shelf unit" shows about thirty cards. Sixteen of
their add buttons carry the identical accessible name `Add "KALLAX Shelf
unit" to cart`; the size and colour live in the card's title link, a sibling.
The raw tree offers those sixteen as one flat run of same-named buttons. The
map scopes each button to its ProductCard, so the picker sees `AddToCartButton
in [ProductCard price="49.99" title="KALLAX, Shelf unit, white, 30 1/8x30 1/8"]`,
and the loop offers each card as a group headed by that title.

```
                      ikea-variants/no-map/jev:jev-latest  ikea-variants/map/jev:jev-latest  ikea-variants/map-no-memory/jev:jev-latest
reached               10/15                                15/15                             15/15
steps / goal          7                                    5                                 6
wasted steps          34                                   5                                 7
no-effect steps       44                                   1                                 7
fallback picks        -                                    0                                 3
low-confidence picks  79                                   2                                 34
candidates offered    146                                  561                               560
same-name candidates  28                                   189                               189
low-coverage pages    -                                    2                                 5
seconds               337.0                                198.8                             222.0
```

Per goal, actions a goal (median of five) and how many runs reached it:

| goal | no-map | map | map, no memory |
|---|---|---|---|
| white 2x2 | 5, 5/5, 10 unsure | 4, 5/5, 1 unsure | 4, 5/5, 6 unsure |
| black-brown 2x2 | 9, 5/5, 15 unsure | 5, 5/5, 1 unsure | 6, 5/5, 10 unsure |
| white 1x4 + black-brown 2x2 | 20, 0/5, 54 unsure | 6, 5/5, 0 unsure | 6, 5/5, 18 unsure |

What each condition does. Without the map Jev never presses a listing add
button: it opens the product page for the variant it can read in a title
link, adds there, and goes to the bag; five actions on the easy goal, nine on
the black-brown one (a swatch under the white card, which the raw tree toggles
twice before it trusts it), and the two-item goal never finishes in twenty
steps, mostly spent removing and re-adding on the bag page. With the map every
add is the card's own button at 1.00, the black-brown one after a VariantSwatch
pick, and the two-item goal is search, Enter, add, swatch, add, bag. The map
without its notes takes the same routes with the same step counts but is
unsure seventeen times more often, and picks an unnamed node three times: on
this map the memory buys confidence, not steps.

The `same-name candidates` row is a property of the pages a run visited, not
of the condition: the map runs took their picks on the listing, where 189
candidates share a name with another, and were still sure; the raw tree left
for the product page, where fewer do.

The `no-effect steps` row is the row that was missing on 2026-09-18. It counts
actions after which the next observation showed nothing new: no navigation,
no value in the filled field, no change in the controls offered. The raw
tree's 44 are mostly waits and scrolls on the bag page and adds whose
confirmation sheet had not rendered by the next look; that last case is a
timing false positive the row does not yet distinguish.

`jev-turbo mine results/saucedemo-map.json` reads run files and prints the
step sequences that recur across their successful runs as a sightkick
tools.yaml draft. A sequence stays on one view, a navigating step can only
end it after fills (a form and its submit) and otherwise stands alone, a
fill's value key becomes a parameter, and the view a step reaches becomes a
wait. On thirty saucedemo runs it drafts login, add_to_cart, open_cart, menu,
checkout, the checkout form, finish and logout; on the IKEA runs, search,
add_to_cart and open_bag, every journey those runs took, and none of the
hand-written tools the runs never exercised (open_product, close_survey,
read_bag). A journey every run repeats by hand is a tool waiting to be
written; the draft is the starting point, not the layer.

`jev-turbo same-names --sightmap-dir ikea/.sightmap --url URL` lists the
controls on a live page that share a role and a name, with the entry or
component that tells each apart and the accessible name that would say so.
It is the same-name row as a report, and an accessibility finding: sixteen
buttons announced identically is a defect for a screen reader before it is
one for a picker. On the KALLAX listing without a map it reports thirteen
add buttons and thirteen save buttons by their cards; with the map loaded
those groups are gone, because the map names them.

`jev-turbo memory-lint ikea/.sightmap` flags three of the map's seventeen
notes as prescriptive; all three are about overlays and the add flow and
were kept on purpose after reading them. The lint is a prompt to read, not a
rule.

Commands:

```bash
cd bench
sightmap browser start --detach --headless --url https://www.ikea.com/us/en/ --sightmap-dir ikea/.sightmap \
  --profile ~/.sightmap/profiles/explore-ikea-3 --port 7957 --cdp-port 7958 --chrome-flag=--window-size=1200,900
jev-turbo bench ikea-variants.json --repeat 5 --no-map --out results/ikea-variants-no-map.json
jev-turbo bench ikea-variants.json --repeat 5 --out results/ikea-variants-map.json
jev-turbo bench ikea-variants.json --repeat 5 --no-memory --out results/ikea-variants-map-no-memory.json
jev-turbo bench ikea-variants.json --only kallax-2x2-white --no-map --record ../out/v-no-map --out results/ikea-variants-demo-no-map.json
jev-turbo bench ikea-variants.json --only kallax-2x2-white --record ../out/v-map --out results/ikea-variants-demo-map.json
cd ikea && sightmap browser stop --port 7957 && cd ..

jev-turbo score results/ikea-variants-no-map.json results/ikea-variants-map.json results/ikea-variants-map-no-memory.json
jev-turbo memory-lint ikea/.sightmap
python3 ../scripts/render-demo.py --compare "without a map=../out/v-no-map" "with a map=../out/v-map" ../docs/demo \
  --title "One exact variant from a listing of sixteen same-named add buttons." \
  --subtitle "The white 2x2 KALLAX into the bag. Without a map: the product page, 5 steps. With the map: the card's own button, 4 steps, every pick sure."
```

## IKEA (2026-09-19)

One goal on ikea.com, the KALLAX shelf unit in white into the shopping bag,
run five times with `--no-map` and five times with the map in
`ikea/.sightmap`. Jev `jev-latest`, headless, in one session on a fresh
profile with a 1200×900 window. The tools column is the 2026-09-18 run over
the earlier map; the tool layer was not rerun.

```
condition=no-map jev:jev-latest: 5/5 ok · 25 steps · 72.8s total · 2.91s/step · model 30 calls, 247 ms/call, 226833+11471 tok · wasted 0 · fallback 0 · low-conf 5 · candidates ~607
condition=map jev:jev-latest: 5/5 ok · 25 steps · 69.2s total · 2.77s/step · model 30 calls, 286 ms/call, 387512+17447 tok · wasted 0 · fallback 0 · low-conf 7 · candidates ~608
```

```
                      ikea/no-map/jev:jev-latest  ikea/map/jev:jev-latest  ikea/tools/jev:jev-latest
reached               5/5                         5/5                      5/5
steps / goal          4                           4                        3
wasted steps          0                           0                        0
fallback picks        -                           0                        5
low-confidence picks  5                           7                        2
candidates offered    607                         608                      563
low-coverage pages    -                           1                        1
seconds               72.8                        69.2                     34.7
```

Every run takes the same four steps: type into the search field, click the
"kallax shelf unit white" suggestion, Add to cart on the results page, open
the bag. With the map each of those is a named pick (SearchField,
SearchSuggestion, ProductCard AddToCartButton, BagLink); the add is picked at
0.97 to 0.99, against 0.76 to 0.86 from the raw tree. The seven low-confidence
picks are the suggestion click at 0.52 to 0.62, where Enter is the other
plausible option. The goal has no room left for the map to win on steps: four
is the floor for a search, a pick, an add and the bag.

### What the 2026-09-18 runs got wrong

The first runs took 8 steps a goal without the map and 15 with it:

```
                      ikea/no-map/jev:jev-latest  ikea/map/jev:jev-latest
reached               5/5                         5/5
steps / goal          8                           15
wasted steps          5                           16
fallback picks        -                           44
low-confidence picks  18                          50
candidates offered    423                         423
low-coverage pages    -                           7
seconds               71.6                        130.2
```

Every run in both conditions opened the same way: fill the search field, fill
it again, press Enter, and the URL never changed. A live session showed the
fill never landed. IKEA's OneTrust banner holds keyboard focus, a synthetic
click does not move it, and the driver's combobox path typed wherever focus
was: into the banner. The field stayed empty, so Jev typed again, and Enter
on an empty search box does nothing. The map's own memory blamed the
suggestion dropdown for stealing focus, which was wrong.

After the failed search the raw tree just took the Products menu into a
category page and added KALLAX from its card. The map said not to: its Home
view read "nothing on it is needed for the bag flow beyond the header
globals", its memory said the flow is search → product page, and a BILLY-era
note said cards on rails are not a way to reach a named variant. Jev obeyed,
clicked the photo-search button and the IKEA Home link at 0.15 to 0.45
confidence, and got to a category page five to eight steps later than the raw
tree. On the pages it did reach, the map named 4 of 587 interactive controls
on Home and none on the category pages; the Search view, with 37 named
controls, was never reached. Where the map did cover a page (two runs landed
on a Product view) Jev took VariantOption, AddToBagButton and BagLink at 1.00
each.

Two changes, both in this repo. The driver verifies that a fill landed in the
field it was asked to fill and sets the value there when it did not, and Enter
is pressed in the field the last fill typed into. The map gained a Category
view (`/us/en/cat/**`), the header menu (NavEntry, MenuTab, MenuLink), and a
shared ProductCard with an AddToCartButton, used by the Search and Category
views alike; the memory that forbade the category route is gone, and the
SearchField note says what to do when a search does not run.

The suite's earlier goal was the BILLY bookcase. The home page carries a BILLY
rail with an Add button, so a raw tree reached that goal in three actions, and
the goal was replaced with the KALLAX shelf unit, which has to be searched for.

The README video is now the variants goal (see above). Two single runs of
this goal, one per condition, are in `results/ikea-demo-no-map.json` and
`results/ikea-demo-map.json`; the 2026-09-18 tools recording is still in
`results/ikea-demo-tools.json`.

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
python3 ../scripts/render-demo.py --compare "without a map=../out/ikea-no-map" "with a map=../out/ikea-map" ../docs/demo \
  --title "4 steps without a map, 4 with it. With the map, every pick is a named component." \
  --subtitle "One goal on ikea.com, same model, same loop. Add to cart picked at 0.97 with the map, 0.81 without."
# the three-act version, once a tools act is recorded:
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
