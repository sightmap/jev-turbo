package explore

import (
	"fmt"
	"strings"
	"testing"
)

func TestCandidatesFiltering(t *testing.T) {
	btn := mk("1", "button", "View details", "a", "", "", true)
	icon := mk("2", "image", "View details", "img", "", "", true)
	inner := mk("3", "button", "View details", "span", "", "", true) // same name, nested, unmatched
	under(btn, icon, inner)
	del := mk("4", "button", "Delete workspace", "button", "", "", true)
	hidden := mk("5", "button", "Hidden", "button", "", "", true)
	hidden.Visible = false
	static := mk("6", "heading", "Title", "h1", "", "", false)
	pay := withProps(mk("7", "button", "Go", "button", "", "PayButton", true), "label", "Pay now")

	got := Candidates([]*Node{btn, icon, inner, del, hidden, static, pay}, CandidateOptions{Avoid: []string{"delete", "Pay"}})
	if len(got) != 1 || got[0].Key != "n1" {
		t.Fatalf("candidates = %v", keys(got))
	}
	if got[0].SeenKey != `clicked button View details` {
		t.Fatalf("seenKey = %q", got[0].SeenKey)
	}
}

func TestCandidatesSkipContainersAndBackdrops(t *testing.T) {
	form := mk("f", "search", "Flight", "div", "", "", true)
	field := mk("i", "combobox", "Where from?", "input", "", "", true)
	under(form, field)
	form.InteractiveDesc = 1
	backdrop := mk("b", "generic", "", "div", "", "", true)
	iconOnly := mk("k", "button", "", "button", "", "", true)
	got := Candidates([]*Node{form, field, backdrop, iconOnly}, CandidateOptions{})
	if strings.Join(keys(got), ",") != "ni,nk" {
		t.Fatalf("candidates = %v", keys(got))
	}
}

func TestCandidatesKeepControlInsideSameNamedContainer(t *testing.T) {
	form := mk("f", "form", "Login", "form", "", "", true)
	user := mk("1", "textbox", "Username", "input", "", "", true)
	login := mk("2", "button", "Login", "input", "type=submit", "", true)
	under(form, user, login)
	form.InteractiveDesc = 2
	keys := []string{}
	for _, c := range Candidates([]*Node{form, user, login}, CandidateOptions{}) {
		keys = append(keys, c.Key)
	}
	if strings.Join(keys, ",") != "n1,n2" {
		t.Fatalf("keys = %v (the form is a container, the button inside it is an action)", keys)
	}
	// The original case still holds: an icon inside a same-named button is a wrapper.
	btn := mk("3", "button", "Search", "button", "", "", true)
	icon := mk("4", "button", "Search", "span", "", "", true)
	under(btn, icon)
	btn.InteractiveDesc = 1
	keys = keys[:0]
	for _, c := range Candidates([]*Node{btn, icon}, CandidateOptions{}) {
		keys = append(keys, c.Key)
	}
	if strings.Join(keys, ",") != "n3" {
		t.Fatalf("keys = %v (the inner span duplicates its button)", keys)
	}
}

func TestCandidatesRepeatGuard(t *testing.T) {
	n := mk("1", "button", "Add", "button", "", "", true)
	seen := map[string]int{"https://s/|" + `clicked button Add`: 2}
	if got := Candidates([]*Node{n}, CandidateOptions{Seen: seen, URL: "https://s/"}); len(got) != 0 {
		t.Fatalf("expected the repeated action hidden, got %v", keys(got))
	}
	if got := Candidates([]*Node{n}, CandidateOptions{Seen: seen, URL: "https://other/"}); len(got) != 1 {
		t.Fatal("the guard is per URL")
	}
}

