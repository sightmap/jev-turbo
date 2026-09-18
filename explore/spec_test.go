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
