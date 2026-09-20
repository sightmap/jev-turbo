# jev-turbo

**Browser use where a 200 ms model picks every step, measured with and without a map of the site.**

Give it one goal. A [sightmap](https://sightmap.org) turns the page into a short list of named actions. [Jev](https://docs.typesafe.ai/introduction), TypeSafe's typed-answer model, picks one and says whether the goal is met. No large model is called to act. jev-turbo runs that loop with the map and without it, over the same site and the same model, and reports what the map changed.

<a href="docs/demo.mp4"><img src="docs/demo.gif" alt="One exact KALLAX variant into the bag on ikea.com at 1x, twice with the same model. Without a map: 5 steps through the product page, two unsure picks. With the map: 4 steps, the card's own add button, every pick at 1.00." width="100%" /></a>

The white 2x2 KALLAX into the bag on ikea.com at 1x, twice with the same model. The listing shows sixteen add buttons that all read `Add "KALLAX Shelf unit" to cart`. Without a map Jev cannot tell them apart, so it goes through the product page: 5 steps, two unsure picks. With the map each button belongs to a ProductCard with a title, so it takes the right one at 1.00: 4 steps. [MP4](docs/demo.mp4) · [The map run](bench/results/ikea-variants-demo-map.json) · [All benchmarks](bench/README.md)

## Three ways in

Three conditions over the same three goals: the raw tree, the map, and the map with its memory notes withheld.

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

On ikea.com the map pays where the tree is ambiguous. Three goals ask for one exact KALLAX variant from a listing where every add button carries the same name. Over five runs each: the raw tree reaches 10 of 15 with 7 steps a goal, 34 wasted steps and 79 unsure picks; the map reaches 15 of 15 with 5 steps a goal, 5 wasted and 2 unsure. The two-variant goal the raw tree never finishes in 20 steps; the map does it in 6. The same map with its memory notes removed still reaches 15 of 15 but with 34 unsure picks, so the notes buy confidence, not steps. The first IKEA runs went the other way, 15 steps a goal with the map against 8 without, and the reasons are in the bench README: a fill that never landed, and notes written for an earlier goal that steered Jev off the working route. The score is what pointed at both problems, and it now also counts steps that had no effect and candidates that share a name.

## Install

```bash
npm install -g @sightmap/jev-turbo   # installs the sightmap CLI with it
export TYPESAFE_API_KEY=...          # the only key you need
```

Or `go install github.com/sightmap/jev-turbo/cmd/jev-turbo@latest`.

## Run a goal

Start a browser session on the site, then give jev-turbo a goal, a finish check, and the values it may type:

```bash
sightmap browser start --detach --url https://www.saucedemo.com/ --sightmap-dir bench/saucedemo/.sightmap
jev-turbo explore --sightmap-dir bench/saucedemo/.sightmap \
  --goal "Log in and put the Sauce Labs Backpack in the cart, then open the cart" \
  --done-when view=Cart --value username=standard_user --value password=secret_sauce
```

```text
 1. Login  filled [UsernameField] with username  [n29:1.00 back:0.00]  done=0.01  321ms (snap 13, pick 149, act 36, settle 122)
 2. Login  filled [PasswordField] with password  [n31:1.00 back:0.00]  done=0.01  297ms (snap 8, pick 136, act 30, settle 122)
 3. Login  clicked [LoginButton]  [n33:1.00 back:0.00]  done=0.01  282ms (snap 7, pick 142, act 10, settle 122)
 4. Inventory  clicked [AddToCartButton label="Add to cart"]  [n91:1.00 back:0.00]  done=0.01  297ms (snap 17, pick 149, act 7, settle 123)
 5. Inventory  clicked [CartLink count="Cart, 1 items"]  [n55:1.00 back:0.00]  done=0.02  335ms (snap 28, pick 181, act 5, settle 121)
 6. Cart  done
OK  done_when satisfied  steps=6  1.5s  picker=jev:jev-latest calls=5 732ms tokens=6120+610
```

No map of the site yet? Point `--sightmap-dir` at an empty directory and add `--grow`. jev-turbo scaffolds coverage as it goes: it scopes each new component to the nearest stable ancestor the page already has, verifies the selector with the offline matcher, names the component from a template, reads off `label`, `href` and `placeholder` as properties, and names views from their routes. Any property beyond those three, a memory line, or a name a person would choose still comes from the authoring skill or a curator. `--start` launches the browser session for you. The session is a headed Chrome by default. jev-turbo never brings it to the front, but Chrome takes focus once when it launches; add `--headless` to `sightmap browser start` (or to `--start`) to keep it off your screen.

## What happens in a step

1. The sightmap library reads the page over CDP and matches it against the map. Every visible control becomes a candidate, described by its component name and properties: `[OriginField]`, `[Option label="Zurich Airport (ZRH)"]`.
2. Jev answers two questions in one request: which candidate, and whether the goal is met. It can also pick back, scroll, wait, or Enter.
3. jev-turbo performs the action and waits for the page to settle.

With `--tools DIR`, the tools of a sightkick layer are offered ahead of the page's elements: `log_in(username, password)`, `add_to_cart(name)`. A picked tool runs through `sightkick call`, and its guidance puts the suggested next tools first. If no tool fits the page or a call fails, the step falls back to elements.

Jev never writes text. Everything the loop types comes from `--value` or a spec file. If you would rather have a model write the spec, `--plan` calls Claude once per goal. That is the only large-model call in the tool, and it is optional.

## What the map changed

Same loop, same site, same model, run once with a sightmap and once with `--no-map` so Jev sees the raw HTML and ARIA tree instead. The first ikea.com goal also has a third condition, the map's sightkick tools. A fourth, `--no-memory`, keeps the map's components and views and withholds only its memory notes, so a run with it against a plain map run says whether the site's prose helped or hurt; `jev-turbo memory-lint DIR` flags the notes that prescribe a route before a run has to.

| suite | condition | goals reached | steps per reached goal | wasted steps | unsure picks | seconds |
|---|---|---|---|---|---|---|
| saucedemo, 10 goals ×3 | map | 30/30 | 4.5 | 3 | 0 | 48.4 |
| saucedemo, 10 goals ×3 | no map | 30/30 | 4.5 | 3 | 2 | 45.6 |
| journeys, 2 long goals ×3 | map | 3/6 | 13 | 63 | 34 | 53.8 |
| journeys, 2 long goals ×3 | no map | 3/6 | 13 | 53 | 49 | 53.4 |
| Google Flights, Zürich to London ×10 | map | 10/10 | 10 | 7 | 4 | 116.3 |
| Google Flights, Zürich to London ×10 | no map | 10/10 | 15 | 12 | 47 | 143.2 |
| ikea.com, KALLAX into the bag ×5 | map | 5/5 | 4 | 0 | 7 | 69.2 |
| ikea.com, KALLAX into the bag ×5 | no map | 5/5 | 4 | 0 | 5 | 72.8 |
| ikea.com, KALLAX into the bag ×5 | tools | 5/5 | 3 | 0 | 2 | 34.7 |
| ikea.com, 3 exact-variant goals ×5 | map | 15/15 | 5 | 5 | 2 | 198.8 |
| ikea.com, 3 exact-variant goals ×5 | no map | 10/15 | 7 | 34 | 79 | 337.0 |
| ikea.com, 3 exact-variant goals ×5 | no memory | 15/15 | 6 | 7 | 34 | 222.0 |

Unsure picks are picks Jev gave under 60% probability; on a page large enough to be answered as a group and then an option inside it, that is the two probabilities multiplied. Wasted steps are a stale click, a back, or a repeat of a control already used on that page.

On the first four suites, both conditions reached the same goals. There the map did not decide whether a goal was reached. It changed the steps, the unsure picks, and the time on Google Flights. Steps per reached goal went from 15 to 10. Unsure picks went from 47 to 4. The ten runs took 116 seconds instead of 143. On saucedemo, whose raw tree already carries good names, the map changed little: 30 of 30 goals, 4.5 steps and 3 wasted steps either way, two unsure picks without the map against none with it, and three seconds between the two conditions. Two earlier no-map runs failed from bugs in this loop, not from the map. Saucedemo reached 0 of 30 because a button inside a same-named form was dropped from the candidates, and Google Flights reached 1 of 10 because a suggestion list was observed before its entries reached the tree. Both bugs are fixed on this branch. In the long goals, `cheapest-two` passes both ways and `three-orders` fails both ways at the 45-step cap, because the loop has no way to run a flow a second time. That is a limit of the loop, not something the map caused.

On ikea.com the first goal, any KALLAX into the bag, is four steps either way once the search submits, and the map's sightkick tools take it in three. The three exact-variant goals are where the map pays: 15 of 15 reached against 10 of 15, five steps a goal against seven, and two unsure picks against 79, because sixteen add buttons on the listing share one name and only the map ties each to its card. The same map without its notes reaches every goal too but is unsure 34 times, so on this map the memory buys confidence rather than steps. The first IKEA runs, before the fill fix and the map rewrite, took 15 steps a goal with the map against 8 without; that history is kept in the bench README.

Suites, maps, and every run file, including the Jev-versus-Claude-Sonnet comparison, are in [`bench/`](bench/README.md).

## Commands

```
jev-turbo explore --goal "..." [--done-when view=Cart] [--value user=alice] [--avoid Delete] [--plan] [--grow] [--tools DIR] [--record DIR] [--no-map] [--no-memory]
jev-turbo bench   SUITE.json [--repeat N] [--picker jev|anthropic] [--grow] [--tools DIR] [--record DIR] [--no-map] [--no-memory]
jev-turbo score   RESULT.json [RESULT.json ...]   # one column per file
jev-turbo plan    --goal "..." [--site host]
jev-turbo graph   [RUN.json ...]
jev-turbo memory-lint DIR                          # memory notes that prescribe a route instead of describing the page; exit 1 when any
```

`--done-when` is a deterministic finish check: `view=NAME`, `url=SUBSTR`, `text=SUBSTR`, `text_absent=SUBSTR`, `text_count=N:SUBSTR` (SUBSTR shows exactly N times; `N+:` at least, `N-M:` between), `component=NAME`, `prop=Comp.name~value`, or `history_count=N:SUBSTR` (at least N earlier steps mention SUBSTR). Repeat it to AND checks. Without one, the loop stops when Jev's own "done" answer passes 0.85. A spec file carries the same in JSON:

```json
{ "done_when": { "view": "Cart" }, "values": { "username": "standard_user", "password": "secret_sauce" }, "avoid": ["Delete", "Pay"] }
```

`--avoid` drops matching controls from what Jev can pick. On a real account it is the only guard, so use it.

`--record DIR` captures frames while a goal runs, and `scripts/render-demo.py DIR out/name` turns them into an MP4 and a GIF. `scripts/render-demo.py --compare "LABEL=DIR" ... out/name` renders several recordings as acts of one video, which is how the three acts above were made.

## Limits

- The short goals are 2 to 12 steps on cooperative sites; the two long ones cap at 30 and 45 steps, and one of them fails both with and without the map because the loop cannot repeat a flow.
- Jev picks from what the sightmap library can see: HTML and ARIA controls. Canvas, frames, and file uploads are out.
- Snapshotting a Google Flights page costs 250 to 350 ms a step, and that is most of the gap to a loop tuned for one page.
- Enter is pressed in the field the last fill typed into. It submits the field's form the way a keyboard does, which is not always what a site's own submit control does; when a search does not run, clicking that control is the fallback.

## Related work

[browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast) runs Jev over a raw element table and has a small LLM write the typed text. Its Google Flights demo is the task in `bench/flights.json`.

## License

MIT. Jev is TypeSafe's model; sightmap is a Fullstory project.
