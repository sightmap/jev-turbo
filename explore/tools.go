package explore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// toolTimeout bounds one sightkick process. A build that never finishes, or a
// call whose wait_for sits on a page that never settles, would otherwise hold
// the whole run open.
const toolTimeout = 60 * time.Second

// ToolPrefix marks a picker option that runs a sightkick tool.
const ToolPrefix = "t:"

type ToolParam struct {
	Type        string
	Description string
	Enum        []string
}

// Tool is one compiled sightkick tool, reduced to what a picker needs.
type Tool struct {
	Name        string
	Description string
	Params      map[string]ToolParam
	Required    []string
	View        string // ensureView; "" when the tool is callable from any view
	Guidance    []Suggestion
}

type Suggestion struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
	When   string `json:"when"`
	View   string `json:"view"`
}

type ToolSet struct {
	Name  string
	Tools []Tool
}

// irFile mirrors the fields of sightkick's compiled IR that matter here.
type irFile struct {
	Name  string `json:"name"`
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema struct {
			Properties map[string]struct {
				Type        string   `json:"type"`
				Description string   `json:"description"`
				Enum        []string `json:"enum"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"inputSchema"`
		EnsureView *struct {
			View string `json:"view"`
		} `json:"ensureView"`
		Guidance []struct {
			Tool, Reason, When, View string
		} `json:"guidance"`
	} `json:"tools"`
}

// ParseIR reads the JSON that `sightkick build` prints.
func ParseIR(b []byte) (*ToolSet, error) {
	var f irFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("tools: parse IR: %w", err)
	}
	ts := &ToolSet{Name: f.Name}
	for _, t := range f.Tools {
		tool := Tool{Name: t.Name, Description: t.Description, Params: map[string]ToolParam{}, Required: t.InputSchema.Required}
		for k, p := range t.InputSchema.Properties {
			tool.Params[k] = ToolParam{Type: p.Type, Description: p.Description, Enum: p.Enum}
		}
		if t.EnsureView != nil {
			tool.View = t.EnsureView.View
		}
		for _, g := range t.Guidance {
			tool.Guidance = append(tool.Guidance, Suggestion{Tool: g.Tool, Reason: g.Reason, When: g.When, View: g.View})
		}
		ts.Tools = append(ts.Tools, tool)
	}
	return ts, nil
}

// LoadTools compiles the tool layer under appDir with `sightkick build`.
func LoadTools(ctx context.Context, appDir string) (*ToolSet, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sightkick", "build", appDir)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tools: sightkick build %s: %v: %s", appDir, err, strings.TrimSpace(errb.String()))
	}
	return ParseIR(out.Bytes())
}

func (ts *ToolSet) Get(name string) *Tool {
	for i := range ts.Tools {
		if ts.Tools[i].Name == name {
			return &ts.Tools[i]
		}
	}
	return nil
}

// ToolOptions lists the tools callable on view whose required params can be
// filled from values. Names in first come first, marked as suggested.
func ToolOptions(ts *ToolSet, view string, values map[string]string, first []string) []Criterion {
	if ts == nil {
		return nil
	}
	rank := map[string]int{}
	for i, n := range first {
		rank[n] = i + 1
	}
	var opts []Criterion
	for i := range ts.Tools {
		t := &ts.Tools[i]
		if t.View != "" && t.View != view {
			continue
		}
		if _, ok := ToolArgs(t, values); !ok {
			continue
		}
		names := make([]string, 0, len(t.Params))
		for k := range t.Params {
			names = append(names, k)
		}
		sort.Strings(names)
		desc := fmt.Sprintf("tool %s(%s): %s", t.Name, strings.Join(names, ", "), t.Description)
		if rank[t.Name] > 0 {
			desc += " (suggested next)"
		}
		opts = append(opts, Criterion{Key: ToolPrefix + t.Name, Desc: desc})
	}
	sort.SliceStable(opts, func(i, j int) bool {
		ri, rj := rank[strings.TrimPrefix(opts[i].Key, ToolPrefix)], rank[strings.TrimPrefix(opts[j].Key, ToolPrefix)]
		if (ri > 0) != (rj > 0) {
			return ri > 0
		}
		return ri < rj
	})
	return opts
}

// ToolArgs fills a tool's params from values by word overlap between the param
// name and the value key. ok is false when a required param has no value.
func ToolArgs(t *Tool, values map[string]string) (map[string]string, bool) {
	args := map[string]string{}
	for name := range t.Params {
		if v, ok := values[name]; ok {
			args[name] = v
			continue
		}
		pw := wordSet(name)
		best, bestScore := "", 0
		for k := range values {
			kw := wordSet(k)
			score := 0
			for w := range kw {
				if pw[w] {
					score++
				}
			}
			// A match must cover kw fully too, or a short param name like
			// "name" would spuriously match a longer key like "first name".
			if score != len(kw) {
				continue
			}
			if score > bestScore || (score == bestScore && score > 0 && k < best) {
				best, bestScore = k, score
			}
		}
		if bestScore > 0 && bestScore == len(pw) {
			args[name] = values[best]
		}
	}
	for _, r := range t.Required {
		if _, ok := args[r]; !ok {
			return args, false
		}
	}
	return args, true
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == '_' || r == ' ' || r == '-' }) {
		out[w] = true
	}
	return out
}

// ToolRunner executes a tool against the live session.
type ToolRunner interface {
	Run(ctx context.Context, tool string, args map[string]string) (ToolResult, error)
}

// ToolResult is sightkick call's stdout object.
type ToolResult struct {
	OK       bool            `json:"ok"`
	Value    json.RawMessage `json:"value,omitempty"`
	Items    json.RawMessage `json:"items,omitempty"`
	Message  string          `json:"message,omitempty"`
	Skipped  bool            `json:"skipped,omitempty"`
	Guidance []Suggestion    `json:"guidance,omitempty"`
}

// SightkickRunner shells out to `sightkick call <appDir> <tool> --param k=v --via cli`.
type SightkickRunner struct {
	AppDir string
}

func (r SightkickRunner) Run(ctx context.Context, tool string, args map[string]string) (ToolResult, error) {
	argv := []string{"call", r.AppDir, tool}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		argv = append(argv, "--param", k+"="+args[k])
	}
	argv = append(argv, "--via", "cli")
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sightkick", argv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run() // exit 1 with ok:false is a normal outcome; the JSON is still on stdout
	var res ToolResult
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &res); err != nil {
		if runErr != nil {
			return res, fmt.Errorf("tools: sightkick call %s: %v: %s", tool, runErr, strings.TrimSpace(errb.String()))
		}
		return res, fmt.Errorf("tools: sightkick call %s: unreadable output: %w", tool, err)
	}
	return res, nil
}
