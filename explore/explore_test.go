package explore

import (
	"context"
	"strings"
	"testing"
)

// loginSite is a two-page fake: a login form, then an inventory page with a cart link.
func loginSite() *fakeDriver {
	user := mk("29", "textbox", "Username", "input", "type=text,name=user-name", "UsernameField", true)
	pass := mk("31", "textbox", "Password", "input", "type=password,name=password", "PasswordField", true)
	login := mk("33", "button", "Login", "input", "type=submit", "LoginButton", true)
	form := mk("27", "form", "Login", "form", "", "", true)
	under(form, user, pass, login)
	loginPage := &fakePage{url: "https://s/", view: "Login", nodes: []*Node{form, user, pass, login}, edges: map[string]string{"33": "https://s/inventory.html"}}

	add := withProps(mk("90", "button", "Add to cart", "button", "", "AddToCartButton", true), "label", "Add to cart")
	cart := withProps(mk("54", "button", "Cart", "a", "href=cart.html", "CartLink", true), "count", "Cart, empty")
	inv := &fakePage{url: "https://s/inventory.html", view: "Inventory", nodes: []*Node{add, cart}, edges: map[string]string{"54": "https://s/cart.html"}}
	cartPage := &fakePage{url: "https://s/cart.html", view: "Cart", nodes: []*Node{mk("1", "button", "Checkout", "button", "", "CheckoutButton", true)}}
	return newFakeDriver("https://s/", loginPage, inv, cartPage)
}

