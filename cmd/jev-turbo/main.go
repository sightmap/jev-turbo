// jev-turbo drives a browser toward a goal with a typed model picking every
// step over a sightmap-annotated page. It connects to a running
// `sightmap browser start` session and needs no big-model call to act.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sightmap/jev-turbo/explore"
	"github.com/sightmap/jev-turbo/grow"
	"github.com/sightmap/sightmap/go/browser"
	"github.com/sightmap/sightmap/go/sightmap"
)

// version is set by goreleaser (-X main.version=...).
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "explore":
		err = runExplore(os.Args[2:])
	case "bench":
		err = runBench(os.Args[2:])
	case "score":
		err = runScore(os.Args[2:])
	case "plan":
		err = runPlan(os.Args[2:])
	case "graph":
		err = runGraph(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("jev-turbo", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "jev-turbo: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		if err == flag.ErrHelp {
			return
		}
		if err != errNotDone {
			fmt.Fprintf(os.Stderr, "jev-turbo: %v\n", err)
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `jev-turbo — browser use where Jev picks every step over a sightmap

Commands:
  explore --goal "..." [--done-when view=Cart] [--value user=alice] [--picker jev|anthropic] [--plan] [--grow] [--no-map] [--record DIR]
  bench   SUITE.json [--repeat N] [--only NAME] [--out FILE] [--picker jev|anthropic] [--grow] [--no-map] [--record DIR]
  score   RUN.json [RUN.json ...]             one column per run file
  plan    --goal "..." [--site host]          print the spec the planner would write (ANTHROPIC_API_KEY)
  graph   [RUN.json ...]                       print the transitions observed in run files
  version

Session flags (explore, bench):
  --sightmap-dir DIR   corpus dir; its .session file locates the running Chrome (default .sightmap)
  --addr HOST:PORT     CDP address, overrides the session file
  --tab ID             tab to drive when several are open
  --url URL            navigate here first
  --start              run 'sightmap browser start --detach' when no session exists (needs sightmap on PATH)
  --headless           with --start, launch the session headless

Keys: TYPESAFE_API_KEY (Jev, required), ANTHROPIC_API_KEY (planner and the anthropic picker).
`)
}

var errNotDone = fmt.Errorf("goal not reached")

/* ---------------- session flags ---------------- */

type liveFlags struct {
	dir      *string
	addr     *string
	tab      *string
	url      *string
	wait     *float64
	start    *bool
	headless *bool
}

func addLiveFlags(fs *flag.FlagSet) *liveFlags {
	return &liveFlags{
		dir:      fs.String("sightmap-dir", ".sightmap", "Path to the .sightmap/ corpus (its .session file locates Chrome)"),
		addr:     fs.String("addr", "", "CDP address host:port (default: the session recorded for --sightmap-dir)"),
		tab:      fs.String("tab", "", "Tab id from 'sightmap browser status' when several tabs are open"),
		url:      fs.String("url", "", "Navigate to this URL before starting"),
		wait:     fs.Float64("wait", 0, "Extra seconds to wait after navigation"),
		start:    fs.Bool("start", false, "Start a session with 'sightmap browser start --detach' when none is running"),
		headless: fs.Bool("headless", false, "With --start, launch the session headless"),
	}
}

func (lf *liveFlags) cdpAddr() string {
	if *lf.addr != "" {
		return *lf.addr
	}
	if info, err := browser.ReadSessionInfo(*lf.dir); err == nil && info.Port > 0 {
		return fmt.Sprintf("localhost:%d", info.Port)
	}
	return ""
}

func (lf *liveFlags) connect(ctx context.Context) (*browser.CDPConn, error) {
	addr := lf.cdpAddr()
	if addr == "" && *lf.start {
		args := []string{"browser", "start", "--detach", "--sightmap-dir", *lf.dir}
		if *lf.url != "" {
			args = append(args, "--url", *lf.url)
		}
		if *lf.headless {
			args = append(args, "--headless")
		}
		cmd := exec.Command("sightmap", args...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("--start: sightmap browser start: %w", err)
		}
		addr = lf.cdpAddr()
	}
	if addr == "" {
		return nil, fmt.Errorf("no session for %s: run 'sightmap browser start --detach --sightmap-dir %s' or pass --start / --addr", *lf.dir, *lf.dir)
	}
	conn, err := browser.Connect(addr, *lf.tab)
	if err != nil {
		return nil, err
	}
	if *lf.url != "" {
		if err := browser.NavigateAndWaitIdle(ctx, conn, *lf.url, 8*time.Second); err != nil {
			conn.Close()
			return nil, fmt.Errorf("navigate: %w", err)
		}
	}
	if *lf.wait > 0 {
		time.Sleep(time.Duration(*lf.wait * float64(time.Second)))
	}
	return conn, nil
}

