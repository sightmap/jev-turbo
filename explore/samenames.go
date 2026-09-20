package explore

import (
	"fmt"
	"sort"
	"strings"
)

// SameNameGroup is a set of controls on one page that a person using a screen
// reader, and a picker reading the tree, cannot tell apart: the same role and
// the same accessible name. Each member carries what would tell them apart
// when the page has it, the entry or component the control sits in, and the
// accessible name that would say so.
type SameNameGroup struct {
	Desc    string           `json:"desc"`
	Members []SameNameMember `json:"members"`
}

// SameNameMember is one control of a same-name group.
type SameNameMember struct {
	Key      string `json:"key"`
	Owner    string `json:"owner,omitempty"`    // the entry or component the control sits in, when known
	Proposed string `json:"proposed,omitempty"` // an accessible name that would tell it apart, when the owner is known
}

// SameNames lists the same-name groups among a page's candidates, largest
// first. A group with no owner for its members is the one a map, or a fix to
// the page, has to supply.
func SameNames(page *Page, avoid []string) []SameNameGroup {
	byDesc := map[string][]*Candidate{}
	var order []string
	for _, c := range Candidates(page.Nodes, CandidateOptions{Avoid: avoid}) {
		if c.Node.Name == "" {
			continue // a control with no name at all is a different defect from two with the same one
		}
		if _, ok := byDesc[c.Desc]; !ok {
			order = append(order, c.Desc)
		}
		byDesc[c.Desc] = append(byDesc[c.Desc], c)
	}
	var out []SameNameGroup
	for _, d := range order {
		members := byDesc[d]
		if len(members) < 2 {
			continue
		}
		g := SameNameGroup{Desc: d}
		for _, c := range members {
			m := SameNameMember{Key: c.Key}
			var owner, title string
			switch {
			case c.Node.ParentComp != nil && c.Node.ParentComp.Comp != c.Node.Comp:
				owner = CompLabel(c.Node.ParentComp)
				title = c.Node.ParentComp.Props["title"]
				if title == "" {
					title = c.Node.ParentComp.Props["label"]
				}
			case c.Node.Item != nil:
				owner = ItemLabel(c.Node.Item)
				title = c.Node.Item.ItemTitle
			}
			m.Owner = owner
			if title != "" && c.Node.Name != "" {
				m.Proposed = fmt.Sprintf("%s (%s)", c.Node.Name, title)
			}
			g.Members = append(g.Members, m)
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Members) > len(out[j].Members) })
	return out
}

// DescribeSameNames renders the report for a person.
func DescribeSameNames(groups []SameNameGroup) string {
	var b strings.Builder
	controls, named := 0, 0
	for _, g := range groups {
		fmt.Fprintf(&b, "%d × %s\n", len(g.Members), g.Desc)
		for _, m := range g.Members {
			controls++
			switch {
			case m.Proposed != "":
				named++
				fmt.Fprintf(&b, "    %s  in %s\n        could read: %q\n", m.Key, m.Owner, m.Proposed)
			case m.Owner != "":
				fmt.Fprintf(&b, "    %s  in %s\n", m.Key, m.Owner)
			default:
				fmt.Fprintf(&b, "    %s  no entry or component tells it apart\n", m.Key)
			}
		}
	}
	fmt.Fprintf(&b, "%d controls in %d same-name groups; %d have an entry or component to name them by, %d do not\n", controls, len(groups), named, controls-named)
	return b.String()
}
