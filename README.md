# jev-turbo

Browser use where [Jev](https://docs.typesafe.ai/introduction) picks every step over a [sightmap](https://sightmap.org). Sub-second steps. No big-model call to act.

A sightmap turns a page into a short list of named actions. Jev only answers typed questions: pick one of these, yes or no. Put them together and "what do I click next?" is a multiple-choice question a 200 ms model answers well.

```
observe the annotated tree (10–70 ms) → named candidates → Jev picks one + judges "done?" (150–250 ms) → act (<50 ms) → settle (~120 ms)
```

The loop never invents text. Every value it types comes from `--value` or a spec file. A big model is called at most once per goal, to write that spec (`--plan`), and you can skip it.

`--grow` builds the sightmap while it explores: unmapped controls are grouped by their container, Jev classifies them, a template names them, the selector is verified, the YAML lands in the corpus with validation after every write. An empty corpus reaches full coverage of the visited pages in one pass.

## Numbers

Same loop, both pickers, ten goals a site. The Claude rows are Claude Sonnet asked for the same pick as a JSON reply, a bare picker rather than a full agent.

| site | picker | goals | wall / step | model / call | Anthropic bill |
|---|---|---|---|---|---|
| saucedemo.com, 39-component corpus | jev-latest | 10/10 | 0.24 s | 160 ms | $0 |
| saucedemo.com | claude-sonnet-5 | 10/10 | 1.16 s | 1,230 ms | $0.25 |
| books.toscrape.com, empty corpus | jev-latest | 10/10 | 0.30 s | 184 ms | $0 |
| books.toscrape.com | claude-sonnet-5 | 9/10 | 2.54 s | 1,802 ms | $0.81 |

The 12-step saucedemo checkout runs in 3.2 s. Growing books.toscrape.com from an empty corpus: three passes, 30/30 goals, 0% → 89% → 100% coverage of the visited pages, `validate` and `lint` clean, 1.2 s of grow work. Suites, corpora, run files, and the commands are in [`bench/`](bench/README.md).

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

No corpus yet? Point `--sightmap-dir` at an empty directory and add `--grow`. `--start` runs `sightmap browser start --detach` for you when no session exists. `--picker anthropic` puts Claude in the same seat (`ANTHROPIC_API_KEY`).

Or from source: `go install github.com/sightmap/jev-turbo/cmd/jev-turbo@latest`. Inside a clone of this repo, `npx @sightmap/jev-turbo` resolves to the local `npm/` workspace package, which carries no binary; run `go run ./cmd/jev-turbo` there instead.

## Commands

```
jev-turbo explore --goal "..." [--done-when view=Cart] [--value user=alice] [--avoid Delete] [--picker jev|anthropic] [--plan] [--grow] [--max-steps N] [--json]
jev-turbo bench   SUITE.json [--repeat N] [--only NAME] [--out FILE] [--picker jev|anthropic] [--grow]
jev-turbo plan    --goal "..." [--site host]
jev-turbo graph   [RUN.json ...]
```

`--done-when` is a deterministic finish check, repeatable and ANDed: `view=NAME`, `url=SUBSTR`, `text=SUBSTR`, `component=NAME`, `history=SUBSTR`, or `prop=Comp.name~value[@Within.name~value]`. Without one, the loop stops when Jev's own "done" judgment passes 0.85. A spec file carries the same in JSON:

```json
{ "done_when": { "view": "Cart" }, "values": { "username": "standard_user", "password": "secret_sauce" }, "avoid": ["Delete", "Pay"] }
```

Use `--avoid` on any real account. It drops matching controls from the candidate list, and it is the only guard.

## How a step works

1. `observe.Page` from the sightmap library extracts the tree over CDP and matches the corpus, in one call over one connection held for the whole run.
2. Visible interactive nodes become candidates, described as `[Component prop="value"] role "name"`. Pages with more than 60 candidates fold into one option per owning component or landmark, plus up to 20 elements whose name or link matches a word of the goal; a group answer triggers a second pick inside the group. A control acted on twice at one URL is hidden.
3. Jev answers two questions in one request: which option, and whether the goal is already met.
4. Text fields take the spec value whose key overlaps the field's name; ties go to Jev. Selects list their options for Jev. Everything else is a DOM click, with the real mouse path as the fallback.
5. Settle: readyState complete and two samples 40 ms apart agree on URL, mutation count, and text length, capped at 3 s.
6. A re-rendered element is re-observed and retried once by description, then skipped.

## What Jev does not do here

Invent text to type, read an answer back as prose, name components, or plan multi-goal work. Values come from the spec. Answers come from sightmap properties. Names come from templates. Planning is one optional big-model call.

Related: [browser-use/jev-ultrafast](https://github.com/browser-use/jev-ultrafast) runs the same idea over a raw element table with an operation head plus target heads in one request and a small text model for typed input. jev-turbo offers sightmap names and properties as the options and takes typed values from a spec.

## License

MIT. Jev is TypeSafe's model; sightmap is a Fullstory project.
