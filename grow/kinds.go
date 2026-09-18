package grow

import "strings"

// KindFromRoles decides the obvious cases from tag and role. It is not
// decisive for repeated containers, where nav, card and noise need judgment.
// Kind drives only the name suffix and the description, so roles that need the
// same two collapse: checkbox, radio and switch map to button, and a combobox
// that is not a <select> maps to input.
func KindFromRoles(tag, role, hook string, count int) (kind string, decisive bool) {
	switch role {
	case "button", "menuitem", "tab", "checkbox", "radio", "switch":
		return "button", true
	case "link":
		// nav and link share the Link suffix in kindSuffix, so this split
		// changes the description and nothing else.
		if count >= 3 && isNavHook(hook) {
			return "nav", true
		}
		return "link", true
	case "textbox", "searchbox", "spinbutton":
		return "input", true
	case "listbox":
		return "select", true
	case "combobox":
		if tag == "select" {
			return "select", true
		}
		return "input", true
	}
	switch tag {
	case "button":
		return "button", true
	case "a":
		if count >= 3 && isNavHook(hook) {
			return "nav", true
		}
		return "link", true
	case "input", "textarea":
		return "input", true
	case "select":
		return "select", true
	}
	return "", false
}

// isNavHook reports whether a container selector reads as navigation.
func isNavHook(hook string) bool {
	h := strings.ToLower(hook)
	for _, w := range []string{"nav", "tablist", "pagination", "pager", "breadcrumb", "menu"} {
		if strings.Contains(h, w) {
			return true
		}
	}
	return false
}
