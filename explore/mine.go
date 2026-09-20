package explore

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A MinedStep is one action of a run in the terms a tool would repeat it:
// the view it was taken on, the verb, the component, the value key a fill
// typed, and the view it reached.
type MinedStep struct {
	View string `json:"view"`
	Verb string `json:"verb"` // fill, click, enter, select, toggle
	Comp string `json:"comp,omitempty"`
	Key  string `json:"key,omitempty"` // the spec value a fill typed
	To   string `json:"to,omitempty"`  // the view reached when the step navigated
}

// token is what two steps must share to count as the same step of a journey.
func (s MinedStep) token() string {
	return s.View + "|" + s.Verb + "|" + s.Comp + "|" + s.Key
}

// A MinedTool is a run of steps that recurs across successful runs, with the
// name a tool layer would give it.
type MinedTool struct {
	Name    string      `json:"name"`
	Steps   []MinedStep `json:"steps"`
	Support int         `json:"support"` // runs the sequence appeared in
	Runs    int         `json:"runs"`    // successful runs mined
}

var (
	filledRe = regexp.MustCompile(`^filled \[(\w+)[^\]]*\] with (\w+)`)
	actedRe  = regexp.MustCompile(`^(clicked|selected|toggled) \[(\w+)`)
)

// MinedSteps turns a run's transitions into steps worth repeating: the meta
// actions (wait, scroll, back, stale) and the picks that carried no component
// are left out, since a tool cannot name them.
func MinedSteps(run *Run) []MinedStep {
	var out []MinedStep
	for _, t := range run.Transitions {
		var s MinedStep
		switch {
		case strings.HasPrefix(t.Action, "pressed Enter"):
			s = MinedStep{View: t.From, Verb: "enter"}
		case filledRe.MatchString(t.Action):
			m := filledRe.FindStringSubmatch(t.Action)
			s = MinedStep{View: t.From, Verb: "fill", Comp: m[1], Key: m[2]}
		case actedRe.MatchString(t.Action):
			m := actedRe.FindStringSubmatch(t.Action)
			verb := map[string]string{"clicked": "click", "selected": "select", "toggled": "toggle"}[m[1]]
			s = MinedStep{View: t.From, Verb: verb, Comp: m[2]}
		default:
			continue
		}
		if t.Changed && t.To != "" && t.To != t.From && !strings.HasPrefix(t.To, "/") {
			s.To = t.To
		}
		out = append(out, s)
	}
	return out
}

// Mine finds the step sequences that recur across the successful runs: every
// contiguous run of 1 to 6 steps, shaped like one tool, seen in at least minSupport runs, keeping
// only the closed ones (a sequence no longer sequence matches with the same
// support), largest support and then longest first.
func Mine(runs []*Run, minSupport int) []MinedTool {
	if minSupport < 2 {
		minSupport = 2
	}
	var seqs [][]MinedStep
	for _, r := range runs {
		if r.OK {
			if s := MinedSteps(r); len(s) > 0 {
				seqs = append(seqs, s)
			}
		}
	}
	support := map[string]int{}
	example := map[string][]MinedStep{}
	for _, seq := range seqs {
		seen := map[string]bool{}
		for n := 1; n <= 6; n++ {
			for i := 0; i+n <= len(seq); i++ {
				sub := seq[i : i+n]
				if !toolShaped(sub) {
					continue
				}
				k := seqKey(sub)
				if seen[k] {
					continue
				}
				seen[k] = true
				support[k]++
				if _, ok := example[k]; !ok {
					example[k] = append([]MinedStep{}, sub...)
				}
			}
		}
	}
	var keys []string
	for k, n := range support {
		if n >= minSupport {
			keys = append(keys, k)
		}
	}
	// Closed: drop a sequence when a longer one contains it at the same support.
	var closed []string
	for _, k := range keys {
		sub := example[k]
		open := false
		for _, other := range keys {
			if other == k || support[other] != support[k] || len(example[other]) <= len(sub) {
				continue
			}
			if containsSeq(example[other], sub) {
				open = true
				break
			}
		}
		if !open {
			closed = append(closed, k)
		}
	}
	sort.SliceStable(closed, func(i, j int) bool {
		if support[closed[i]] != support[closed[j]] {
			return support[closed[i]] > support[closed[j]]
		}
		if len(example[closed[i]]) != len(example[closed[j]]) {
			return len(example[closed[i]]) > len(example[closed[j]])
		}
		return closed[i] < closed[j]
	})
	var out []MinedTool
	for _, k := range closed {
		steps := example[k]
		out = append(out, MinedTool{Name: ToolName(steps), Steps: steps, Support: support[k], Runs: len(seqs)})
	}
	return out
}

