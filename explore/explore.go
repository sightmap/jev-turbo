package explore

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Options configures one goal.
type Options struct {
	Goal          string
	Spec          *Spec
	Picker        Picker
	MaxSteps      int     // default 20
	MaxCandidates int     // default 60
	DoneThreshold float64 // picker "done" confidence that ends a goal with no deterministic check (default 0.85)
	HasMap        bool    // the corpus has at least one component; enables Step.Fallback
	// Tools and ToolRunner, when both set, offer sightkick tools as picker
	// options alongside elements; a tool call runs through ToolRunner instead
	// of driving an element directly.
	Tools      *ToolSet
	ToolRunner ToolRunner
	// Hook runs on every observed page before candidates are built (used by --grow).
	Hook PageHook
	// OnStep is called after each step with its record.
	OnStep func(Step)
}

// PageHook sees every page the loop observes.
type PageHook interface {
	OnPage(ctx context.Context, page *Page) error
}

// Step records one iteration.
type Step struct {
	N         int      `json:"step"`
	URL       string   `json:"url"`
	View      string   `json:"view,omitempty"`
	Coverage  *CovStat `json:"coverage,omitempty"`
	Pick      string   `json:"pick,omitempty"`
	Group     string   `json:"group,omitempty"`
	Probs     string   `json:"probs,omitempty"`
	DoneProb  float64  `json:"done_prob"`
	Why       string   `json:"why,omitempty"`
	Action    string   `json:"action"`
	URLAfter  string   `json:"url_after,omitempty"`
	MsSnap    int      `json:"ms_snap"`
	MsPick    int      `json:"ms_pick"`
	MsAct     int      `json:"ms_act"`
	MsSettle  int      `json:"ms_settle"`
	Ms        int      `json:"ms"`
	Navigated bool     `json:"navigated,omitempty"`

	Candidates      int     `json:"candidates,omitempty"`       // element candidates after the guards
	Options         int     `json:"options,omitempty"`          // options on the first pick; a group counts once
	Named           int     `json:"named,omitempty"`            // candidates that carry a sightmap component
	Confidence      float64 `json:"confidence,omitempty"`       // probability of the chosen option
	GroupConfidence float64 `json:"group_confidence,omitempty"` // probability of the chosen group on a grouped pick
	Fallback        bool    `json:"fallback,omitempty"`         // a map exists but the pick is an unnamed node
	Wasted          bool    `json:"wasted,omitempty"`           // stale, back, a control or tool repeated at this URL, or a tool call that failed and did not finish the goal

	Tool   string `json:"tool,omitempty"`    // the sightkick tool run, when the pick was a "t:" option
	ToolOK bool   `json:"tool_ok,omitempty"` // the tool call reported ok
}

// CovStat is the page's coverage at the moment of a step.
type CovStat struct {
	Interactive int `json:"interactive"`
	T1          int `json:"t1"`
	T2          int `json:"t2"`
	Orphaned    int `json:"t3"`
}

// Transition is one observed edge of the site graph.
type Transition struct {
	From    string `json:"from"`
	Action  string `json:"action"`
	Comp    string `json:"comp,omitempty"`
	To      string `json:"to"`
	Changed bool   `json:"changed"`
}

// Run is the result of one goal.
type Run struct {
	Goal        string       `json:"goal"`
	Spec        *Spec        `json:"spec,omitempty"`
	Picker      string       `json:"picker"`
	OK          bool         `json:"ok"`
	Reason      string       `json:"reason"`
	Steps       []Step       `json:"steps"`
	Transitions []Transition `json:"transitions"`
	Ms          int          `json:"ms"`
	Stats       Stats        `json:"picker_stats"`
	Metrics     RunMetrics   `json:"metrics"`
	HookErrors  int          `json:"hook_errors,omitempty"`
}