func (lf *liveFlags) corpus() (*sightmap.Corpus, error) {
	if _, err := os.Stat(*lf.dir); err != nil {
		return nil, nil
	}
	c, err := sightmap.Load(*lf.dir)
	if err != nil {
		return nil, fmt.Errorf("load corpus %s: %w", *lf.dir, err)
	}
	return c, nil
}

// parseInterspersed lets flags follow positionals: flags (with their values)
// are gathered and parsed, positionals are returned in order.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

/* ---------------- explore ---------------- */

func runExplore(args []string) error {
	fs := flag.NewFlagSet("explore", flag.ContinueOnError)
	lf := addLiveFlags(fs)
	goalFlag := fs.String("goal", "", "What to achieve, in plain words")
	specFlag := fs.String("spec", "", "JSON spec file: done_when, values, hint, avoid")
	var doneWhen, values, avoid stringList
	fs.Var(&doneWhen, "done-when", "Deterministic finish check, repeatable: view=NAME | url=SUBSTR | text=SUBSTR | component=NAME | history=SUBSTR | history_count=N:SUBSTR | prop=Comp.name~value[@Within.name~value]")
	fs.Var(&values, "value", "A value the loop may type, as key=text (repeatable). The loop never invents text.")
	fs.Var(&avoid, "avoid", "Drop controls whose name contains this (repeatable), e.g. Delete, Pay")
	pickerFlag := fs.String("picker", "jev", "jev[:model] (TYPESAFE_API_KEY) or anthropic[:model] (ANTHROPIC_API_KEY)")
	planFlag := fs.Bool("plan", false, "Ask Anthropic once to write the spec from the goal")
	growFlag := fs.Bool("grow", false, "Grow the corpus while exploring: name unmapped controls on every page visited")
	noMapFlag := fs.Bool("no-map", false, "Observe with no map: no components, views, or memory. Same session, same loop.")
	maxStepsFlag := fs.Int("max-steps", 20, "Stop after this many steps")
	jsonFlag := fs.Bool("json", false, "Print the run as JSON on stdout")
	recordFlag := fs.String("record", "", "Capture the tab as JPEG frames into this directory while the goal runs (see scripts/render-demo.py)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *goalFlag == "" && len(pos) > 0 {
		*goalFlag = strings.Join(pos, " ")
	}
	if *goalFlag == "" {
		return fmt.Errorf("explore needs --goal (or the goal as positional words)")
	}
	if *noMapFlag && *growFlag {
		return fmt.Errorf("--no-map and --grow do not combine")
	}

	ctx := context.Background()
	conn, err := lf.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	corpus, err := lf.corpus()
	if err != nil {
		return err
	}
	if *noMapFlag {
		corpus = nil
	} else if corpus == nil {
		fmt.Fprintf(os.Stderr, "explore: no corpus at %q, driving the raw tree (add --grow to build one as you go)\n", *lf.dir)
	}
	hasMap := corpus != nil && len(corpus.AllComponents()) > 0
	drv := explore.NewCDPDriver(conn, corpus)

	hook, report, err := maybeGrow(*growFlag, *lf.dir, drv)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { fmt.Fprintln(os.Stderr, report()) }()
	}

	spec, err := buildSpec(ctx, *specFlag, doneWhen, values, avoid, *planFlag, *goalFlag, *lf.url)
	if err != nil {
		return err
	}
	picker, err := makePicker(*pickerFlag)
	if err != nil {
		return err
	}
	var rec *recorder
	if *recordFlag != "" {
		rec, err = startRecorder(ctx, conn, *recordFlag)
		if err != nil {
			return err
		}
	}
	run, err := explore.Explore(ctx, drv, explore.Options{
		Goal: *goalFlag, Spec: spec, Picker: picker, MaxSteps: *maxStepsFlag, HasMap: hasMap, Hook: hook,
		OnStep: func(s explore.Step) {
			line := explore.FormatStep(s)
			fmt.Fprintln(os.Stderr, line)
			if rec != nil {
				rec.event(s)
			}
		},
	})
	if rec != nil {
		rec.stop(run)
	}
	if run != nil && *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		_ = enc.Encode(run)
	}
	if err != nil {
		return err
	}
	if !*jsonFlag {
		status := "FAIL"
		if run.OK {
			status = "OK"
		}
		st := run.Stats
		fmt.Printf("%s  %s  steps=%d  %.1fs  picker=%s calls=%d %dms tokens=%d+%d", status, run.Reason, len(run.Steps), float64(run.Ms)/1000, run.Picker, st.Calls, st.Ms, st.InputTokens, st.OutputTokens)
		if st.USD > 0 {
			fmt.Printf(" $%.3f", st.USD)
		}
		fmt.Println()
	}
	if !run.OK {
		return errNotDone
	}
	return nil
}

