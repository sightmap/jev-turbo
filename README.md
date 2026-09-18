# jev-turbo

**Browser use where a 200 ms model picks every step, measured with and without a map of the site.**

Give it one goal. A [sightmap](https://sightmap.org) turns the page into a short list of named actions. [Jev](https://docs.typesafe.ai/introduction), TypeSafe's typed-answer model, picks one and says whether the goal is met. No large model is called to act. jev-turbo runs that loop with the map and without it, over the same site and the same model, and reports what the map changed.

<a href="docs/demo.mp4"><img src="docs/demo.gif" alt="One-way Zürich to London on Google Flights at 1x: nine actions in 8.9 seconds, every one picked by Jev over a 14-component sightmap" width="100%" /></a>

One-way Zürich to London on Google Flights at 1x: nine actions in 8.9 seconds, loading waits included. [MP4](docs/demo.mp4) · [The run](bench/results/flights-demo-run.json) · [All benchmarks](bench/README.md)

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

No map of the site yet? Point `--sightmap-dir` at an empty directory and add `--grow`. jev-turbo names the controls it meets as it goes, and the result is a plain `.sightmap/` directory that any other agent can read. `--start` launches the browser session for you. The session is a headed Chrome by default. jev-turbo never brings it to the front, but Chrome takes focus once when it launches; add `--headless` to `sightmap browser start` (or to `--start`) to keep it off your screen.

## What happens in a step

1. The sightmap library reads the page over CDP and matches it against the map. Every visible control becomes a candidate, described by its component name and properties: `[OriginField]`, `[Option label="Zurich Airport (ZRH)"]`.
2. Jev answers two questions in one request: which candidate, and whether the goal is met. It can also pick back, scroll, wait, or Enter.
3. jev-turbo performs the action and waits for the page to settle.

Jev never writes text. Everything the loop types comes from `--value` or a spec file. If you would rather have a model write the spec, `--plan` calls Claude once per goal. That is the only large-model call in the tool, and it is optional.

## What the map changed

Same loop, same site, same model, run once with a sightmap and once with `--no-map` so Jev sees the raw HTML and ARIA tree instead.

| suite | condition | goals reached | steps per reached goal | wasted steps | unsure picks | seconds |
|---|---|---|---|---|---|---|
| saucedemo, 10 goals ×3 | map | 30/30 | 4.5 | 3 | 0 | 48.4 |
| saucedemo, 10 goals ×3 | no map | 30/30 | 4.5 | 3 | 2 | 45.6 |
| journeys, 2 long goals ×3 | map | 3/6 | 13 | 63 | 34 | 53.8 |
| journeys, 2 long goals ×3 | no map | 3/6 | 13 | 53 | 49 | 53.4 |
| Google Flights, Zürich to London ×10 | map | 10/10 | 10 | 7 | 4 | 116.3 |
| Google Flights, Zürich to London ×10 | no map | 10/10 | 15 | 12 | 47 | 143.2 |

Unsure picks are picks Jev gave under 60% probability; on a page large enough to be answered as a group and then an option inside it, that is the two probabilities multiplied. Wasted steps are a stale click, a back, or a repeat of a control already used on that page.

On all three suites, both conditions reached the same goals. The map did not decide whether a goal was reached. It changed the steps, the unsure picks, and the time on Google Flights. Steps per reached goal went from 15 to 10. Unsure picks went from 47 to 4. The ten runs took 116 seconds instead of 143. On saucedemo, whose raw tree already carries good names, the map changed little: 30 of 30 goals, 4.5 steps and 3 wasted steps either way, two unsure picks without the map against none with it, and three seconds between the two conditions. Two earlier no-map runs failed from bugs in this loop, not from the map. Saucedemo reached 0 of 30 because a button inside a same-named form was dropped from the candidates, and Google Flights reached 1 of 10 because a suggestion list was observed before its entries reached the tree. Both bugs are fixed on this branch. In the long goals, `cheapest-two` passes both ways and `three-orders` fails both ways at the 45-step cap, because the loop has no way to run a flow a second time. That is a limit of the loop, not something the map caused.

Suites, maps, and every run file, including the Jev-versus-Claude-Sonnet comparison, are in [`bench/`](bench/README.md).

## Commands

```
jev-turbo explore --goal "..." [--done-when view=Cart] [--value user=alice] [--avoid Delete] [--plan] [--grow] [--record DIR] [--no-map]
jev-turbo bench   SUITE.json [--repeat N] [--picker jev|anthropic] [--grow] [--record DIR] [--no-map]
jev-turbo score   RESULT.json [RESULT.json ...]   # one column per file
jev-turbo plan    --goal "..." [--site host]
jev-turbo graph   [RUN.json ...]
```

`--done-when` is a deterministic finish check: `view=NAME`, `url=SUBSTR`, `text=SUBSTR`, `component=NAME`, `prop=Comp.name~value`, or `history_count=N:SUBSTR` (at least N earlier steps mention SUBSTR). Repeat it to AND checks. Without one, the loop stops when Jev's own "done" answer passes 0.85. A spec file carries the same in JSON:

```json
{ "done_when": { "view": "Cart" }, "values": { "username": "standard_user", "password": "secret_sauce" }, "avoid": ["Delete", "Pay"] }
```

`--avoid` drops matching controls from what Jev can pick. On a real account it is the only guard, so use it.

`--record DIR` captures frames while a goal runs, and `scripts/render-demo.py DIR out/name` turns them into the MP4 and GIF above.

## Limits

- The short goals are 2 to 12 steps on cooperative sites; the two long ones cap at 30 and 45 steps, and one of them fails both with and without the map because the loop cannot repeat a flow.
- Jev picks from what the sightmap library can see: HTML and ARIA controls. Canvas, frames, and file uploads are out.
- Snapshotting a Google Flights page costs 250 to 350 ms a step, and that is most of the gap to a loop tuned for one page.
- After a value is typed with the native setter, pressing Enter submits a form only some of the time in headless Chrome. Click the submit control instead.

## Related work

[browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast) runs Jev over a raw element table and has a small LLM write the typed text. Its Google Flights demo is the task in `bench/flights.json`.

## License

MIT. Jev is TypeSafe's model; sightmap is a Fullstory project.