// Explore drives the browser toward opts.Goal and returns the run. It returns
// an error only for failures outside the loop's control (a picker outage, a
// dead connection); a goal that is not reached is a Run with OK=false.
func Explore(ctx context.Context, drv Driver, opts Options) (*Run, error) {
	if opts.Picker == nil {
		return nil, fmt.Errorf("explore: no picker")
	}
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 20
	}
	if opts.DoneThreshold <= 0 {
		opts.DoneThreshold = 0.85
	}
	spec := opts.Spec
	if spec == nil {
		spec = &Spec{}
	}
	values := spec.Values
	usedValues := map[string]bool{}
	seen := map[string]int{}
	var history []string
	suggestionsOpen := false
	var filledNode *Node // the field the last action typed into; Enter is pressed in it
	navigations := 0
	afterFill := false
	run := &Run{Goal: opts.Goal, Spec: spec, Picker: opts.Picker.Name()}
	var suggested []string // tool names to rank first, from the last tool call's Guidance; the next ToolOptions call consumes it and it is cleared there
	skipTool := ""         // a tool that just failed: left out of the next pick only, while its guidance still counts
	failedToolAt := -1     // index in run.Steps of the step just appended, when it was a failed tool call
	t0 := time.Now()
	defer func() {
		run.Ms = int(time.Since(t0).Milliseconds())
		run.Stats = opts.Picker.Stats()
		run.Metrics = Metrics(run.Steps)
	}()

	for n := 1; n <= opts.MaxSteps; n++ {
		if ctx.Err() != nil {
			run.Reason = "cancelled"
			return run, nil
		}
		tS := time.Now()
		page, err := drv.Observe(ctx)
		if err != nil {
			return run, fmt.Errorf("explore: observe: %w", err)
		}
		if len(page.Nodes) < 3 {
			drv.Settle(ctx, page.URL)
			if page, err = drv.Observe(ctx); err != nil {
				return run, fmt.Errorf("explore: observe: %w", err)
			}
		}
		step := Step{N: n, URL: page.URL, View: page.View, Coverage: covStat(page), MsSnap: int(time.Since(tS).Milliseconds())}
		if len(run.Transitions) > 0 {
			run.Transitions[len(run.Transitions)-1].To = pageLabel(page)
		}
		if opts.Hook != nil {
			if err := opts.Hook.OnPage(ctx, page); err != nil {
				run.HookErrors++
			}
		}

		if spec.DoneWhen.Deterministic() && spec.DoneWhen.Check(page, history) {
			if failedToolAt >= 0 {
				// The call reported a failure, but the page it left behind is
				// the one the goal asks for, so the step did the work.
				run.Steps[failedToolAt].Wasted = false
			}
			step.Action = "done"
			run.Steps = append(run.Steps, step)
			run.OK = true
			run.Reason = "done_when satisfied"
			emit(opts, step)
			return run, nil
		}

		cands := Candidates(page.Nodes, CandidateOptions{Seen: seen, URL: page.URL, Avoid: spec.Avoid})
		if suggestionsOpen {
			// The last action typed into a suggestion field and its list is showing:
			// the typed value only counts once an option is chosen, so offer only those.
			if opts := onlyOptions(cands); len(opts) > 0 {
				cands = opts
			} else {
				// The options can still be rendering when this observation landed
				// (a few hundred ms behind in headless Chrome): look once more
				// before giving up and offering the whole page.
				drv.Wait(ctx, 300*time.Millisecond)
				if page, err = drv.Observe(ctx); err != nil {
					return run, fmt.Errorf("explore: observe: %w", err)
				}
				step.URL = page.URL
				step.View = page.View
				step.Coverage = covStat(page)
				step.MsSnap = int(time.Since(tS).Milliseconds())
				cands = Candidates(page.Nodes, CandidateOptions{Seen: seen, URL: page.URL, Avoid: spec.Avoid})
				if opts := onlyOptions(cands); len(opts) > 0 {
					cands = opts
				}
			}
			suggestionsOpen = false
		}
		if len(cands) == 0 && len(Candidates(page.Nodes, CandidateOptions{Avoid: spec.Avoid})) > 0 {
			// Every control here has already been tried twice; forget this page's history once and try again.
			for k := range seen {
				if strings.HasPrefix(k, page.URL+"|") {
					delete(seen, k)
				}
			}
			cands = Candidates(page.Nodes, CandidateOptions{Seen: seen, URL: page.URL, Avoid: spec.Avoid})
		}
		if len(cands) == 0 {
			step.Action = "no-candidates"
			run.Steps = append(run.Steps, step)
			run.Reason = "no actionable elements"
			emit(opts, step)
			return run, nil
		}
		crit := BuildCriteria(cands, CriteriaOptions{MaxCandidates: opts.MaxCandidates, Goal: opts.Goal, Seen: seen, URL: page.URL, AfterFill: afterFill, CanGoBack: navigations > 0})
		var toolOpts []Criterion
		if opts.Tools != nil && opts.ToolRunner != nil {
			toolOpts = dropTool(ToolOptions(opts.Tools, page.View, values, suggested), skipTool)
			suggested = suggested[:0]
			crit.Options = append(append([]Criterion{}, toolOpts...), crit.Options...)
		}
		skipTool = ""
		step.Candidates = len(cands)
		step.Options = len(crit.Options)
		step.Named = countNamed(cands)
		state := buildState(opts.Goal, spec, page, history, cands, toolOpts)

		tP := time.Now()
		pick, err := opts.Picker.Pick(ctx, state, crit)
		if err != nil {
			return run, fmt.Errorf("explore: pick: %w", err)
		}
		if strings.HasPrefix(pick.Next, "g:") && crit.Groups != nil {
			members := crit.Groups[pick.Next]
			gp := pick.Probs[pick.Next]
			second, err := opts.Picker.Pick(ctx, state, GroupCriteria(members))
			if err != nil {
				return run, fmt.Errorf("explore: pick in group: %w", err)
			}
			step.Group = pick.Next
			step.GroupConfidence = gp
			if second.Done < pick.Done {
				second.Done = pick.Done
			}
			pick = second
		}
		step.MsPick = int(time.Since(tP).Milliseconds())
		step.Pick = pick.Next
		step.Probs = topProbs(pick.Probs, 3)
		step.DoneProb = pick.Done
		step.Why = pick.Why
		step.Confidence = pick.Probs[pick.Next]
		if step.Group != "" {
			// A grouped pick is two answers: the option's own probability is
			// conditional on the group, so the step's confidence is the joint one.
			step.Confidence *= step.GroupConfidence
		}
		if c := findCandidate(cands, pick.Next); c != nil {
			step.Fallback = opts.HasMap && c.Node.Comp == ""
			step.Wasted = seen[page.URL+"|"+c.SeenKey] > 0
		}

		if pick.Done >= opts.DoneThreshold && !spec.DoneWhen.Deterministic() {
			step.Action = "done(judged)"
			run.Steps = append(run.Steps, step)
			run.OK = true
			run.Reason = fmt.Sprintf("picker judged done (%.2f)", pick.Done)
			emit(opts, step)
			return run, nil
		}

		tA := time.Now()
		var doneFn func(*Page) bool
		if spec.DoneWhen.Deterministic() {
			doneFn = func(p *Page) bool { return spec.DoneWhen.Check(p, history) }
		}
		var act *action
		if strings.HasPrefix(pick.Next, ToolPrefix) {
			name := strings.TrimPrefix(pick.Next, ToolPrefix)
			tool := opts.Tools.Get(name)
			if tool == nil {
				return run, fmt.Errorf("explore: act: picked unknown tool %q", name)
			}
			args, _ := ToolArgs(tool, values)
			var res ToolResult
			act, res, err = performTool(ctx, drv, opts.ToolRunner, tool, args, page)
			step.Tool = tool.Name
			step.ToolOK = err == nil && res.OK
			for _, g := range res.Guidance {
				suggested = append(suggested, g.Tool)
			}
			if err == nil {
				// A tool run twice at one URL repeats work, the same as a
				// control clicked twice there.
				step.Wasted = step.Wasted || seen[page.URL+"|"+act.seenKey] > 0
				if !res.OK {
					step.Wasted = true
					skipTool = tool.Name
				}
			}
		} else {
			act, err = perform(ctx, drv, opts.Picker, pick.Next, cands, values, usedValues, state, page, doneFn, filledNode)
		}
		if step.Tool == "" && err != nil && isStale(err) {
			// The page re-rendered between snapshot and act: re-observe and retry the same element by description.
			fresh, oErr := drv.Observe(ctx)
			if oErr != nil {
				return run, fmt.Errorf("explore: observe: %w", oErr)
			}
			want := findCandidate(cands, pick.Next)
			var again *Node
			if want != nil {
				for _, fn := range fresh.Nodes {
					if fn.Interactive && fn.Visible && Describe(fn) == want.Desc {
						again = fn
						break
					}
				}
			}
			if again != nil {
				retry := []*Candidate{{Key: "n" + again.ID, Node: again, Desc: want.Desc, SeenKey: want.SeenKey}}
				act, err = perform(ctx, drv, opts.Picker, retry[0].Key, retry, values, usedValues, state, fresh, doneFn, filledNode)
			}
			if again == nil || (err != nil && isStale(err)) {
				// Gone twice: record the miss as a step and let the next observation decide.
				seenKey := pick.Next
				if want != nil {
					seenKey = want.SeenKey
				}
				act = &action{summary: fmt.Sprintf("stale element, skipped (%s)", pickLabel(want, pick.Next)), stale: true, seenKey: seenKey, urlAfter: fresh.URL}
				err = nil
			}
		}
		if err != nil {
			return run, fmt.Errorf("explore: act: %w", err)
		}
		step.MsAct = int(time.Since(tA).Milliseconds()) - act.settleMs
		if step.MsAct < 0 {
			step.MsAct = 0
		}
		step.MsSettle = act.settleMs
		step.Action = act.summary
		if pick.Next == MetaBack || act.stale {
			step.Wasted = true
		}
		step.URLAfter = act.urlAfter
		step.Navigated = act.urlAfter != page.URL
		if step.Navigated {
			navigations++
		}
		step.Ms = int(time.Since(tS).Milliseconds())
		run.Steps = append(run.Steps, step)
		failedToolAt = -1
		if step.Tool != "" && !step.ToolOK {
			failedToolAt = len(run.Steps) - 1
		}
		run.Transitions = append(run.Transitions, Transition{From: pageLabel(page), Action: act.summary, Comp: act.comp, To: shortURL(act.urlAfter), Changed: step.Navigated})
		history = append(history, fmt.Sprintf("%d. %s → %s", n, act.summary, shortURL(act.urlAfter)))
		seen[page.URL+"|"+act.seenKey]++
		suggestionsOpen = act.combobox // decided on the next observation: options in the tree, whatever their DOM shape
		afterFill = act.filled
		if act.filled {
			filledNode = act.node
		} else if pick.Next != MetaEnter && pick.Next != MetaWait {
			filledNode = nil // any other action moves on from the field
		}
		emit(opts, step)
	}
	run.Reason = fmt.Sprintf("no result within %d steps", opts.MaxSteps)
	return run, nil
}

