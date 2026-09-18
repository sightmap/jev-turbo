package grow

import "testing"

func TestKindFromRoles(t *testing.T) {
	cases := []struct {
		tag, role, hook string
		count           int
		kind            string
		decisive        bool
	}{
		{"button", "button", "div.x", 1, "button", true},
		{"a", "link", "", 1, "link", true},
		{"a", "link", "ul.nav.nav-list", 14, "nav", true},
		{"a", "link", "nav.pagination", 3, "nav", true},
		{"a", "link", "div.product_pod", 20, "link", true},
		{"input", "textbox", "form", 1, "input", true},
		{"input", "searchbox", "", 1, "input", true},
		{"select", "combobox", "", 1, "select", true},
		{"input", "checkbox", "", 4, "button", true},
		{"li", "listitem", "ul.results", 20, "", false},
		{"article", "", "section", 12, "", false},
		{"div", "", "", 1, "", false},
	}
	for _, c := range cases {
		kind, ok := KindFromRoles(c.tag, c.role, c.hook, c.count)
		if kind != c.kind || ok != c.decisive {
			t.Errorf("%s/%s in %q ×%d: got %q,%v want %q,%v", c.tag, c.role, c.hook, c.count, kind, ok, c.kind, c.decisive)
		}
	}
}