// toolShaped reports whether a sequence is the shape of one tool: it stays
// on one view, which is where a tool's ensure_view holds; a step that
// navigates can only be its last, and then only after fills (a form and its
// submit); otherwise the navigating step is a tool of its own, and so is
// what came before it. A journey across views is several tools, not one.
func toolShaped(steps []MinedStep) bool {
	last := len(steps) - 1
	for i, s := range steps {
		if s.View != steps[0].View {
			return false
		}
		if s.To != "" && i < last {
			return false
		}
	}
	if steps[last].To != "" {
		for _, s := range steps[:last] {
			if s.Verb != "fill" {
				return false
			}
		}
	}
	// A fill that nothing follows is half a form, not a tool.
	if steps[last].Verb == "fill" {
		return false
	}
	return true
}

func seqKey(steps []MinedStep) string {
	parts := make([]string, len(steps))
	for i, s := range steps {
		parts[i] = s.token()
	}
	return strings.Join(parts, " > ")
}

func containsSeq(hay, needle []MinedStep) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for j := range needle {
			if hay[i+j].token() != needle[j].token() {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// ToolName names a sequence from what it ends in: a fill followed by Enter
// is a search; otherwise the last control names the tool, without its
// Button, Link or Field suffix (LoginButton is log in, CartLink is cart),
// except that a control whose name says nothing (Continue, Submit, Next)
// names the tool by the view it reaches.
func ToolName(steps []MinedStep) string {
	if len(steps) == 0 {
		return "tool"
	}
	last := steps[len(steps)-1]
	if last.Verb == "enter" && len(steps) > 1 && steps[len(steps)-2].Verb == "fill" {
		return "search"
	}
	name := snake(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(last.Comp, "Button"), "Link"), "Field"))
	generic := map[string]bool{"continue": true, "submit": true, "next": true, "go": true, "ok": true, "": true}
	// A link opens what it reaches, unless its own name is the action, the
	// way a logout link's is.
	if last.To != "" && (generic[name] || (strings.HasSuffix(last.Comp, "Link") && !strings.Contains(name, "log"))) {
		return "open_" + snake(last.To)
	}
	if name == "" {
		return "tool"
	}
	return name
}

var camelRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func snake(s string) string {
	s = camelRe.ReplaceAllString(s, "${1}_${2}")
	s = strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(s))
	return strings.Trim(s, "_")
}

// DescribeMined renders the mined tools as a sightkick tools.yaml draft:
// the same step shapes a tool layer uses, with a fill's value key as a
// parameter and a wait for the view a step reaches.
func DescribeMined(tools []MinedTool) string {
	var b strings.Builder
	b.WriteString("version: 1\ntools:\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "  - name: %s\n", t.Name)
		fmt.Fprintf(&b, "    description: >\n      %s Seen in %d of %d runs.\n", describeSteps(t.Steps), t.Support, t.Runs)
		if t.Steps[0].View != "" {
			fmt.Fprintf(&b, "    ensure_view: %s\n", t.Steps[0].View)
		}
		var params []string
		for _, s := range t.Steps {
			if s.Verb == "fill" && s.Key != "" {
				params = append(params, s.Key)
			}
		}
		if len(params) > 0 {
			b.WriteString("    params:\n")
			for _, p := range params {
				fmt.Fprintf(&b, "      - name: %s\n        type: string\n        required: true\n", p)
			}
		}
		b.WriteString("    steps:\n")
		for _, s := range t.Steps {
			switch s.Verb {
			case "fill":
				fmt.Fprintf(&b, "      - fill:\n          query: %s\n          value: \"{{%s}}\"\n", s.Comp, s.Key)
			case "enter":
				b.WriteString("      - keypress:\n          key: Enter\n")
			case "click", "toggle", "select":
				fmt.Fprintf(&b, "      - click:\n          query: %s\n", s.Comp)
			}
			if s.To != "" {
				fmt.Fprintf(&b, "      - wait_for:\n          view: %s\n", s.To)
			}
		}
	}
	return b.String()
}

func describeSteps(steps []MinedStep) string {
	var parts []string
	for _, s := range steps {
		switch s.Verb {
		case "fill":
			parts = append(parts, fmt.Sprintf("fill %s with {{%s}}", s.Comp, s.Key))
		case "enter":
			parts = append(parts, "press Enter")
		default:
			parts = append(parts, fmt.Sprintf("%s %s", s.Verb, s.Comp))
		}
		if s.To != "" {
			parts[len(parts)-1] += " (reaches " + s.To + ")"
		}
	}
	return "From " + steps[0].View + ": " + strings.Join(parts, ", ") + "."
}