func emit(opts Options, s Step) {
	if opts.OnStep != nil {
		opts.OnStep(s)
	}
}

type action struct {
	summary      string
	comp         string
	seenKey      string
	urlAfter     string
	settleMs     int
	stale        bool   // the element was gone twice: nothing was acted on
	combobox     bool   // typed into, or opened, a control with a list; wait for its options before observing
	optionsShown bool   // the list was visible after the wait
	filled       bool   // typed into a field; Enter is offered next
	typed        string // the value typed, for matching the suggestions
	node         *Node  // the element acted on, when the action had one
}

// onlyOptions keeps the suggestion entries (role option) of a candidate list.
func onlyOptions(cands []*Candidate) []*Candidate {
	var out []*Candidate
	for _, c := range cands {
		if c.Node.Role == "option" {
			out = append(out, c)
		}
	}
	return out
}

// perform runs one pick. enterIn is the field the last fill typed into, so
// the Enter meta action is pressed there and not wherever focus drifted to.
func perform(ctx context.Context, drv Driver, picker Picker, pick string, cands []*Candidate, values map[string]string, usedValues map[string]bool, state string, page *Page, done func(*Page) bool, enterIn *Node) (*action, error) {
	act := &action{seenKey: pick}
	switch pick {
	case MetaBack:
		if err := drv.Back(ctx); err != nil {
			return nil, err
		}
		act.summary = "went back"
	case MetaScroll:
		if err := drv.Scroll(ctx); err != nil {
			return nil, err
		}
		act.summary = "scrolled down"
	case MetaWait:
		if done != nil {
			// Poll the finish check while waiting so a page that completes early ends the wait.
			deadline := time.Now().Add(1200 * time.Millisecond)
			for time.Now().Before(deadline) {
				drv.Wait(ctx, 150*time.Millisecond)
				if p, err := drv.Observe(ctx); err == nil && done(p) {
					break
				}
			}
		} else {
			drv.Wait(ctx, 700*time.Millisecond)
		}
		act.summary = "waited"
	case MetaEnter:
		if err := drv.PressEnter(ctx, enterIn); err != nil {
			return nil, err
		}
		act.summary = "pressed Enter"
	default:
		c := findCandidate(cands, pick)
		if c == nil {
			return nil, fmt.Errorf("picked unknown key %q", pick)
		}
		n := c.Node
		act.comp = n.Comp
		act.seenKey = c.SeenKey
		act.node = n
		label := CompLabel(n)
		if label == "" {
			label = fmt.Sprintf("%s %q", n.Role, trunc(n.Name, 40))
		}
		switch {
		case IsTextInput(n):
			key, val, ok, err := chooseValue(ctx, picker, n, values, usedValues, state)
			if err != nil {
				return nil, err
			}
			if ok {
				if err := drv.Fill(ctx, n, val); err != nil {
					return nil, err
				}
				usedValues[key] = true
				act.combobox = IsCombobox(n)
				act.typed = val
				act.filled = true
				act.summary = fmt.Sprintf("filled %s with %s", label, key)
			} else {
				if err := drv.Click(ctx, n); err != nil {
					return nil, err
				}
				act.summary = fmt.Sprintf("focused %s (no value to type)", label)
			}
		case IsSelect(n):
			opts, err := drv.SelectOptions(ctx, n)
			if err != nil {
				return nil, err
			}
			idx, err := chooseOption(ctx, picker, n, opts, state)
			if err != nil {
				return nil, err
			}
			if err := drv.Select(ctx, n, idx); err != nil {
				return nil, err
			}
			chosen := ""
			if idx >= 0 && idx < len(opts) {
				chosen = opts[idx]
			}
			act.summary = fmt.Sprintf("selected %q in %s", chosen, label)
		case IsCheckable(n):
			if err := drv.Click(ctx, n); err != nil {
				return nil, err
			}
			act.summary = "toggled " + label
		default:
			if err := drv.Click(ctx, n); err != nil {
				return nil, err
			}
			act.summary = "clicked " + label
			act.combobox = OpensList(n)
		}
	}
	info := drv.Settle(ctx, page.URL)
	act.settleMs = info.Ms
	act.urlAfter = info.URL
	if act.combobox {
		t := time.Now()
		act.optionsShown = drv.WaitForOptions(ctx, 1500*time.Millisecond, act.typed)
		act.settleMs += int(time.Since(t).Milliseconds())
	}
	if act.urlAfter == "" {
		if u, err := drv.URL(ctx); err == nil {
			act.urlAfter = u
		} else {
			act.urlAfter = page.URL
		}
	}
	return act, nil
}

