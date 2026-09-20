package explore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Suite is a reproducible set of goals against one site.
type Suite struct {
	Name        string   `json:"name"`
	SightmapDir string   `json:"sightmap_dir,omitempty"`
	Tools       string   `json:"tools,omitempty"` // sightkick tool layer dir; resolved relative to the suite file, like SightmapDir
	StartURL    string   `json:"start_url"`
	Reset       string   `json:"reset,omitempty"` // "clear-storage" (cookies + storage, then navigate) or "navigate"
	Avoid       []string `json:"avoid,omitempty"`
	Goals       []Goal   `json:"goals"`
}

// Goal is one suite entry.
type Goal struct {
	Name     string `json:"name"`
	Goal     string `json:"goal"`
	Spec     *Spec  `json:"spec,omitempty"`
	StartURL string `json:"start_url,omitempty"`
	MaxSteps int    `json:"max_steps,omitempty"`
}

// LoadSuite reads a suite JSON file.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Suite
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("suite %s: %w", path, err)
	}
	if len(s.Goals) == 0 {
		return nil, fmt.Errorf("suite %s: no goals", path)
	}
	for _, g := range s.Goals {
		if g.Spec == nil {
			continue
		}
		if err := g.Spec.DoneWhen.validate(); err != nil {
			return nil, fmt.Errorf("suite %s: goal %q: %w", path, g.Name, err)
		}
	}
	return &s, nil
}

// SuiteOptions configures a suite run.
type SuiteOptions struct {
	NewPicker    func() (Picker, error) // a fresh picker per goal so stats are per goal
	Repeat       int
	Only         string // substring filter on goal names
	MaxSteps     int    // overrides per-goal max_steps when > 0
	HasMap       bool   // the corpus has at least one component; forwarded to every goal's Options
	NoMemory     bool   // withhold the map's memory notes from the picker; forwarded to every goal's Options
	JudgeEffects bool   // ask the picker what each action did; forwarded to every goal's Options
	SecondLook   bool   // open the likeliest groups after an unsure group pick; forwarded to every goal\'s Options
	// Tools and ToolRunner, when both set, offer a sightkick tool layer's
	// tools alongside elements in every goal (forwarded to each goal's
	// Options); RunSuite reports the condition as "tools" when Tools is set.
	Tools      *ToolSet
	ToolRunner ToolRunner
	Hook       PageHook
	Out        io.Writer  // step-by-step progress; nil for silent
	OnStep     func(Step) // called for every step of every goal, after it is printed
	OnGoal     func(Goal) // called when a goal is about to start (after the reset)
}

// GoalResult is one goal's run inside a suite.
type GoalResult struct {
	Name string `json:"name"`
	Rep  int    `json:"rep"`
	*Run
	Error string `json:"error,omitempty"`
}

// SuiteResult is the whole run.
type SuiteResult struct {
	Suite     string       `json:"suite"`
	Picker    string       `json:"picker"`
	Condition string       `json:"condition,omitempty"` // "map", "no-map", or "tools"; "-no-memory" is appended when the map's notes were withheld
	When      time.Time    `json:"when"`
	Summary   Summary      `json:"summary"`
	Runs      []GoalResult `json:"runs"`
}

