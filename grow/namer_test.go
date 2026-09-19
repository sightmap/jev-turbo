package grow

import (
	"context"
	"testing"

	"github.com/sightmap/jev-turbo/explore"
)

type stubPicker struct {
	answer string
	calls  int
}

func (s *stubPicker) Name() string { return "stub" }
func (s *stubPicker) Pick(ctx context.Context, state string, crit explore.Criteria) (explore.Pick, error) {
	s.calls++
	return explore.Pick{Next: s.answer}, nil
}
func (s *stubPicker) Choose(ctx context.Context, state string, crit explore.Criteria, instructions string) (string, error) {
	s.calls++
	return s.answer, nil
}
func (s *stubPicker) Stats() explore.Stats { return explore.Stats{Calls: s.calls} }

// namerTestNode builds an explore.Node for these tests. naming_test.go
// already declares a package-level `node` helper with a different signature
// (name, tag, attrs), so this one uses a distinct name to avoid colliding.
func namerTestNode(tag, role, name string) *explore.Node {
	return &explore.Node{ID: "1", Tag: tag, Role: role, Name: name, Attrs: map[string]string{}}
}

func TestTemplateNamerSkipsJevForObviousKinds(t *testing.T) {
	p := &stubPicker{answer: "card"}
	n := &TemplateNamer{Picker: p}
	// GroupName drops the stopword "to" (see naming_test.go's "Books to
	// Scrape" -> "BooksScrapeLink" case), so "Add to cart" names AddCartButton.
	d, err := n.Name(context.Background(), Group{Hook: "div.x", Tag: "button", Role: "button", Members: []*explore.Node{namerTestNode("button", "button", "Add to cart")}}, 1)
	if err != nil || d.Kind != "button" || d.Name != "AddCartButton" || p.calls != 0 || n.Calls() != 0 {
		t.Fatalf("got %+v err=%v calls=%d", d, err, p.calls)
	}
	d, err = n.Name(context.Background(), Group{Hook: "ul.results", Tag: "li", Role: "listitem", Members: []*explore.Node{namerTestNode("li", "listitem", "A"), namerTestNode("li", "listitem", "B"), namerTestNode("li", "listitem", "C")}}, 3)
	if err != nil || d.Kind != "card" || p.calls != 1 || n.Calls() != 1 || d.Name == "" {
		t.Fatalf("got %+v err=%v calls=%d", d, err, p.calls)
	}
	p.answer = "noise"
	d, _ = n.Name(context.Background(), Group{Tag: "div", Role: "", Members: []*explore.Node{namerTestNode("div", "", "")}}, 1)
	if d.Kind != "noise" {
		t.Fatalf("got %+v", d)
	}
}
