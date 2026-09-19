package grow

import (
	"context"
	"fmt"
	"strings"

	"github.com/sightmap/jev-turbo/explore"
)

// Group is a set of orphans that share a container hook, tag, and role.
type Group struct {
	Hook       string
	Tag        string
	Role       string
	Members    []*explore.Node
	Candidates []string // the first member's own ranked selector candidates
}

// Decision is what a Namer decided about a Group.
type Decision struct {
	Kind string // one of Kinds; "noise" skips the group
	Name string // component name; empty means the template name
}

// Namer decides a Group's kind and name. TemplateNamer is the default:
// obvious kinds are decided from roles alone, and only ambiguous groups
// (repeated containers where nav, card, and noise need judgment) reach the
// underlying picker.
type Namer interface {
	Name(ctx context.Context, g Group, offlineCount int) (Decision, error)
	Calls() int
}

// TemplateNamer names groups from KindFromRoles first, falling back to a Jev
// Choose call for the groups that rule can't decide, then names the group
// from a template.
type TemplateNamer struct {
	Picker explore.Picker
	calls  int
}

// Name decides a Group's kind and name. It only counts a call against Calls
// when the decision needed the underlying picker.
func (t *TemplateNamer) Name(ctx context.Context, g Group, offlineCount int) (Decision, error) {
	kind, ok := KindFromRoles(g.Tag, g.Role, g.Hook, len(g.Members))
	if !ok {
		var examples []string
		for i, m := range g.Members {
			if i >= 5 {
				break
			}
			e := fmt.Sprintf("%q", truncate(m.Name, 40))
			if h := m.Attrs["href"]; h != "" {
				e += " → " + truncate(h, 40)
			}
			examples = append(examples, e)
		}
		desc := fmt.Sprintf("%d × <%s> role=%s inside %s; examples: %s", len(g.Members), g.Tag, g.Role, orDefault(g.Hook, "(no stable container)"), strings.Join(examples, ", "))
		t.calls++
		answer, err := t.Picker.Choose(ctx, "ELEMENTS: "+desc, explore.Criteria{Options: Kinds}, "What kind of UI elements are these? Answer noise if they are not worth naming as a component.")
		if err != nil {
			return Decision{}, fmt.Errorf("grow: classify: %w", err)
		}
		if answer == "" {
			answer = "noise"
		}
		kind = answer
	}
	if kind == "noise" {
		return Decision{Kind: kind}, nil
	}
	return Decision{Kind: kind, Name: GroupName(g.Hook, g.Tag, g.Role, g.Members, kind, offlineCount)}, nil
}

// Calls reports how many times Name reached the underlying picker.
func (t *TemplateNamer) Calls() int { return t.calls }