func TestExploreReachesDeterministicDone(t *testing.T) {
	drv := loginSite()
	picker := &fakePicker{script: []string{"UsernameField", "PasswordField", "LoginButton", "CartLink"}}
	run, err := Explore(context.Background(), drv, Options{
		Goal:   "log in and open the cart",
		Spec:   &Spec{DoneWhen: &DoneWhen{View: "Cart"}, Values: map[string]string{"username": "standard_user", "password": "secret_sauce"}},
		Picker: picker,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || run.Reason != "done_when satisfied" {
		t.Fatalf("run not ok: %+v", run)
	}
	// username and password were chosen by field-name overlap, without asking the picker
	if drv.fills["29"] != "standard_user" || drv.fills["31"] != "secret_sauce" {
		t.Fatalf("fills = %v", drv.fills)
	}
	if len(picker.chooses) != 0 {
		t.Fatalf("expected no Choose calls, got %v", picker.chooses)
	}
	// login, then cart: 4 acting steps + the final done step
	if len(run.Steps) != 5 || run.Steps[4].Action != "done" {
		t.Fatalf("steps: %+v", run.Steps)
	}
	if got := run.Steps[0].Action; got != "filled [UsernameField] with username" {
		t.Fatalf("step 1 action = %q", got)
	}
	// transitions carry view names once the next page is seen
	if tr := run.Transitions[2]; tr.From != "Login" || tr.To != "Inventory" || !tr.Changed || tr.Comp != "LoginButton" {
		t.Fatalf("transition = %+v", tr)
	}
	if run.Stats.Calls != 4 {
		t.Fatalf("picker calls = %d", run.Stats.Calls)
	}
}

func TestExploreJudgedDoneWithoutSpec(t *testing.T) {
	drv := loginSite()
	picker := &fakePicker{prefer: []string{"LoginButton"}, done: 0.9}
	run, err := Explore(context.Background(), drv, Options{Goal: "anything", Picker: picker})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || !strings.HasPrefix(run.Reason, "picker judged done") || len(run.Steps) != 1 {
		t.Fatalf("run = %+v", run)
	}
	if len(drv.clicks) != 0 {
		t.Fatalf("judged done must not act, clicks = %v", drv.clicks)
	}
}

func TestExploreGivesUpAtMaxSteps(t *testing.T) {
	drv := loginSite()
	picker := &fakePicker{prefer: []string{"PasswordField"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "x", Spec: &Spec{DoneWhen: &DoneWhen{View: "Cart"}}, Picker: picker, MaxSteps: 3})
	if err != nil {
		t.Fatal(err)
	}
	if run.OK || run.Reason != "no result within 3 steps" || len(run.Steps) != 3 {
		t.Fatalf("run = %+v", run)
	}
}

func TestExploreRepeatGuardHidesLoopingAction(t *testing.T) {
	drv := loginSite()
	// The picker always wants the Password field; after two fills at the same URL it must be hidden.
	picker := &fakePicker{prefer: []string{"PasswordField", "LoginButton"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "x", Spec: &Spec{DoneWhen: &DoneWhen{View: "Inventory"}, Values: map[string]string{"password": "p"}}, Picker: picker, MaxSteps: 6})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK {
		t.Fatalf("expected the guard to force progress, run = %+v", run)
	}
	// two password fills, then the login button
	want := []string{"n31", "n31", "n33"}
	if strings.Join(picker.picks, ",") != strings.Join(want, ",") {
		t.Fatalf("picks = %v, want %v", picker.picks, want)
	}
}

func TestExploreRetriesStaleElement(t *testing.T) {
	drv := loginSite()
	drv.failIDs["33"] = 1
	picker := &fakePicker{prefer: []string{"LoginButton"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "x", Spec: &Spec{DoneWhen: &DoneWhen{View: "Inventory"}}, Picker: picker})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || len(drv.clicks) != 1 {
		t.Fatalf("run = %+v clicks = %v", run, drv.clicks)
	}
}

func TestExploreSkipsElementGoneTwice(t *testing.T) {
	drv := loginSite()
	drv.failIDs["33"] = 2
	picker := &fakePicker{prefer: []string{"LoginButton"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "x", Spec: &Spec{DoneWhen: &DoneWhen{View: "Inventory"}}, Picker: picker, MaxSteps: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || !strings.HasPrefix(run.Steps[0].Action, "stale element, skipped") || len(drv.clicks) != 1 {
		t.Fatalf("run = %+v clicks = %v", run.Steps, drv.clicks)
	}
	// Nothing was acted on, so the skip counts as wasted work; so does step 2,
	// which picks the same control again at the same URL.
	if !run.Steps[0].Wasted || run.Metrics.Wasted != 2 {
		t.Fatalf("a skipped stale element is wasted: step %+v metrics %+v", run.Steps[0], run.Metrics)
	}
}

func TestExploreGroupsLargePages(t *testing.T) {
	// 70 links under one list landmark, plus the one the goal wants under another.
	list := mk("l", "list", "", "ul", "", "", false)
	var nodes []*Node
	nodes = append(nodes, list)
	for i := 0; i < 70; i++ {
		n := mk("a"+strings.Repeat("0", 2)+string(rune('a'+i%26))+string(rune('0'+i/26)), "link", "Category", "a", "href=/c"+string(rune('a'+i%26)), "", true)
		under(list, n)
		nodes = append(nodes, n)
	}
	pager := mk("p", "list", "pager", "ul", "", "", false)
	next := mk("next", "link", "next", "a", "href=/catalogue/page-2.html", "", true)
	under(pager, next)
	nodes = append(nodes, pager, next)
	home := &fakePage{url: "https://b/", nodes: nodes, edges: map[string]string{"next": "https://b/catalogue/page-2.html"}}
	page2 := &fakePage{url: "https://b/catalogue/page-2.html", nodes: []*Node{mk("x", "link", "Home", "a", "href=/", "", true)}}
	drv := newFakeDriver("https://b/", home, page2)

	picker := &fakePicker{prefer: []string{"next"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "Go to page 2 of the full book list.", Spec: &Spec{DoneWhen: &DoneWhen{URLContains: "page-2"}}, Picker: picker})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || len(run.Steps) != 2 {
		t.Fatalf("run = %+v", run)
	}
	// "next" carried the goal token "2" in its href, so it was promoted out of the pager group and picked directly.
	if run.Steps[0].Group != "" {
		t.Fatalf("expected a direct pick, got group %q", run.Steps[0].Group)
	}
}

func TestExploreSecondPickInsideGroup(t *testing.T) {
	list := mk("l", "list", "", "ul", "", "", false)
	var nodes []*Node
	nodes = append(nodes, list)
	for i := 0; i < 70; i++ {
		n := mk("k"+string(rune('a'+i%26))+string(rune('0'+i/26)), "link", "Thing "+string(rune('a'+i%26)), "a", "", "", true)
		under(list, n)
		nodes = append(nodes, n)
	}
	target := nodes[40]
	home := &fakePage{url: "https://b/", nodes: nodes, edges: map[string]string{target.ID: "https://b/done"}}
	done := &fakePage{url: "https://b/done", nodes: []*Node{mk("x", "link", "Home", "a", "", "", true)}}
	drv := newFakeDriver("https://b/", home, done)
	picker := &fakePicker{script: []string{"g:l", "n" + target.ID}, probs: []float64{0.5, 0.8}}
	run, err := Explore(context.Background(), drv, Options{Goal: "zzz", Spec: &Spec{DoneWhen: &DoneWhen{URLContains: "done"}}, Picker: picker})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || run.Steps[0].Group != "g:l" || picker.calls != 2 {
		t.Fatalf("run = %+v calls=%d", run.Steps[0], picker.calls)
	}
	// The second pick's 0.8 is P(option | group); the step records the joint
	// probability, with the group's own half kept separately.
	if got := run.Steps[0].Confidence; got != 0.4 {
		t.Fatalf("confidence = %v, want the joint 0.5*0.8", got)
	}
	if got := run.Steps[0].GroupConfidence; got != 0.5 {
		t.Fatalf("group confidence = %v, want 0.5", got)
	}
}

func TestExploreAsksPickerWhenValuesAreAmbiguous(t *testing.T) {
	field := mk("f", "textbox", "Code", "input", "type=text", "", true)
	page := &fakePage{url: "https://s/", nodes: []*Node{field}}
	drv := newFakeDriver("https://s/", page)
	picker := &fakePicker{prefer: []string{"Code", "zip"}}
	_, err := Explore(context.Background(), drv, Options{Goal: "enter the code", Spec: &Spec{Values: map[string]string{"zip": "30301", "promo": "SAVE"}}, Picker: picker, MaxSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(picker.chooses) != 1 || drv.fills["f"] != "30301" {
		t.Fatalf("chooses=%v fills=%v", picker.chooses, drv.fills)
	}
}

func TestExploreSelectsOption(t *testing.T) {
	sel := mk("s", "combobox", "Sort products", "select", "", "SortSelect", true)
	page := &fakePage{url: "https://s/", nodes: []*Node{sel}}
	drv := newFakeDriver("https://s/", page)
	picker := &fakePicker{prefer: []string{"SortSelect", "Price (low to high)"}}
	run, err := Explore(context.Background(), drv, Options{Goal: "sort by price", Spec: &Spec{DoneWhen: &DoneWhen{HistoryContains: "low to high"}}, Picker: picker})
	if err != nil {
		t.Fatal(err)
	}
	if !run.OK || drv.fills["s"] != "option 1" || run.Steps[0].Action != `selected "Price (low to high)" in [SortSelect]` {
		t.Fatalf("run = %+v fills=%v", run.Steps[0], drv.fills)
	}
}

func TestBuildStateListsEverything(t *testing.T) {
	drv := loginSite()
	page, _ := drv.Observe(context.Background())
	cands := Candidates(page.Nodes, CandidateOptions{})
	s := buildState("log in", &Spec{DoneWhen: &DoneWhen{View: "Cart"}, Values: map[string]string{"username": "u"}}, page, []string{"1. did a thing → /x"}, cands, nil)
	for _, want := range []string{"GOAL: log in", `the page is the "Cart" view`, `username="u"`, "view=Login", "COMPONENTS ON PAGE: LoginButton, PasswordField, UsernameField", "1. did a thing", "n29: [UsernameField]", "back: go back"} {
		if !strings.Contains(s, want) {
			t.Errorf("state missing %q:\n%s", want, s)
		}
	}
}

func TestStepMetrics(t *testing.T) {
	a := mk("1", "button", "Go", "button", "", "GoButton", true)
	b := mk("2", "link", "Other", "a", "", "", true)
	site := func() *fakeDriver {
		return newFakeDriver("/", &fakePage{url: "/", view: "Home", nodes: []*Node{a, b}})
	}
	run, err := Explore(context.Background(), site(), Options{Goal: "go", Picker: &fakePicker{script: []string{"n1", "n1"}}, MaxSteps: 2, HasMap: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 {
		t.Fatalf("steps = %d", len(run.Steps))
	}
	s0, s1 := run.Steps[0], run.Steps[1]
	if s0.Candidates != 2 || s0.Named != 1 || s0.Options < 2 {
		t.Fatalf("counts = candidates %d named %d options %d", s0.Candidates, s0.Named, s0.Options)
	}
	if s0.Confidence != 1 || s0.Fallback || s0.Wasted {
		t.Fatalf("first step: confidence %v fallback %v wasted %v", s0.Confidence, s0.Fallback, s0.Wasted)
	}
	if !s1.Wasted {
		t.Fatalf("second click of the same control at the same URL should be wasted: %+v", s1)
	}
	// Explore's defer folds the steps into Run.Metrics; the bench summary and
	// both READMEs' numbers are read from there, not from the steps.
	if run.Metrics.Steps != 2 || run.Metrics.Wasted != 1 {
		t.Fatalf("run metrics = %+v, want 2 steps and 1 wasted", run.Metrics)
	}
	run, err = Explore(context.Background(), site(), Options{Goal: "go", Picker: &fakePicker{script: []string{"n2"}}, MaxSteps: 1, HasMap: true})
	if err != nil {
		t.Fatal(err)
	}
	if !run.Steps[0].Fallback {
		t.Fatalf("an unnamed pick with a map present is a fallback: %+v", run.Steps[0])
	}
	run, _ = Explore(context.Background(), site(), Options{Goal: "go", Picker: &fakePicker{script: []string{"n2"}}, MaxSteps: 1})
	if run.Steps[0].Fallback {
		t.Fatalf("without a map nothing is a fallback: %+v", run.Steps[0])
	}
}

func TestSuggestionsReobserveWhenOptionsLag(t *testing.T) {
	field := mk("1", "combobox", "Where from?", "input", "aria-autocomplete=list", "OriginField", true)
	// Non-interactive filler so each page has 3+ nodes: fewer trips the loop's
	// own "almost nothing came back" retry, which would (as a side effect)
	// already swap in the option before step 2 starts and defeat this test.
	heading := mk("h", "heading", "Flights", "h1", "", "", false)
	blurb := mk("b", "text", "Search for flights", "p", "", "", false)
	option := mk("2", "option", "Zurich Airport (ZRH)", "li", "", "", true)
	first := &fakePage{url: "/", view: "Home", nodes: []*Node{field, heading, blurb}}
	// A different view than "first" so the test can confirm the step record
	// reflects the page the second look actually landed on, not the first.
	later := &fakePage{url: "/", view: "HomeOpen", nodes: []*Node{field, heading, blurb, option}}
	d := newFakeDriver("/", first)
	// The option only shows up once the "first" page has been observed
	// twice (step 1's look, then step 2's own first look, still lagging);
	// only the loop's second look within step 2 (this task's fix) should see it.
	seenFirst := 0
	d.observeHook = func(p *fakePage) {
		if p == first {
			seenFirst++
			if seenFirst == 2 {
				d.pages["/"] = later
			}
		}
	}
	p := &fakePicker{script: []string{"n1", "n2"}}
	run, err := Explore(context.Background(), d, Options{Goal: "fly from Zurich", Picker: p, MaxSteps: 2, Spec: &Spec{Values: map[string]string{"from": "Zurich"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || !strings.Contains(run.Steps[1].Action, "Zurich Airport") {
		t.Fatalf("expected the option to be picked on step 2, got %+v", run.Steps)
	}
	if len(p.picks) != 2 || p.picks[1] != "n2" {
		t.Fatalf("second pick should see only the option: %v", p.picks)
	}
	if run.Steps[1].View != "HomeOpen" {
		t.Fatalf("step should record the page the second look landed on, got view %q", run.Steps[1].View)
	}
}

func TestToolsMode(t *testing.T) {
	ts, _ := ParseIR([]byte(irFixture))
	login := &fakePage{url: "/", view: "Login", nodes: []*Node{mk("1", "textbox", "Username", "input", "", "UsernameField", true)}}
	inv := &fakePage{url: "/inventory.html", view: "Inventory", nodes: []*Node{mk("2", "button", "Add to cart", "button", "", "AddToCartButton", true)}}
	cart := &fakePage{url: "/cart.html", view: "Cart", nodes: []*Node{mk("3", "button", "Checkout", "button", "", "CheckoutButton", true)}}
	d := newFakeDriver("/", login, inv, cart)
	runner := &fakeToolRunner{drv: d, after: map[string]string{"log_in": "/inventory.html", "go_to_cart": "/cart.html"},
		results: map[string]ToolResult{"log_in": {OK: true, Guidance: []Suggestion{{Tool: "go_to_cart", When: "now"}}}}}
	p := &fakePicker{script: []string{"t:log_in", "t:go_to_cart"}}
	values := map[string]string{"username": "standard_user", "password": "secret_sauce"}
	run, err := Explore(context.Background(), d, Options{Goal: "log in and open the cart", Picker: p,
		Spec: &Spec{Values: values, DoneWhen: &DoneWhen{View: "Cart"}}, Tools: ts, ToolRunner: runner, MaxSteps: 5})
	if err != nil || !run.OK {
		t.Fatalf("run: %v %+v", err, run)
	}
	if strings.Join(runner.calls, "; ") != "log_in password=secret_sauce username=standard_user; go_to_cart" {
		t.Fatalf("calls = %v", runner.calls)
	}
	if run.Steps[0].Tool != "log_in" || !run.Steps[0].ToolOK || run.Steps[0].Fallback {
		t.Fatalf("step 1 = %+v", run.Steps[0])
	}
	if len(d.clicks) != 0 {
		t.Fatalf("no element clicks expected, got %v", d.clicks)
	}
}

func TestToolFailureFallsBackToElements(t *testing.T) {
	ts, _ := ParseIR([]byte(irFixture))
	site := func() *fakeDriver {
		login := &fakePage{url: "/", view: "Login", nodes: []*Node{mk("1", "button", "Log in", "button", "", "LoginButton", true)}, edges: map[string]string{"1": "/inventory.html"}}
		inv := &fakePage{url: "/inventory.html", view: "Inventory", nodes: []*Node{mk("2", "button", "Add to cart", "button", "", "AddToCartButton", true)}}
		return newFakeDriver("/", login, inv)
	}
	failing := func(d *fakeDriver) *fakeToolRunner {
		return &fakeToolRunner{drv: d, results: map[string]ToolResult{"log_in": {OK: false, Message: "wait_for timed out"}}}
	}
	spec := func() *Spec {
		return &Spec{Values: map[string]string{"username": "u", "password": "p"}, DoneWhen: &DoneWhen{View: "Inventory"}}
	}

	d := site()
	p := &fakePicker{script: []string{"t:log_in"}, prefer: []string{"t:log_in", "n1"}}
	run, err := Explore(context.Background(), d, Options{Goal: "log in", Picker: p, Spec: spec(), Tools: ts, ToolRunner: failing(d), MaxSteps: 4})
	if err != nil || !run.OK {
		t.Fatalf("run: %v %+v", err, run)
	}
	if run.Steps[0].Tool != "log_in" || run.Steps[0].ToolOK || !run.Steps[0].Wasted {
		t.Fatalf("failed tool step = %+v", run.Steps[0])
	}
	// The pick after a failed tool is offered every tool but that one, so the
	// sticky preference for "t:log_in" cannot be honoured and the picker falls
	// through to "n1", the login page's only button.
	if len(p.picks) < 2 || p.picks[1] != "n1" {
		t.Fatalf("picks = %v", p.picks)
	}

	// Only the failed tool is skipped, not the layer: the fixture's go_to_cart
	// has no ensureView, so it is offered anywhere, including on this pick.
	d = site()
	p = &fakePicker{script: []string{"t:log_in"}, prefer: []string{"t:log_in", "t:go_to_cart"}}
	if _, err := Explore(context.Background(), d, Options{Goal: "log in", Picker: p, Spec: spec(), Tools: ts, ToolRunner: failing(d), MaxSteps: 4}); err != nil {
		t.Fatal(err)
	}
	if len(p.picks) < 2 || p.picks[1] != "t:go_to_cart" {
		t.Fatalf("picks = %v", p.picks)
	}
}

func TestFailedToolThatFinishesTheGoalIsNotWasted(t *testing.T) {
	ts, _ := ParseIR([]byte(irFixture))
	login := &fakePage{url: "/", view: "Login", nodes: []*Node{mk("1", "button", "Log in", "button", "", "LoginButton", true)}}
	inv := &fakePage{url: "/inventory.html", view: "Inventory", nodes: []*Node{mk("2", "button", "Add to cart", "button", "", "AddToCartButton", true)}}
	d := newFakeDriver("/", login, inv)
	// The call reports a failure, its wait_for having given up, but it left the
	// page on the view the goal asks for.
	runner := &fakeToolRunner{drv: d, after: map[string]string{"log_in": "/inventory.html"},
		results: map[string]ToolResult{"log_in": {OK: false, Message: "wait_for timed out"}}}
	p := &fakePicker{script: []string{"t:log_in"}}
	run, err := Explore(context.Background(), d, Options{Goal: "log in", Picker: p,
		Spec:  &Spec{Values: map[string]string{"username": "u", "password": "p"}, DoneWhen: &DoneWhen{View: "Inventory"}},
		Tools: ts, ToolRunner: runner, MaxSteps: 4})
	if err != nil || !run.OK {
		t.Fatalf("run: %v %+v", err, run)
	}
	if run.Steps[0].Wasted || run.Metrics.Wasted != 0 {
		t.Fatalf("a failed call the next page finishes is not wasted: step %+v metrics %+v", run.Steps[0], run.Metrics)
	}
}

func TestRepeatedToolCallAtOneURLIsWasted(t *testing.T) {
	ts, _ := ParseIR([]byte(irFixture))
	page := &fakePage{url: "/", view: "Login", nodes: []*Node{mk("1", "button", "Log in", "button", "", "LoginButton", true)}}
	d := newFakeDriver("/", page)
	// go_to_cart is callable from any view, and this after map leaves the page
	// where it was, so the second call repeats the first exactly.
	runner := &fakeToolRunner{drv: d, after: map[string]string{"go_to_cart": "/"}}
	p := &fakePicker{prefer: []string{"t:go_to_cart"}}
	run, err := Explore(context.Background(), d, Options{Goal: "open the cart", Picker: p,
		Spec: &Spec{DoneWhen: &DoneWhen{View: "Cart"}}, Tools: ts, ToolRunner: runner, MaxSteps: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || run.Steps[0].Wasted || !run.Steps[1].Wasted {
		t.Fatalf("the second call of one tool at one URL is wasted: %+v", run.Steps)
	}
}

func TestEnterIsPressedInTheFilledField(t *testing.T) {
	// A fill can leave focus elsewhere (a consent banner's focus trap, a
	// modal); Enter must go to the field that was filled, not to focus.
	field := mk("1", "combobox", "Search by product", "input", "type=search", "", true)
	banner := mk("2", "button", "Ok", "button", "", "", true)
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{field, banner}}
	d := newFakeDriver("/", home)
	p := &fakePicker{script: []string{"n1", "enter"}}
	run, err := Explore(context.Background(), d, Options{Goal: "search", Picker: p, MaxSteps: 2, Spec: &Spec{Values: map[string]string{"search": "KALLAX"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || run.Steps[1].Action != "pressed Enter" {
		t.Fatalf("expected fill then Enter, got %+v", run.Steps)
	}
	if got := strings.Join(d.clicks, ","); !strings.Contains(got, "enter in 1") {
		t.Fatalf("Enter should target the filled field n1, driver saw: %s", got)
	}
}

func TestEnterAfterAnotherActionTargetsFocus(t *testing.T) {
	// Once another element has been acted on, the filled field is no longer
	// where Enter belongs; it goes to whatever holds focus, as before.
	field := mk("1", "combobox", "Search by product", "input", "type=search", "", true)
	other := mk("2", "button", "Ok", "button", "", "", true)
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{field, other}}
	d := newFakeDriver("/", home)
	p := &fakePicker{script: []string{"n1", "n2", "n1", "enter"}}
	// Enter is only offered right after a fill, so the script refills before pressing it.
	_, err := Explore(context.Background(), d, Options{Goal: "search", Picker: p, MaxSteps: 4, Spec: &Spec{Values: map[string]string{"search": "KALLAX", "again": "KALLAX 2"}}})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(d.clicks, ",")
	if !strings.Contains(got, "enter in 1") || strings.Contains(got, ",enter,") || strings.HasSuffix(got, ",enter") {
		t.Fatalf("Enter after a refill should again target n1, driver saw: %s", got)
	}
}

func TestActionSummaryNamesTheOwningComponent(t *testing.T) {
	// Sixteen "Add to cart" buttons on a listing share one label; the card
	// each belongs to is what makes the history line useful afterwards.
	card := mk("c", "group", "", "div", "", "ProductCard", false)
	card.Props = map[string]string{"title": "KALLAX, Shelf unit, white, 30 1/8x30 1/8", "price": "49.99"}
	add := mk("1", "button", "Add \"KALLAX Shelf unit\" to cart", "button", "", "AddToCartButton", true)
	add.ParentComp = card
	page := &fakePage{url: "/search", view: "Search", nodes: []*Node{card, add}}
	d := newFakeDriver("/search", page)
	run, err := Explore(context.Background(), d, Options{Goal: "add the 2x2", Picker: &fakePicker{script: []string{"n1"}}, MaxSteps: 1, HasMap: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `clicked [AddToCartButton] in [ProductCard price="49.99" title="KALLAX, Shelf unit, white, 30 1/8x30 1/8"]`
	if got := run.Steps[0].Action; got != want {
		t.Fatalf("summary should name the owning card:\n got %s\nwant %s", got, want)
	}
}

func TestEffectValueWhenTheFillLands(t *testing.T) {
	// The next observation shows the typed value in the field, so the fill
	// counts as having done something. The run hits its step limit after the
	// second step, so that one has no observation to judge it by.
	field := mk("1", "textbox", "Search by product", "input", "type=search", "", true)
	button := mk("2", "button", "Search", "button", "", "", true)
	blurb := mk("b", "text", "Find furniture", "p", "", "", false)
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{field, button, blurb}}
	d := newFakeDriver("/", home)
	d.observeHook = func(p *fakePage) {
		if v, ok := d.fills["1"]; ok {
			field.Value = v
		}
	}
	p := &fakePicker{script: []string{"n1", "n2"}}
	run, err := Explore(context.Background(), d, Options{Goal: "search", Picker: p, MaxSteps: 2, Spec: &Spec{Values: map[string]string{"search": "KALLAX"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || !strings.HasPrefix(run.Steps[0].Action, "filled") {
		t.Fatalf("expected a fill then a click, got %+v", run.Steps)
	}
	if run.Steps[0].Effect != "value" {
		t.Fatalf("a fill that landed has effect value, got %q", run.Steps[0].Effect)
	}
	if run.Steps[1].Effect != "" {
		t.Fatalf("the last step of a run that hit its limit has no effect yet, got %q", run.Steps[1].Effect)
	}
}

func TestEffectNavigatedWhenTheURLMoves(t *testing.T) {
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{mk("1", "link", "Products", "a", "href=/products", "", true), mk("h", "heading", "Shop", "h1", "", "", false), mk("b", "text", "Welcome", "p", "", "", false)}, edges: map[string]string{"1": "/products"}}
	products := &fakePage{url: "/products", view: "Products", nodes: []*Node{mk("2", "button", "Add to cart", "button", "", "", true), mk("h2", "heading", "Products", "h1", "", "", false), mk("b2", "text", "All", "p", "", "", false)}}
	d := newFakeDriver("/", home, products)
	p := &fakePicker{script: []string{"n1", "n2"}}
	run, err := Explore(context.Background(), d, Options{Goal: "add", Picker: p, MaxSteps: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || run.Steps[0].Effect != "navigated" {
		t.Fatalf("a click that moved the URL has effect navigated, got %+v", run.Steps)
	}
}

func TestEffectNoneWhenNothingChanges(t *testing.T) {
	// The click has no edge and the page reads the same afterwards: the step
	// did nothing, whatever the driver reported.
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{mk("1", "button", "Ok", "button", "", "", true), mk("2", "button", "Cancel", "button", "", "", true), mk("b", "text", "Cookies", "p", "", "", false)}}
	d := newFakeDriver("/", home)
	p := &fakePicker{script: []string{"n1", "n2"}}
	run, err := Explore(context.Background(), d, Options{Goal: "dismiss", Picker: p, MaxSteps: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || run.Steps[0].Effect != "none" {
		t.Fatalf("a click that changed nothing has effect none, got %+v", run.Steps)
	}
	if run.Metrics.NoEffect != 1 {
		t.Fatalf("metrics should count the one no-effect step, got %+v", run.Metrics)
	}
}

func TestEffectChangedWhenTheControlsDiffer(t *testing.T) {
	// The click stays on the URL but the page offers a different control
	// afterwards (a menu opened), which is an effect short of a navigation.
	menu := mk("1", "button", "Menu", "button", "", "", true)
	entry := mk("2", "link", "Products", "a", "href=/products", "", true)
	blurb := mk("b", "text", "Welcome", "p", "", "", false)
	heading := mk("h", "heading", "Shop", "h1", "", "", false)
	home := &fakePage{url: "/", view: "Home", nodes: []*Node{menu, heading, blurb}}
	d := newFakeDriver("/", home)
	d.observeHook = func(p *fakePage) {
		if len(d.clicks) > 0 {
			p.nodes = []*Node{menu, entry, heading, blurb}
		}
	}
	p := &fakePicker{script: []string{"n1", "n2"}}
	run, err := Explore(context.Background(), d, Options{Goal: "open products", Picker: p, MaxSteps: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 2 || run.Steps[0].Effect != "changed" {
		t.Fatalf("a click that opened a menu has effect changed, got %+v", run.Steps)
	}
}

func TestToolStepEffectIsItsOwnOutcome(t *testing.T) {
	ts, _ := ParseIR([]byte(irFixture))
	login := &fakePage{url: "/", view: "Login", nodes: []*Node{mk("1", "textbox", "Username", "input", "", "UsernameField", true)}}
	inv := &fakePage{url: "/inventory.html", view: "Inventory", nodes: []*Node{mk("2", "button", "Add to cart", "button", "", "AddToCartButton", true)}}
	d := newFakeDriver("/", login, inv)
	runner := &fakeToolRunner{drv: d, after: map[string]string{"log_in": "/inventory.html"},
		results: map[string]ToolResult{"go_to_cart": {OK: false, Message: "wait_for timed out"}}}
	p := &fakePicker{script: []string{"t:go_to_cart", "t:log_in"}}
	run, err := Explore(context.Background(), d, Options{Goal: "log in", Picker: p,
		Spec:  &Spec{Values: map[string]string{"username": "u", "password": "p"}, DoneWhen: &DoneWhen{View: "Inventory"}},
		Tools: ts, ToolRunner: runner, MaxSteps: 4})
	if err != nil || !run.OK {
		t.Fatalf("run: %v %+v", err, run)
	}
	if len(run.Steps) < 3 || run.Steps[0].Effect != "none" || run.Steps[1].Effect != "navigated" {
		t.Fatalf("a failed call that stayed put is none and one that moved the URL is navigated, got %+v", run.Steps)
	}
}

func TestAmbiguousCountsSameNamedCandidates(t *testing.T) {
	// Two add buttons read the same to the picker; the checkout button does
	// not, so two of the three candidates are ambiguous.
	home := &fakePage{url: "/", view: "Listing", nodes: []*Node{
		mk("1", "button", "Add to cart", "button", "", "", true),
		mk("2", "button", "Add to cart", "button", "", "", true),
		mk("3", "button", "Checkout", "button", "", "", true),
	}}
	d := newFakeDriver("/", home)
	p := &fakePicker{script: []string{"n3"}}
	run, err := Explore(context.Background(), d, Options{Goal: "check out", Picker: p, MaxSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 1 || run.Steps[0].Candidates != 3 || run.Steps[0].Ambiguous != 2 {
		t.Fatalf("expected 2 of 3 candidates ambiguous, got %+v", run.Steps)
	}
	if run.Metrics.AmbiguousMedian != 2 {
		t.Fatalf("metrics median should be 2, got %+v", run.Metrics)
	}
}