func buildSpec(ctx context.Context, specPath string, doneWhen, values, avoid []string, plan bool, goal, site string) (*explore.Spec, error) {
	spec := &explore.Spec{Values: map[string]string{}}
	if specPath != "" {
		s, err := explore.LoadSpec(specPath)
		if err != nil {
			return nil, err
		}
		spec = s
		if spec.Values == nil {
			spec.Values = map[string]string{}
		}
	} else if plan {
		client, err := explore.NewAnthropicClient("")
		if err != nil {
			return nil, fmt.Errorf("--plan: %w", err)
		}
		host := ""
		if u, err := url.Parse(site); err == nil {
			host = u.Host
		}
		s, ms, err := client.Plan(ctx, goal, host)
		if err != nil {
			return nil, fmt.Errorf("--plan: %w", err)
		}
		b, _ := json.Marshal(s)
		fmt.Fprintf(os.Stderr, "plan (%d ms): %s\n", ms, b)
		spec = s
	}
	if len(doneWhen) > 0 {
		d, err := explore.ParseDoneWhen(doneWhen)
		if err != nil {
			return nil, err
		}
		spec.DoneWhen = d
	}
	for _, kv := range values {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("--value %q: expected key=text", kv)
		}
		spec.Values[k] = v
	}
	spec.Avoid = append(spec.Avoid, avoid...)
	return spec, nil
}

func makePicker(name string) (explore.Picker, error) {
	kind, model, _ := strings.Cut(name, ":")
	switch kind {
	case "jev", "":
		return explore.NewJevPicker(model)
	case "anthropic", "claude":
		return explore.NewAnthropicClient(model)
	}
	return nil, fmt.Errorf("--picker %q: use jev[:model] or anthropic[:model]", name)
}

func maybeGrow(on bool, dir string, drv *explore.CDPDriver) (explore.PageHook, func() string, error) {
	if !on {
		return nil, nil, nil
	}
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("--grow: create %s: %w", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "components.yaml"), []byte("version: 1\ncomponents: []\n"), 0o644); err != nil {
			return nil, nil, err
		}
	}
	jev, err := explore.NewJevPicker("")
	if err != nil {
		return nil, nil, fmt.Errorf("--grow: %w", err)
	}
	g := grow.New(dir, jev)
	g.Reload = func() error {
		c, err := sightmap.Load(dir)
		if err != nil {
			return err
		}
		if errs := sightmap.Validate(c); len(errs) > 0 {
			return fmt.Errorf("%d validation errors (first: %v)", len(errs), errs[0])
		}
		drv.Corpus = c
		return nil
	}
	g.Log = func(msg string) { fmt.Fprintln(os.Stderr, msg) }
	report := func() string {
		st := g.Stats()
		return fmt.Sprintf("grow: %d components, %d views, %d promoted to global, %d pages, %d model calls, %d ms", st.Added, st.Views, st.Promoted, st.Pages, st.Calls, st.Ms)
	}
	return g, report, nil
}

/* ---------------- bench ---------------- */

