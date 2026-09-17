# jev-turbo

Browser use where [Jev](https://docs.typesafe.ai/introduction) picks every step over a [sightmap](https://sightmap.org). Sub-second steps. No big-model call to act.

<img src="docs/demo.gif" alt="One-way Zürich to London on Google Flights: nine actions, 8.9 seconds at 1x, every step picked by Jev over a 14-component sightmap" width="100%" />

One-way Zürich to London on Google Flights, September 20, 2026: nine actions in 8.9 s at 1x, loading waits included, with Jev choosing every step. [MP4](docs/demo.mp4) · [the run](bench/results/flights-demo-run.json) · [numbers](#numbers)

A sightmap turns a page into a short list of named actions. Jev only answers typed questions: pick one of these, yes or no. Put them together and "what do I click next?" is a multiple-choice question a 200 ms model answers well.

```
observe the annotated tree (10–350 ms) → named candidates → Jev picks one + judges "done?" (150–300 ms) → act (<150 ms) → settle (~100 ms)
```

The loop never invents text. Every value it types comes from `--value` or a spec file. A big model is called at most once per goal, to write that spec (`--plan`), and you can skip it.

`--grow` builds the sightmap while it explores: unmapped controls are grouped by their container, Jev classifies them, a template names them, the selector is verified, the YAML lands in the corpus with validation after every write. An empty corpus reaches full coverage of the visited pages in one pass.

## Numbers

Same loop, both pickers. The Claude rows are Claude Sonnet asked for the same pick as a JSON reply, a bare picker rather than a full agent.

| site | picker | goals | wall / step | model / call | Anthropic bill |
|---|---|---|---|---|---|
| saucedemo.com, 39-component corpus, 10 goals | jev-latest | 10/10 | 0.24 s | 160 ms | $0 |
| saucedemo.com | claude-sonnet-5 | 10/10 | 1.16 s | 1,230 ms | $0.25 |
| books.toscrape.com, empty corpus, 10 goals | jev-latest | 10/10 | 0.30 s | 184 ms | $0 |
| books.toscrape.com | claude-sonnet-5 | 9/10 | 2.54 s | 1,802 ms | $0.81 |
| Google Flights, 14-component corpus, Zürich→London ×5 | jev-latest | 5/5, median 8.4 s a run | 0.83 s | 185 ms | $0 |
| Google Flights | claude-sonnet-5 | 1/1, 61.6 s | 3.62 s | 1,769 ms | $0.29 |

The 12-step saucedemo checkout runs in 3.2 s. Growing books.toscrape.com from an empty corpus: three passes, 30/30 goals, 0% → 89% → 100% coverage of the visited pages, `validate` and `lint` clean, 1.2 s of grow work. Suites, corpora, run files, and the commands are in [`bench/`](bench/README.md).

### Google Flights, side by side with jev-ultrafast

[browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast) demos the same task on the same site: one-way Zürich to London, September 20, 2026, one adult, economy, timed at 1x with loading included. They report a 7.1 s run and a 7.1 s median over three runs. jev-turbo's five runs came in at 7.9, 8.3, 8.4, 8.6, and 10.9 s, nine actions each: origin, suggestion, destination, suggestion, date, trip type, "One way", Search, wait.

What differs. Their loop answers "operation plus target" in one request over a raw element table and lets a small text model write each field; it is tuned for this page, with suggestion waits capped at 200 ms. jev-turbo offers sightmap component names and properties as the options, takes typed values from a spec, and uses the same generic loop it uses on every other site. The corpus is 14 components on aria-label hooks plus six memory lines, written in half an hour, and it is what turned a wandering unmapped run into the same nine actions every time.

Where jev-turbo's seconds go: a snapshot of Google's 4,000-node tree costs 250 to 350 ms a step, Jev 150 to 300 ms, the final wait for results to render about 1.5 s. Cutting the snapshot to the visible subtree would bring a run under 7 s; that is library work in sightmap, not in this loop. An Atlanta to Zürich variant is not reliable yet: with the origin pre-filled Google swaps "Search" for "Explore" and opens a calendar on the date field, and the loop has no answer for that today.

## Try it

Bring your own keys. `TYPESAFE_API_KEY` is the only required one.

```bash
npm install -g @sightmap/jev-turbo        # brings the sightmap CLI along
export TYPESAFE_API_KEY=...

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

The flight search from the video, from a clean profile:

```bash
sightmap browser start --detach --url 'https://www.google.com/travel/flights?hl=en' --sightmap-dir bench/flights/.sightmap \
  --profile ~/.sightmap/profiles/flights --port 7931 --cdp-port 7932
jev-turbo bench bench/flights.json --record out/rec && python3 scripts/render-demo.py out/rec out/demo
```

No corpus yet? Point `--sightmap-dir` at an empty directory and add `--grow`. `--start` runs `sightmap browser start --detach` for you when no session exists. `--picker anthropic` puts Claude in the same seat (`ANTHROPIC_API_KEY`). The session is a headed Chrome that is brought to the front on every step; run it on a display you are not typing on.

Or from source: `go install github.com/sightmap/jev-turbo/cmd/jev-turbo@latest`. Inside a clone of this repo, `npx @sightmap/jev-turbo` resolves to the local `npm/` workspace package, which carries no binary; run `go run ./cmd/jev-turbo` there instead.

## Commands

```
jev-turbo explore --goal "..." [--done-when view=Cart] [--value user=alice] [--avoid Delete] [--picker jev|anthropic] [--plan] [--grow] [--max-steps N] [--json] [--record DIR]
jev-turbo bench   SUITE.json [--repeat N] [--only NAME] [--out FILE] [--picker jev|anthropic] [--grow] [--record DIR]
jev-turbo plan    --goal "..." [--site host]
jev-turbo graph   [RUN.json ...]
```

`--done-when` is a deterministic finish check, repeatable and ANDed: `view=NAME`, `url=SUBSTR`, `text=SUBSTR`, `component=NAME`, `history=SUBSTR`, or `prop=Comp.name~value[@Within.name~value]`. Without one, the loop stops when Jev's own "done" judgment passes 0.85. A spec file carries the same in JSON:

```json
{ "done_when": { "view": "Cart" }, "values": { "username": "standard_user", "password": "secret_sauce" }, "avoid": ["Delete", "Pay"] }
```

Use `--avoid` on any real account. It drops matching controls from the candidate list, and it is the only guard.

`--record DIR` writes JPEG frames, the step events, and the run while a goal runs; `scripts/render-demo.py DIR out/name` assembles them into `name.mp4` and `name.gif` at 1x with step captions (needs ffmpeg; Pillow for the captions).

## How a step works

1. `observe.Page` from the sightmap library extracts the tree over CDP and matches the corpus, in one call over one connection held for the whole run. Corpus memory lines for the site, the view, and the matched components travel to the picker as site notes.
2. Visible interactive nodes become candidates, described as `[Component prop="value"] role "name"`. Containers that hold other controls, presentational nodes, and unnamed wrappers are left out. Pages with more than 60 candidates fold into one option per owning component or landmark, plus up to 20 elements whose name or link matches a word of the goal; a group answer triggers a second pick inside the group. A control acted on twice at one URL is hidden.
3. Jev answers two questions in one request: which option, and whether the goal is already met. Along with the elements it can pick back, scroll, wait, and, right after typing, Enter.
4. Text fields take the spec value whose key overlaps the field's name; ties go to Jev. Suggestion fields are typed with real keystrokes into whichever element takes focus, then the loop waits for entries that mention the typed text and offers only those next. Selects list their options for Jev. Everything else is a DOM click, with the real mouse path as the fallback.
5. Settle: readyState complete and two samples 40 ms apart agree on URL, mutation count, and text length, capped at 3 s. A "wait" polls the finish check while it waits.
6. A re-rendered element is re-observed and retried once by description, then skipped.

## What Jev does not do here

Invent text to type, read an answer back as prose, name components, or plan multi-goal work. Values come from the spec. Answers come from sightmap properties. Names come from templates. Planning is one optional big-model call.

## License

MIT. Jev is TypeSafe's model; sightmap is a Fullstory project.