// Summary aggregates a suite run.
type Summary struct {
	Goals        int     `json:"goals"`
	OK           int     `json:"ok"`
	Steps        int     `json:"steps"`
	Ms           int     `json:"ms"`
	MsPerStep    float64 `json:"ms_per_step"`
	ModelCalls   int     `json:"model_calls"`
	ModelMs      int     `json:"model_ms"`
	MsPerCall    float64 `json:"ms_per_call"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	USD          float64 `json:"usd"`

	Wasted           int     `json:"wasted"`
	Fallback         int     `json:"fallback"`
	LowConfidence    int     `json:"low_confidence"`
	CandidatesMedian float64 `json:"candidates_median"`
}

// RunSuite runs every goal (optionally repeated), resetting the browser between goals.
func RunSuite(ctx context.Context, drv Driver, suite *Suite, opts SuiteOptions) (*SuiteResult, error) {
	if opts.NewPicker == nil {
		return nil, fmt.Errorf("bench: no picker")
	}
	if opts.Repeat <= 0 {
		opts.Repeat = 1
	}
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	condition := "no-map"
	if opts.HasMap {
		condition = "map"
	}
	if opts.Tools != nil {
		condition = "tools"
	}
	if opts.NoMemory && condition != "no-map" {
		condition += "-no-memory"
	}
	result := &SuiteResult{Suite: suite.Name, Condition: condition, When: time.Now()}
	for rep := 1; rep <= opts.Repeat; rep++ {
		for _, g := range suite.Goals {
			if opts.Only != "" && !strings.Contains(g.Name, opts.Only) {
				continue
			}
			picker, err := opts.NewPicker()
			if err != nil {
				return nil, err
			}
			result.Picker = picker.Name()
			if err := resetBetweenGoals(ctx, drv, suite, g); err != nil {
				fmt.Fprintf(out, "reset: %v\n", err)
			}
			label := g.Name
			if opts.Repeat > 1 {
				label = fmt.Sprintf("%s (rep %d)", g.Name, rep)
			}
			fmt.Fprintf(out, "\n=== %s\n", label)
			if opts.OnGoal != nil {
				opts.OnGoal(g)
			}
			spec := &Spec{}
			if g.Spec != nil {
				cp := *g.Spec
				spec = &cp
			}
			spec.Avoid = append(append([]string{}, suite.Avoid...), spec.Avoid...)
			maxSteps := g.MaxSteps
			if opts.MaxSteps > 0 {
				maxSteps = opts.MaxSteps
			}
			var partial []Step
			t0 := time.Now()
			run, err := Explore(ctx, drv, Options{
				Goal: g.Goal, Spec: spec, Picker: picker, MaxSteps: maxSteps, HasMap: opts.HasMap, NoMemory: opts.NoMemory, JudgeEffects: opts.JudgeEffects, SecondLook: opts.SecondLook, Hook: opts.Hook,
				Tools: opts.Tools, ToolRunner: opts.ToolRunner,
				OnStep: func(s Step) {
					partial = append(partial, s)
					fmt.Fprintln(out, FormatStep(s))
					if opts.OnStep != nil {
						opts.OnStep(s)
					}
				},
			})
			gr := GoalResult{Name: g.Name, Rep: rep}
			if err != nil {
				if run == nil {
					run = &Run{Goal: g.Goal, Picker: picker.Name()}
				}
				run.Steps = partial
				run.OK = false
				run.Reason = "error: " + err.Error()
				run.Ms = int(time.Since(t0).Milliseconds())
				run.Stats = picker.Stats()
				gr.Error = err.Error()
			}
			gr.Run = run
			result.Runs = append(result.Runs, gr)
			status := "FAIL"
			if run.OK {
				status = "OK  "
			}
			fmt.Fprintf(out, "--> %s %s  steps=%d  %.1fs\n", status, run.Reason, len(run.Steps), float64(run.Ms)/1000)
		}
	}
	result.Summary = Summarize(result.Runs)
	return result, nil
}

func resetBetweenGoals(ctx context.Context, drv Driver, suite *Suite, g Goal) error {
	start := g.StartURL
	if start == "" {
		start = suite.StartURL
	}
	if suite.Reset == "clear-storage" && start != "" {
		if err := drv.Navigate(ctx, start); err != nil {
			return err
		}
		if err := drv.ClearStorage(ctx); err != nil {
			return err
		}
	}
	if start != "" {
		if err := drv.Navigate(ctx, start); err != nil {
			return err
		}
	}
	drv.Settle(ctx, "")
	return nil
}

// Summarize aggregates goal results.
func Summarize(runs []GoalResult) Summary {
	var s Summary
	var allCandidates []float64
	for _, r := range runs {
		if r.Run == nil {
			continue
		}
		s.Goals++
		if r.OK {
			s.OK++
		}
		s.Steps += len(r.Steps)
		s.Ms += r.Ms
		s.ModelCalls += r.Stats.Calls
		s.ModelMs += r.Stats.Ms
		s.InputTokens += r.Stats.InputTokens
		s.OutputTokens += r.Stats.OutputTokens
		s.USD += r.Stats.USD
		s.Wasted += r.Metrics.Wasted
		s.Fallback += r.Metrics.Fallback
		s.LowConfidence += r.Metrics.LowConfidence
		for _, st := range r.Steps {
			if !IsAction(st) {
				continue
			}
			if st.Candidates > 0 {
				allCandidates = append(allCandidates, float64(st.Candidates))
			}
		}
	}
	if s.Steps > 0 {
		s.MsPerStep = float64(s.Ms) / float64(s.Steps)
	}
	if s.ModelCalls > 0 {
		s.MsPerCall = float64(s.ModelMs) / float64(s.ModelCalls)
	}
	s.CandidatesMedian = median(allCandidates)
	return s
}

// FormatStep renders one step for the terminal.
func FormatStep(s Step) string {
	view := s.View
	if view == "" {
		view = "-"
	}
	line := fmt.Sprintf("%2d. %s  %s", s.N, view, s.Action)
	if s.Probs != "" {
		line += "  [" + s.Probs + "]"
	}
	if s.Action != "done" {
		line += fmt.Sprintf("  done=%.2f", s.DoneProb)
	}
	if s.Ms > 0 {
		line += fmt.Sprintf("  %dms (snap %d, pick %d, act %d, settle %d)", s.Ms, s.MsSnap, s.MsPick, s.MsAct, s.MsSettle)
	}
	return line
}

// FormatTable renders the per-goal table and the summary line.
func FormatTable(res *SuiteResult) string {
	rows := [][]string{{"goal", "ok", "steps", "secs", "model ms", "reason"}}
	for _, r := range res.Runs {
		if r.Run == nil {
			continue
		}
		ok := "✗"
		if r.OK {
			ok = "✓"
		}
		name := r.Name
		if r.Rep > 1 {
			name = fmt.Sprintf("%s#%d", r.Name, r.Rep)
		}
		rows = append(rows, []string{name, ok, fmt.Sprint(len(r.Steps)), fmt.Sprintf("%.1f", float64(r.Ms)/1000), fmt.Sprint(r.Stats.Ms), trunc(r.Reason, 50)})
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, c := range row {
			if l := len([]rune(c)); l > widths[i] {
				widths[i] = l
			}
		}
	}
	var b strings.Builder
	for _, row := range rows {
		for i, c := range row {
			b.WriteString(c)
			b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(c))+2))
		}
		b.WriteString("\n")
	}
	s := res.Summary
	fmt.Fprintf(&b, "condition=%s %s: %d/%d ok · %d steps · %.1fs total · %.2fs/step · model %d calls, %.0f ms/call, %d+%d tok · wasted %d · fallback %d · low-conf %d · candidates ~%.0f", res.Condition, res.Picker, s.OK, s.Goals, s.Steps, float64(s.Ms)/1000, s.MsPerStep/1000, s.ModelCalls, s.MsPerCall, s.InputTokens, s.OutputTokens, s.Wasted, s.Fallback, s.LowConfidence, s.CandidatesMedian)
	if s.USD > 0 {
		fmt.Fprintf(&b, ", $%.3f", s.USD)
	}
	b.WriteString("\n")
	return b.String()
}