func TestDescribe(t *testing.T) {
	row := withProps(mk("10", "generic", "", "div", "", "InventoryItem", false), "name", "Backpack")
	add := withProps(mk("11", "button", "Add to cart", "button", "", "AddToCartButton", true), "label", "Add to cart")
	link := mk("12", "link", "Travel", "a", "href=catalogue/travel/index.html", "", true)
	pw := mk("13", "textbox", "Password", "input", "type=password", "", true)
	under(row, add, link)
	cases := map[*Node]string{
		add:  `[AddToCartButton label="Add to cart"] button "Add to cart" in [InventoryItem name="Backpack"]`,
		link: `link "Travel" href=catalogue/travel/index.html in [InventoryItem name="Backpack"]`,
		pw:   `textbox "Password" type=password`,
	}
	for n, want := range cases {
		if got := Describe(n); got != want {
			t.Errorf("Describe(%s) = %q, want %q", n.ID, got, want)
		}
	}
}

func TestBuildCriteriaSmallPage(t *testing.T) {
	a := mk("1", "button", "A", "button", "", "", true)
	b := mk("2", "link", "B", "a", "", "", true)
	b.InViewport = false
	crit := BuildCriteria(Candidates([]*Node{a, b}, CandidateOptions{}), CriteriaOptions{Goal: "x", CanGoBack: true})
	want := []string{"n1", "n2", "back", "scroll", "wait"}
	if strings.Join(crit.Keys(), ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v", crit.Keys())
	}
	crit = BuildCriteria(Candidates([]*Node{a, b}, CandidateOptions{}), CriteriaOptions{Goal: "x"})
	want = []string{"n1", "n2", "scroll", "wait"} // no navigation yet: back would leave the site
	if strings.Join(crit.Keys(), ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v", crit.Keys())
	}
	if crit.Groups != nil {
		t.Fatal("small pages are not grouped")
	}
	// scroll is dropped when everything is already in the viewport
	b.InViewport = true
	crit = BuildCriteria(Candidates([]*Node{a, b}, CandidateOptions{}), CriteriaOptions{Goal: "x"})
	if crit.Has("scroll") {
		t.Fatal("scroll offered with nothing off-screen")
	}
	if crit.Has("enter") {
		t.Fatal("enter offered with no preceding fill")
	}
	if !BuildCriteria(Candidates([]*Node{a}, CandidateOptions{}), CriteriaOptions{Goal: "x", AfterFill: true}).Has("enter") {
		t.Fatal("enter not offered after a fill")
	}
}

func TestBuildCriteriaGroupsAndPromotes(t *testing.T) {
	nav := mk("nav", "navigation", "Main", "nav", "", "", false)
	var nodes []*Node
	for i := 0; i < 65; i++ {
		n := mk("n"+strings.Repeat("x", i%3)+string(rune('a'+i%26))+string(rune('0'+i/26)), "link", "Item", "a", "href=/item", "", true)
		under(nav, n)
		nodes = append(nodes, n)
	}
	sharp := mk("s", "link", "Sharp Objects", "a", "href=/catalogue/sharp-objects_997/index.html", "", true)
	card := withProps(mk("card", "generic", "", "article", "", "ProductCard", false), "title", "Sharp Objects")
	under(card, sharp)
	nodes = append(nodes, sharp)
	crit := BuildCriteria(Candidates(nodes, CandidateOptions{}), CriteriaOptions{Goal: `Find the book "Sharp Objects" and open its page.`, MaxCandidates: 60})
	if crit.Groups == nil {
		t.Fatal("expected grouping")
	}
	if !crit.Has("ns") {
		t.Fatalf("goal-relevant link not promoted: %v", crit.Keys())
	}
	if !crit.Has("g:nav") || len(crit.Groups["g:nav"]) != 65 {
		t.Fatalf("nav group missing or wrong size: %v", crit.Keys())
	}
	for _, o := range crit.Options {
		if o.Key == "g:nav" && !strings.Contains(o.Desc, "65 elements") {
			t.Fatalf("group desc = %q", o.Desc)
		}
	}
	sub := GroupCriteria(crit.Groups["g:nav"])
	if len(sub.Options) != 65 || sub.Has("back") {
		t.Fatalf("group criteria = %d options", len(sub.Options))
	}
}

