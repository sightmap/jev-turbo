package explore

import (
	"strings"
	"testing"
)

const irFixture = `{"version":1,"name":"saucedemo","views":[{"name":"Login","route":"/"},{"name":"Inventory","route":"/inventory.html"}],
"tools":[
 {"name":"log_in","description":"Log in.","mode":"live","inputSchema":{"type":"object","properties":{"username":{"type":"string"},"password":{"type":"string"}},"required":["username","password"]},"ensureView":{"view":"Login","route":"/"},"steps":[],
  "guidance":[{"tool":"open_item","reason":"browse","when":"after_navigation","view":"Inventory"}]},
 {"name":"add_to_cart","description":"Add a product by name.","mode":"live","inputSchema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},"ensureView":{"view":"Inventory","route":"/inventory.html"},"steps":[]},
 {"name":"go_to_cart","description":"Open the cart.","mode":"live","inputSchema":{"type":"object","properties":{}},"steps":[]},
 {"name":"fill_checkout_info","description":"Fill the form.","mode":"live","inputSchema":{"type":"object","properties":{"first_name":{"type":"string"},"last_name":{"type":"string"},"postal_code":{"type":"string"}},"required":["first_name","last_name","postal_code"]},"ensureView":{"view":"CheckoutInfo","route":"/checkout-step-one.html"},"steps":[]}
]}`

func TestParseIRAndToolOptions(t *testing.T) {
	ts, err := ParseIR([]byte(irFixture))
	if err != nil || len(ts.Tools) != 4 || ts.Get("log_in") == nil || ts.Get("log_in").View != "Login" || ts.Get("go_to_cart").View != "" {
		t.Fatalf("parse: %v %+v", err, ts)
	}
	values := map[string]string{"username": "standard_user", "password": "secret_sauce", "first name": "Ada"}
	opts := ToolOptions(ts, "Login", values, nil)
	keys := []string{}
	for _, o := range opts {
		keys = append(keys, o.Key)
	}
	if strings.Join(keys, ",") != "t:log_in,t:go_to_cart" {
		t.Fatalf("Login options = %v", keys)
	}
	if !strings.Contains(opts[0].Desc, "username") || !strings.Contains(opts[0].Desc, "Log in.") {
		t.Fatalf("desc = %q", opts[0].Desc)
	}
	opts = ToolOptions(ts, "Inventory", values, nil)
	if len(opts) != 1 || opts[0].Key != "t:go_to_cart" {
		t.Fatalf("add_to_cart needs a name value; got %v", opts)
	}
	values["name"] = "Sauce Labs Backpack"
	opts = ToolOptions(ts, "Inventory", values, []string{"go_to_cart"})
	if len(opts) != 2 || opts[0].Key != "t:go_to_cart" || !strings.Contains(opts[0].Desc, "suggested next") {
		t.Fatalf("suggested first: %v", opts)
	}
	if opts := ToolOptions(ts, "CheckoutInfo", values, nil); len(opts) != 1 {
		t.Fatalf("checkout needs last name and postal code: %v", opts)
	}
	args, ok := ToolArgs(ts.Get("log_in"), values)
	if !ok || args["username"] != "standard_user" || args["password"] != "secret_sauce" {
		t.Fatalf("args = %v %v", args, ok)
	}
	values["last name"] = "Lovelace"
	values["postal code"] = "30301"
	args, ok = ToolArgs(ts.Get("fill_checkout_info"), values)
	if !ok || args["first_name"] != "Ada" || args["postal_code"] != "30301" {
		t.Fatalf("args = %v %v", args, ok)
	}
	if _, err := ParseIR([]byte("nope")); err == nil {
		t.Fatal("bad json should error")
	}
}