// performTool runs a sightkick tool and settles the page afterwards.
func performTool(ctx context.Context, drv Driver, runner ToolRunner, t *Tool, args map[string]string, page *Page) (*action, ToolResult, error) {
	res, err := runner.Run(ctx, t.Name, args)
	if err != nil {
		return nil, res, err
	}
	info := drv.Settle(ctx, page.URL)
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var kv []string
	for _, k := range keys {
		kv = append(kv, k+"="+args[k])
	}
	summary := fmt.Sprintf("ran tool %s(%s)", t.Name, strings.Join(kv, ", "))
	switch {
	case !res.OK:
		summary = fmt.Sprintf("tool %s failed: %s", t.Name, res.Message)
	case res.Skipped:
		summary += " (already applied)"
	}
	urlAfter := info.URL
	if urlAfter == "" {
		if u, err := drv.URL(ctx); err == nil {
			urlAfter = u
		} else {
			urlAfter = page.URL
		}
	}
	return &action{summary: summary, comp: "tool:" + t.Name, seenKey: "tool " + t.Name, urlAfter: urlAfter, settleMs: info.Ms}, res, nil
}

// dropTool leaves one tool out of a pick's options. A tool that just failed is
// worth skipping once, but the rest of the layer, and the guidance the failed
// call returned, are still worth offering.
func dropTool(opts []Criterion, name string) []Criterion {
	if name == "" {
		return opts
	}
	var out []Criterion
	for _, o := range opts {
		if strings.TrimPrefix(o.Key, ToolPrefix) != name {
			out = append(out, o)
		}
	}
	return out
}

