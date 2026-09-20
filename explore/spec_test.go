package explore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDoneWhen(t *testing.T) {
	d, err := ParseDoneWhen([]string{"view=Cart"})
	if err != nil || d.View != "Cart" || len(d.All) != 0 {
		t.Fatalf("single: %+v %v", d, err)
	}
	d, err = ParseDoneWhen([]string{"url=page-2", "prop=AddToCartButton.label~Remove@InventoryItem.name~Bike Light", "history=Logout"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.All) != 3 || d.All[0].URLContains != "page-2" || d.All[2].HistoryContains != "Logout" {
		t.Fatalf("all: %+v", d)
	}
	pc := d.All[1].Prop
	if pc == nil || pc.Component != "AddToCartButton" || pc.Name != "label" || pc.Contains != "Remove" || pc.Within == nil || pc.Within.Contains != "Bike Light" {
		t.Fatalf("prop: %+v", pc)
	}
	for _, bad := range []string{"nope", "color=red", "prop=Missing~x", "prop=A.b"} {
		if _, err := ParseDoneWhen([]string{bad}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	if d, _ := ParseDoneWhen(nil); d != nil || d.Deterministic() {
		t.Fatal("nil check must not be deterministic")
	}
}

func TestDoneWhenCheck(t *testing.T) {
	row := withProps(mk("1", "generic", "", "div", "", "InventoryItem", false), "name", "Sauce Labs Bike Light")
	btn := withProps(mk("2", "button", "Remove", "button", "", "AddToCartButton", true), "label", "Remove")
	under(row, btn)
	other := withProps(mk("3", "generic", "", "div", "", "InventoryItem", false), "name", "Backpack")
	btn2 := withProps(mk("4", "button", "Add to cart", "button", "", "AddToCartButton", true), "label", "Add to cart")
	under(other, btn2)
	sel := mk("5", "combobox", "Sort", "select", "", "SortSelect", true)
	sel.Value = "Price (low to high)"
	text := mk("6", "heading", "Thank you for your order!", "h2", "", "", false)
	page := &Page{URL: "https://s/inventory.html?x=1", View: "Inventory", Nodes: []*Node{row, btn, other, btn2, sel, text}}
	history := []string{"4. clicked [LogoutLink] → /"}

	cases := []struct {
		name string
		d    DoneWhen
		want bool
	}{
		{"view", DoneWhen{View: "Inventory"}, true},
		{"wrong view", DoneWhen{View: "Cart"}, false},
		{"url", DoneWhen{URLContains: "inventory"}, true},
		{"text", DoneWhen{TextContains: "thank you"}, true},
		{"component", DoneWhen{Component: "SortSelect"}, true},
		{"missing component", DoneWhen{Component: "Nope"}, false},
		{"prop within", DoneWhen{Prop: &PropCheck{Component: "AddToCartButton", Name: "label", Contains: "remove", Within: &PropCheck{Component: "InventoryItem", Name: "name", Contains: "bike light"}}}, true},
		{"prop within wrong row", DoneWhen{Prop: &PropCheck{Component: "AddToCartButton", Name: "label", Contains: "Remove", Within: &PropCheck{Component: "InventoryItem", Name: "name", Contains: "Backpack"}}}, false},
		{"prop falls back to node value", DoneWhen{Prop: &PropCheck{Component: "SortSelect", Name: "value", Contains: "low to high"}}, true},
		{"history", DoneWhen{HistoryContains: "Logout"}, true},
		{"all", DoneWhen{All: []DoneWhen{{View: "Inventory"}, {URLContains: "x=1"}}}, true},
		{"all with one miss", DoneWhen{All: []DoneWhen{{View: "Inventory"}, {URLContains: "nope"}}}, false},
		{"empty", DoneWhen{}, false},
	}
	for _, c := range cases {
		if got := c.d.Check(page, history); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if s := (&DoneWhen{View: "Cart", Prop: &PropCheck{Component: "A", Name: "b", Contains: "c"}}).String(); s != `the page is the "Cart" view and A.b contains "c"` {
		t.Fatalf("String = %q", s)
	}
}

func TestHistoryCount(t *testing.T) {
	d, err := ParseDoneWhen([]string{"history_count=2:Finish"})
	if err != nil {
		t.Fatal(err)
	}
	page := &Page{URL: "/"}
	one := []string{"1. clicked [FinishButton] → /done"}
	if d.Check(page, one) {
		t.Fatal("one mention must not satisfy min 2")
	}
	if !d.Check(page, append(one, "2. clicked button \"Finish\" → /done")) {
		t.Fatal("two mentions satisfy min 2")
	}
	if !strings.Contains(d.String(), "2") {
		t.Fatalf("String() = %q", d.String())
	}
	for _, bad := range []string{"history_count=x:Foo", "history_count=0:Foo", "history_count=2:"} {
		if _, err := ParseDoneWhen([]string{bad}); err == nil {
			t.Fatalf("%q should not parse", bad)
		}
	}
	var s Spec
	if err := json.Unmarshal([]byte(`{"done_when":{"history_count":{"substr":"Finish","min":3}}}`), &s); err != nil || s.DoneWhen.HistoryCount == nil || s.DoneWhen.HistoryCount.Min != 3 {
		t.Fatalf("json: %v %+v", err, s.DoneWhen)
	}
}

// TestTextAbsentAndCount builds a bag with two rows for the same product and one
// for another. A node's rendered text is its subtree's, so the product name sits
// in the row, the heading, the heading's text node and the list; a check that
// counted every node would see it far more than twice.
func TestTextAbsentAndCount(t *testing.T) {
	list := mk("1", "list", "", "ul", "", "", false)
	var nodes []*Node
	nodes = append(nodes, list)
	for i, name := range []string{"BILLY bookcase", "BILLY bookcase", "KALLAX shelf"} {
		id := string(rune('a' + i))
		row := withProps(mk(id, "listitem", "", "li", "", "BagRow", false), "name", name)
		row.Text = name + " $59 Remove"
		heading := mk(id+"h", "heading", name, "h3", "", "", false)
		heading.Text = name
		text := mk(id+"t", "StaticText", name, "", "", "", false)
		remove := mk(id+"r", "button", "Remove", "button", "", "RemoveButton", true)
		remove.Text = "Remove"
		under(list, row)
		under(row, heading, remove)
		under(heading, text)
		nodes = append(nodes, row, heading, text, remove)
	}
	list.Text = "BILLY bookcase $59 Remove BILLY bookcase $59 Remove KALLAX shelf $59 Remove"
	hidden := mk("z", "generic", "PAX wardrobe", "div", "", "", false)
	hidden.Visible = false
	nodes = append(nodes, hidden)
	page := &Page{URL: "/bag", Nodes: nodes}

	if n := pageTextCount(page, "billy bookcase"); n != 2 {
		t.Fatalf("pageTextCount = %d, want one per row", n)
	}
	// A text only a container shows, because it spans its children, counts on
	// the container; a text every button shows counts once per button.
	if n := pageTextCount(page, "$59 remove"); n != 3 {
		t.Fatalf("pageTextCount(container-only text) = %d, want one per row", n)
	}
	if n := pageTextCount(page, "remove"); n != 3 {
		t.Fatalf("pageTextCount(button text) = %d, want one per button", n)
	}
	// An accessible name that restates the product is a further occurrence,
	// as it is for text_contains; a substring must be chosen with that in mind.
	page.Nodes[4].Name = "Remove BILLY bookcase"
	if n := pageTextCount(page, "billy bookcase"); n != 3 {
		t.Fatalf("pageTextCount with a labelled button = %d, want 3", n)
	}
	page.Nodes[4].Name = "Remove"
	cases := []struct {
		name string
		d    DoneWhen
		want bool
	}{
		{"absent", DoneWhen{TextAbsent: "MALM"}, true},
		{"absent but hidden node shows it", DoneWhen{TextAbsent: "PAX wardrobe"}, true},
		{"absent but shown", DoneWhen{TextAbsent: "kallax"}, false},
		{"exact", DoneWhen{TextCount: &TextCount{Substr: "BILLY bookcase", Min: 2, Max: 2}}, true},
		{"exact misses on a second row", DoneWhen{TextCount: &TextCount{Substr: "BILLY bookcase", Min: 1, Max: 1}}, false},
		{"exact one", DoneWhen{TextCount: &TextCount{Substr: "KALLAX", Min: 1, Max: 1}}, true},
		{"at least", DoneWhen{TextCount: &TextCount{Substr: "billy", Min: 1}}, true},
		{"at least too many wanted", DoneWhen{TextCount: &TextCount{Substr: "billy", Min: 3}}, false},
		{"between", DoneWhen{TextCount: &TextCount{Substr: "billy", Min: 1, Max: 3}}, true},
		{"between below", DoneWhen{TextCount: &TextCount{Substr: "kallax", Min: 2, Max: 3}}, false},
		{"between above", DoneWhen{TextCount: &TextCount{Substr: "Remove", Min: 1, Max: 2}}, false},
		{"at most, min zero", DoneWhen{TextCount: &TextCount{Substr: "MALM", Min: 0, Max: 1}}, true},
		{"count over the same text as text_contains", DoneWhen{TextCount: &TextCount{Substr: "PAX wardrobe", Min: 1}}, false},
		{"invalid bounds never pass", DoneWhen{TextCount: &TextCount{Substr: "billy", Min: 2, Max: 1}}, false},
		{"empty substr never passes", DoneWhen{TextCount: &TextCount{Min: 1}}, false},
		{"both zero never passes", DoneWhen{TextCount: &TextCount{Substr: "billy"}}, false},
	}
	for _, c := range cases {
		if got := c.d.Check(page, nil); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
		if !c.d.Deterministic() {
			t.Errorf("%s: must be deterministic", c.name)
		}
	}
	// Whatever text_contains finds, text_count sees at least once, and what it
	// does not find counts zero.
	for _, sub := range []string{"kallax", "Remove KALLAX", "$59", "PAX wardrobe", "MALM"} {
		has := pageHasText(page, sub)
		if got := pageTextCount(page, sub) > 0; got != has {
			t.Errorf("%q: pageHasText %v but pageTextCount > 0 is %v", sub, has, got)
		}
	}

	strs := map[string]*DoneWhen{
		`the page does not show the text "MALM"`:                                     {TextAbsent: "MALM"},
		`the text "billy" appears exactly 1 times`:                                   {TextCount: &TextCount{Substr: "billy", Min: 1, Max: 1}},
		`the text "billy" appears at least 2 times`:                                  {TextCount: &TextCount{Substr: "billy", Min: 2}},
		`the text "billy" appears between 1 and 3 times`:                             {TextCount: &TextCount{Substr: "billy", Min: 1, Max: 3}},
		`the page shows the text "bag" and the text "billy" appears exactly 1 times`: {TextContains: "bag", TextCount: &TextCount{Substr: "billy", Min: 1, Max: 1}},
	}
	for want, d := range strs {
		if got := d.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestParseTextCount(t *testing.T) {
	good := map[string]TextCount{
		"text_count=2:BILLY":      {Substr: "BILLY", Min: 2, Max: 2},
		"text_count=1+:BILLY":     {Substr: "BILLY", Min: 1, Max: 0},
		"text_count=1-3:BILLY":    {Substr: "BILLY", Min: 1, Max: 3},
		"text_count=0-1:BILLY":    {Substr: "BILLY", Min: 0, Max: 1},
		"text_count=1:Price: $59": {Substr: "Price: $59", Min: 1, Max: 1},
		"text_count=2-2:a-b":      {Substr: "a-b", Min: 2, Max: 2},
	}
	for expr, want := range good {
		d, err := ParseDoneWhen([]string{expr})
		if err != nil {
			t.Errorf("%q: %v", expr, err)
			continue
		}
		if d.TextCount == nil || *d.TextCount != want {
			t.Errorf("%q: got %+v want %+v", expr, d.TextCount, want)
		}
	}
	for _, bad := range []string{"text_count=BILLY", "text_count=x:BILLY", "text_count=2:", "text_count=0:BILLY", "text_count=3-1:BILLY", "text_count=-1:BILLY", "text_count=1-x:BILLY", "text_count=+:BILLY", "text_absent="} {
		if _, err := ParseDoneWhen([]string{bad}); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
	d, err := ParseDoneWhen([]string{"text_absent=Only 1 left", "text_count=1:BILLY"})
	if err != nil || len(d.All) != 2 || d.All[0].TextAbsent != "Only 1 left" || d.All[1].TextCount == nil {
		t.Fatalf("all: %+v %v", d, err)
	}
}

func TestTextCountFromJSONIsValidated(t *testing.T) {
	var s Spec
	if err := json.Unmarshal([]byte(`{"done_when":{"text_count":{"substr":"BILLY","min":1,"max":1},"text_absent":"Only 1 left"}}`), &s); err != nil || s.DoneWhen.TextCount == nil || s.DoneWhen.TextCount.Max != 1 || s.DoneWhen.TextAbsent != "Only 1 left" {
		t.Fatalf("json: %v %+v", err, s.DoneWhen)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"empty substr":  `{"done_when":{"text_count":{"min":1}}}`,
		"max below min": `{"done_when":{"all":[{"text_count":{"substr":"BILLY","min":2,"max":1}}]}}`,
		"both zero":     `{"done_when":{"text_count":{"substr":"BILLY"}}}`,
		"negative min":  `{"done_when":{"text_count":{"substr":"BILLY","min":-1}}}`,
	} {
		p := filepath.Join(dir, "spec.json")
		os.WriteFile(p, []byte(body), 0o644)
		if _, err := LoadSpec(p); err == nil || !strings.Contains(err.Error(), "text_count") {
			t.Errorf("%s: LoadSpec error = %v, want one mentioning text_count", name, err)
		}
	}
	p := filepath.Join(dir, "ok.json")
	os.WriteFile(p, []byte(`{"done_when":{"text_count":{"substr":"BILLY","min":0,"max":2}}}`), 0o644)
	if _, err := LoadSpec(p); err != nil {
		t.Fatalf("min 0 with a max is a valid at-most check: %v", err)
	}
}

func TestHistoryCountFromJSONNeedsMinAndSubstr(t *testing.T) {
	var s Spec
	if err := json.Unmarshal([]byte(`{"done_when":{"history_count":{}}}`), &s); err != nil {
		t.Fatal(err)
	}
	// A zero count would otherwise match an empty history and pass the goal at step 1.
	if s.DoneWhen.Check(&Page{URL: "/"}, nil) {
		t.Fatal("history_count with no min and no substr must not be satisfied")
	}
	dir := t.TempDir()
	suitePath := filepath.Join(dir, "suite.json")
	os.WriteFile(suitePath, []byte(`{"name":"s","start_url":"https://s/","goals":[
	  {"name":"fine","goal":"a","spec":{"done_when":{"view":"Cart"}}},
	  {"name":"broken","goal":"b","spec":{"done_when":{"history_count":{}}}}
	]}`), 0o644)
	_, err := LoadSuite(suitePath)
	if err == nil || !strings.Contains(err.Error(), "history_count") || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("LoadSuite error = %v, want one naming the goal and history_count", err)
	}
	specPath := filepath.Join(dir, "spec.json")
	os.WriteFile(specPath, []byte(`{"done_when":{"all":[{"history_count":{"substr":"Finish"}}]}}`), 0o644)
	if _, err := LoadSpec(specPath); err == nil || !strings.Contains(err.Error(), "history_count") {
		t.Fatalf("LoadSpec error = %v, want one mentioning history_count", err)
	}
}

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.json")
	os.WriteFile(p, []byte(`{"done_when":{"view":"Cart"},"values":{"u":"x"},"hint":"h","avoid":["Pay"]}`), 0o644)
	s, err := LoadSpec(p)
	if err != nil || s.DoneWhen.View != "Cart" || s.Values["u"] != "x" || s.Hint != "h" || s.Avoid[0] != "Pay" {
		t.Fatalf("spec = %+v err = %v", s, err)
	}
	if _, err := LoadSpec(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected error")
	}
}