func runBench(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	lf := addLiveFlags(fs)
	suiteFlag := fs.String("suite", "", "Suite JSON file (or pass it as the positional)")
	pickerFlag := fs.String("picker", "jev", "jev[:model] or anthropic[:model]")
	growFlag := fs.Bool("grow", false, "Grow the corpus while exploring")
	noMapFlag := fs.Bool("no-map", false, "Observe with no map: no components, views, or memory. Same session, same loop.")
	repeatFlag := fs.Int("repeat", 1, "Run the suite this many times")
	onlyFlag := fs.String("only", "", "Only goals whose name contains this")
	maxStepsFlag := fs.Int("max-steps", 0, "Override every goal's max_steps")
	outFlag := fs.String("out", "", "Write the full result JSON here (default: explore-<suite>-<condition>-<picker>-<time>.json)")
	recordFlag := fs.String("record", "", "Capture the tab as JPEG frames into this directory while the suite runs (see scripts/render-demo.py)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *suiteFlag == "" && len(pos) > 0 {
		*suiteFlag = pos[0]
	}
	if *suiteFlag == "" {
		return fmt.Errorf("bench needs a suite file")
	}
	if *noMapFlag && *growFlag {
		return fmt.Errorf("--no-map and --grow do not combine")
	}
	suite, err := explore.LoadSuite(*suiteFlag)
	if err != nil {
		return err
	}
	explicitDir := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "sightmap-dir" {
			explicitDir = true
		}
	})
	if !explicitDir && suite.SightmapDir != "" {
		*lf.dir = filepath.Join(filepath.Dir(*suiteFlag), suite.SightmapDir)
	}
	if *lf.url == "" && *lf.start {
		*lf.url = suite.StartURL
	}

	ctx := context.Background()
	conn, err := lf.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	corpus, err := lf.corpus()
	if err != nil {
		return err
	}
	if *noMapFlag {
		corpus = nil
	}
	hasMap := corpus != nil && len(corpus.AllComponents()) > 0
	drv := explore.NewCDPDriver(conn, corpus)
	hook, report, err := maybeGrow(*growFlag, *lf.dir, drv)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { fmt.Fprintln(os.Stderr, report()) }()
	}
	var rec *recorder
	sopts := explore.SuiteOptions{
		NewPicker: func() (explore.Picker, error) { return makePicker(*pickerFlag) },
		Repeat:    *repeatFlag, Only: *onlyFlag, MaxSteps: *maxStepsFlag, HasMap: hasMap, Hook: hook, Out: os.Stderr,
	}
	if *recordFlag != "" {
		// Recording starts when the first goal starts, after its reset, so the video opens on the start page.
		sopts.OnGoal = func(g explore.Goal) {
			if rec == nil {
				rec, err = startRecorder(ctx, conn, *recordFlag)
				if err != nil {
					fmt.Fprintf(os.Stderr, "record: %v\n", err)
				}
			}
		}
		sopts.OnStep = func(s explore.Step) {
			if rec != nil {
				rec.event(s)
			}
		}
	}
	res, err := explore.RunSuite(ctx, drv, suite, sopts)
	if rec != nil {
		var last *explore.Run
		if len(res.Runs) > 0 {
			last = res.Runs[len(res.Runs)-1].Run
		}
		rec.stop(last)
	}
	if err != nil {
		return err
	}
	fmt.Print("\n" + explore.FormatTable(res))
	fmt.Print("\n" + explore.FormatScores([]explore.Score{explore.ScoreResult(res)}))
	out := *outFlag
	if out == "" {
		out = fmt.Sprintf("explore-%s-%s-%s-%s.json", suite.Name, res.Condition, strings.NewReplacer(":", "_", "/", "_").Replace(res.Picker), time.Now().Format("20060102-150405"))
	}
	data, _ := json.MarshalIndent(res, "", " ")
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved %s\n", out)
	if res.Summary.OK < res.Summary.Goals {
		return fmt.Errorf("%d of %d goals not reached", res.Summary.Goals-res.Summary.OK, res.Summary.Goals)
	}
	return nil
}

/* ---------------- score ---------------- */

func runScore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("score needs one or more run files")
	}
	var scores []explore.Score
	for _, path := range args {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("score %s: %w", path, err)
		}
		var res explore.SuiteResult
		if err := json.Unmarshal(data, &res); err != nil {
			return fmt.Errorf("score %s: not a SuiteResult: %w", path, err)
		}
		if res.Suite == "" || len(res.Runs) == 0 {
			return fmt.Errorf("score %s: not a SuiteResult (no suite name or runs)", path)
		}
		scores = append(scores, explore.ScoreResult(&res))
	}
	fmt.Print(explore.FormatScores(scores))
	return nil
}

/* ---------------- plan ---------------- */

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	goalFlag := fs.String("goal", "", "The goal to plan for")
	siteFlag := fs.String("site", "", "Host name, for context")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *goalFlag == "" {
		*goalFlag = strings.Join(pos, " ")
	}
	if *goalFlag == "" {
		return fmt.Errorf("plan needs --goal")
	}
	client, err := explore.NewAnthropicClient("")
	if err != nil {
		return err
	}
	spec, ms, err := client.Plan(context.Background(), *goalFlag, *siteFlag)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(spec)
	fmt.Fprintf(os.Stderr, "%d ms, %+v\n", ms, client.Stats())
	return nil
}

/* ---------------- graph ---------------- */

func runGraph(args []string) error {
	files := args
	if len(files) == 0 {
		matches, _ := filepath.Glob("explore-*.json")
		files = matches
	}
	edges := map[string]int{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var suite explore.SuiteResult
		var runs []*explore.Run
		if err := json.Unmarshal(data, &suite); err == nil && len(suite.Runs) > 0 {
			for _, r := range suite.Runs {
				if r.Run != nil {
					runs = append(runs, r.Run)
				}
			}
		} else {
			var one explore.Run
			if err := json.Unmarshal(data, &one); err == nil {
				runs = append(runs, &one)
			}
		}
		for _, r := range runs {
			for _, t := range r.Transitions {
				label := t.Comp
				if label == "" {
					label = t.Action
				}
				edges[fmt.Sprintf("%s --%s--> %s", t.From, label, t.To)]++
			}
		}
	}
	type kv struct {
		k string
		n int
	}
	var all []kv
	for k, n := range edges {
		all = append(all, kv{k, n})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].k < all[j].k
	})
	w := io.Writer(os.Stdout)
	for _, e := range all {
		fmt.Fprintf(w, "%4d  %s\n", e.n, e.k)
	}
	return nil
}