func isStale(err error) bool {
	s := err.Error()
	return strings.Contains(s, "not found in live DOM") || strings.Contains(s, "not found") || strings.Contains(s, "noel")
}

func findCandidate(cands []*Candidate, key string) *Candidate {
	for _, c := range cands {
		if c.Key == key {
			return c
		}
	}
	return nil
}

func pickLabel(c *Candidate, key string) string {
	if c != nil {
		return c.Desc
	}
	return key
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// chooseValue picks which spec value to type into a field: by name overlap
// between the field and the value keys when that is unambiguous, otherwise by
// asking the picker. It returns ok=false when nothing fits.
func chooseValue(ctx context.Context, picker Picker, n *Node, values map[string]string, used map[string]bool, state string) (key, val string, ok bool, err error) {
	if len(values) == 0 {
		return "", "", false, nil
	}
	fieldWords := strings.ToLower(strings.Join([]string{n.Name, n.Attrs["placeholder"], n.Attrs["name"], n.Attrs["id"], n.Attrs["aria-label"], n.Attrs["type"], n.Comp}, " "))
	fieldWords = " " + nonAlnum.ReplaceAllString(fieldWords, " ") + " "
	type scored struct {
		key  string
		hits int
	}
	var best []scored
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		hits := 0
		for _, w := range strings.Fields(nonAlnum.ReplaceAllString(strings.ToLower(k), " ")) {
			if strings.Contains(fieldWords, w) {
				hits++
			}
		}
		if hits > 0 {
			best = append(best, scored{k, hits})
		}
	}
	if len(best) > 0 {
		sort.SliceStable(best, func(i, j int) bool {
			if best[i].hits != best[j].hits {
				return best[i].hits > best[j].hits
			}
			return !used[best[i].key] && used[best[j].key]
		})
		ties := 0
		for _, b := range best {
			if b.hits == best[0].hits {
				ties++
			}
		}
		if ties == 1 {
			return best[0].key, values[best[0].key], true, nil
		}
	}
	if len(keys) == 1 {
		return keys[0], values[keys[0]], true, nil
	}
	var crit Criteria
	for _, k := range keys {
		d := fmt.Sprintf("%s = %q", k, values[k])
		if used[k] {
			d += " (already typed once)"
		}
		crit.Options = append(crit.Options, Criterion{k, d})
	}
	crit.Options = append(crit.Options, Criterion{"none", "none of these belongs in this field"})
	res, err := picker.Choose(ctx, state+"\n\nFIELD TO FILL: "+Describe(n), crit, "Which value should be typed into this field?")
	if err != nil {
		return "", "", false, err
	}
	if res == "none" {
		return "", "", false, nil
	}
	if v, ok := values[res]; ok {
		return res, v, true, nil
	}
	return "", "", false, nil
}

