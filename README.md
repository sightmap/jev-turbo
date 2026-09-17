# jev-turbo

**Browser use where a 200 ms model picks every step.**

Give it one goal. A [sightmap](https://sightmap.org) turns the page into a short list of named actions. [Jev](https://docs.typesafe.ai/introduction), TypeSafe's typed-answer model, picks one and says whether the goal is met. No large model is called to act, so a step takes well under a second.

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

No map of the site yet? Point `--sightmap-dir` at an empty directory and add `--grow`. jev-turbo names the controls it meets as it goes, and the result is a plain `.sightmap/` directory that any other agent can read. `--start` launches the browser session for you. The session is a headed Chrome, and jev-turbo brings its tab to the front when it connects, so run it on a display you are not typing on.

## What happens in a step

1. The sightmap library reads the page over CDP and matches it against the map. Every visible control becomes a candidate, described by its component name and properties: `[OriginField]`, `[Option label="Zurich Airport (ZRH)"]`.
2. Jev answers two questions in one request: which candidate, and whether the goal is met. It can also pick back, scroll, wait, or Enter.
3. jev-turbo performs the action and waits for the page to settle.

Jev never writes text. Everything the loop types comes from `--value` or a spec file. If you would rather have a model write the spec, `--plan` calls Claude once per goal. That is the only large-model call in the tool, and it is optional.

## Numbers

Three sites, ten goals each on the first two, one goal run five times on the third. The right column is the same loop with Claude Sonnet picking instead of Jev, for scale.

| site | Jev | Claude Sonnet in the same seat |
|---|---|---|
| saucedemo.com, 39-component map | 10/10, 0.24 s a step | 10/10, 1.16 s a step, $0.25 |
| books.toscrape.com, no map | 10/10, 0.30 s a step | 9/10, 2.54 s a step, $0.81 |
| Google Flights, Zürich to London | 5/5, median 8.4 s a run | 1/1, 61.6 s, $0.29 |

Growing books.toscrape.com from an empty map takes one pass: 10/10 goals, 89% of the visited pages covered after the first pass and 100% after the second. Suites, maps, and every run file are in [`bench/`](bench/README.md).

## Commands

```
jev-turbo explore --goal "..." [--done-when view=Cart] [--value user=alice] [--avoid Delete] [--plan] [--grow] [--record DIR]
jev-turbo bench   SUITE.json [--repeat N] [--picker jev|anthropic] [--grow] [--record DIR]
jev-turbo plan    --goal "..." [--site host]
jev-turbo graph   [RUN.json ...]
```

`--done-when` is a deterministic finish check: `view=NAME`, `url=SUBSTR`, `text=SUBSTR`, `component=NAME`, or `prop=Comp.name~value`. Repeat it to AND checks. Without one, the loop stops when Jev's own "done" answer passes 0.85. A spec file carries the same in JSON:

```json
{ "done_when": { "view": "Cart" }, "values": { "username": "standard_user", "password": "secret_sauce" }, "avoid": ["Delete", "Pay"] }
```

`--avoid` drops matching controls from what Jev can pick. On a real account it is the only guard, so use it.

`--record DIR` captures frames while a goal runs, and `scripts/render-demo.py DIR out/name` turns them into the MP4 and GIF above.

## Limits

- The goals here are 2 to 12 steps on cooperative sites. None needed backtracking, a modal that eats clicks, or an infinite feed.
- Jev picks from what the sightmap library can see: HTML and ARIA controls. Canvas, frames, and file uploads are out.
- Snapshotting a Google Flights page costs 250 to 350 ms a step, and that is most of the gap to a loop tuned for one page.

## Related work

[browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast) runs Jev over a raw element table and has a small LLM write the typed text. Its Google Flights demo is the task in `bench/flights.json`.

## License

MIT. Jev is TypeSafe's model; sightmap is a Fullstory project.