func TestGoalTokens(t *testing.T) {
	got := GoalTokens("Go to page 2 of the full book list.")
	if strings.Join(got, ",") != "2" {
		t.Fatalf("tokens = %v", got)
	}
	got = GoalTokens(`Open the "Sauce Labs Fleece Jacket" product`)
	if strings.Join(got, ",") != "sauce,labs,fleece,jacket,product" {
		t.Fatalf("tokens = %v", got)
	}
}

func keys(cs []*Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Key)
	}
	return out
}

// listing builds a raw two-card listing in document order: a list holding two
// same-shaped entries, each with a title link and a same-named add button, so
// nothing but the entry tells the buttons apart.
func listing() []*Node {
	list := mk("l", "list", "", "ul", "", "", false)
	list.Depth = 1
	var nodes []*Node
	nodes = append(nodes, list)
	for i, title := range []string{"KALLAX, Shelf unit, white, 30 1/8x30 1/8", "KALLAX, Shelf unit, black-brown, 30 1/8x30 1/8"} {
		card := mk(fmt.Sprintf("c%d", i), "listitem", "", "li", "", "", false)
		card.Depth, card.Parent, card.Classes = 2, list, []string{"plp-card"}
		card.Ancestors = []*Node{list}
		link := mk(fmt.Sprintf("t%d", i), "link", title, "a", "", "", true)
		add := mk(fmt.Sprintf("a%d", i), "button", `Add "KALLAX Shelf unit" to cart`, "button", "", "", true)
		for _, n := range []*Node{link, add} {
			n.Depth, n.Parent, n.Ancestors, n.Landmark = 3, card, []*Node{list, card}, list
		}
		card.InteractiveDesc = 2
		nodes = append(nodes, card, link, add)
	}
	list.InteractiveDesc = 4
	return nodes
}

func TestItemsGroupRawCandidatesByTitle(t *testing.T) {
	nodes := listing()
	annotateItems(nodes)
	if nodes[1].ItemTitle != "KALLAX, Shelf unit, white, 30 1/8x30 1/8" || nodes[4].ItemTitle != "KALLAX, Shelf unit, black-brown, 30 1/8x30 1/8" {
		t.Fatalf("each card should be an item titled by its link: %q / %q", nodes[1].ItemTitle, nodes[4].ItemTitle)
	}
	if nodes[3].Item != nodes[1] || nodes[6].Item != nodes[4] {
		t.Fatal("each add button should point at its own card")
	}
	cands := Candidates(nodes, CandidateOptions{})
	crit := BuildCriteria(cands, CriteriaOptions{MaxCandidates: 1, Goal: "buy a lamp"})
	if len(crit.Groups) != 2 {
		t.Fatalf("expected one group per card, got %d: %+v", len(crit.Groups), crit.Options)
	}
	var seen []string
	for _, o := range crit.Options {
		if strings.HasPrefix(o.Key, "g:") {
			seen = append(seen, o.Desc)
		}
	}
	if len(seen) != 2 || !strings.Contains(seen[0], `listitem "KALLAX, Shelf unit, white`) || !strings.Contains(seen[1], `listitem "KALLAX, Shelf unit, black-brown`) {
		t.Fatalf("groups should be labelled by the card titles, got %v", seen)
	}
}

func TestPromotedRawOptionNamesItsItem(t *testing.T) {
	nodes := listing()
	annotateItems(nodes)
	cands := Candidates(nodes, CandidateOptions{})
	// The goal mentions KALLAX, so every control is promoted out of its group
	// and listed flat; the entry must still travel with it.
	crit := BuildCriteria(cands, CriteriaOptions{MaxCandidates: 1, Goal: "add the KALLAX shelf unit"})
	found := false
	for _, o := range crit.Options {
		if strings.HasPrefix(o.Desc, `button "Add`) && strings.Contains(o.Desc, `in listitem "KALLAX, Shelf unit, black-brown`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("a promoted add button should say which card it is in: %+v", crit.Options)
	}
}