func chooseOption(ctx context.Context, picker Picker, n *Node, opts []string, state string) (int, error) {
	if len(opts) <= 1 {
		return 0, nil
	}
	var crit Criteria
	for i, o := range opts {
		crit.Options = append(crit.Options, Criterion{fmt.Sprintf("o%d", i), o})
	}
	res, err := picker.Choose(ctx, state+"\n\nSELECT FIELD: "+Describe(n), crit, "Which option should be selected to move toward the goal?")
	if err != nil {
		return 0, err
	}
	var idx int
	if _, err := fmt.Sscanf(res, "o%d", &idx); err != nil || idx < 0 || idx >= len(opts) {
		return 0, nil
	}
	return idx, nil
}

// buildState renders the picker's context: goal, spec, page, recent steps,
// the sightkick tools callable here (if any), and every actionable element
// with its key.
func buildState(goal string, spec *Spec, page *Page, history []string, cands []*Candidate, tools []Criterion) string {
	var b strings.Builder
	fmt.Fprintf(&b, "GOAL: %s\n", goal)
	if spec.Hint != "" {
		fmt.Fprintf(&b, "HINT: %s\n", spec.Hint)
	}
	fmt.Fprintf(&b, "DONE WHEN: %s\n", spec.DoneWhen.String())
	if len(spec.Values) > 0 {
		keys := make([]string, 0, len(spec.Values))
		for k := range spec.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var vals []string
		for _, k := range keys {
			vals = append(vals, fmt.Sprintf("%s=%q", k, spec.Values[k]))
		}
		fmt.Fprintf(&b, "VALUES YOU MAY TYPE: %s\n", strings.Join(vals, ", "))
	}
	view := page.View
	if view == "" {
		view = "unknown"
	}
	fmt.Fprintf(&b, "CURRENT PAGE: view=%s url=%s\n", view, shortURL(page.URL))
	if names := componentNames(page); len(names) > 0 {
		fmt.Fprintf(&b, "COMPONENTS ON PAGE: %s\n", strings.Join(names, ", "))
	}
	if len(page.Notes) > 0 {
		b.WriteString("SITE NOTES:\n")
		for i, n := range page.Notes {
			if i >= 12 {
				break
			}
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}
	b.WriteString("RECENT STEPS:\n")
	if len(history) == 0 {
		b.WriteString("  (none yet)\n")
	}
	start := 0
	if len(history) > 8 {
		start = len(history) - 8
	}
	for _, h := range history[start:] {
		fmt.Fprintf(&b, "  %s\n", h)
	}
	for _, c := range cands {
		if c.Node.Role == "option" {
			b.WriteString("A SUGGESTION LIST IS OPEN: a value typed into a field only counts once one of its option entries is chosen.\n")
			break
		}
	}
	if len(tools) > 0 {
		b.WriteString("TOOLS (run a whole named action; prefer one when it fits):\n")
		for _, t := range tools {
			fmt.Fprintf(&b, "  %s: %s\n", t.Key, t.Desc)
		}
	}
	b.WriteString("ACTIONABLE ELEMENTS (key: description):\n")
	for i, c := range cands {
		if i >= 200 {
			break
		}
		fmt.Fprintf(&b, "  %s: %s\n", c.Key, c.Desc)
	}
	fmt.Fprintf(&b, "  back: %s\n", metaDesc[MetaBack])
	b.WriteString("NOTE: every element on the page is already listed above, including ones below the fold; scrolling adds nothing unless the page loads more on scroll.\n")
	return b.String()
}

func componentNames(page *Page) []string {
	set := map[string]bool{}
	for _, n := range page.Nodes {
		if n.Comp != "" {
			set[n.Comp] = true
		}
	}
	names := make([]string, 0, len(set))
	for k := range set {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func covStat(page *Page) *CovStat {
	if page.Result == nil {
		return nil
	}
	cov := page.Result.Coverage
	return &CovStat{Interactive: cov.Total, T1: cov.T1, T2: cov.T2, Orphaned: cov.T3}
}

func pageLabel(page *Page) string {
	if page.View != "" {
		return page.View
	}
	return shortURL(page.URL)
}

func shortURL(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return u
	}
	s := p.Path
	if p.RawQuery != "" {
		s += "?" + p.RawQuery
	}
	if s == "" {
		s = "/"
	}
	return s
}

// countNamed counts candidates that carry a sightmap component.
func countNamed(cands []*Candidate) int {
	n := 0
	for _, c := range cands {
		if c.Node != nil && c.Node.Comp != "" {
			n++
		}
	}
	return n
}
