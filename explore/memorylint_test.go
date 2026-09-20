package explore

import (
	"strings"
	"testing"

	"github.com/sightmap/sightmap/go/sightmap"
)

func TestLintMemoryFlagsPrescriptions(t *testing.T) {
	c := &sightmap.Corpus{
		Memory: []string{
			"The header search field and the bag link are on every page.",
			"The flow is search → product page.",
		},
		Views: []sightmap.ViewDef{
			{Name: "Home", Memory: []string{"Nothing on this page is needed for the bag flow beyond the header globals."}},
			{Name: "Search", Memory: []string{"Every card's title starts with the series name and a comma."}},
		},
		GlobalComponents: []sightmap.ComponentDef{
			{Name: "SurveyDialog", Memory: []string{"Answering it is never part of a shopping task — always close it instead of picking a rating."}},
			{Name: "BagLink", Memory: []string{"The count in the label is the number of items in the bag."}},
		},
	}
	if got := len(MemoryNotes(c)); got != 6 {
		t.Fatalf("notes = %d, want 6", got)
	}
	got := LintMemory(c)
	want := []MemoryFinding{
		{MemoryNote{"corpus", "The flow is search → product page."}, "prescribes a route"},
		{MemoryNote{"view Home", "Nothing on this page is needed for the bag flow beyond the header globals."}, "rules a page out"},
		{MemoryNote{"component SurveyDialog", "Answering it is never part of a shopping task — always close it instead of picking a rating."}, "commands an action, forbids an action, prescribes a route"},
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLintMemoryPatterns(t *testing.T) {
	for note, reason := range map[string]string{
		"Search is the only way to reach a named variant.":       "prescribes a route",
		"Take the menu instead of the search box.":               "prescribes a route",
		"Do not open the photo search.":                          "forbids an action",
		"Don't pick a rating.":                                   "forbids an action",
		"The banner must be closed first.":                       "commands an action",
		"Start by closing the survey.":                           "prescribes a route",
		"The category menu is the right path to a series.":       "prescribes a route",
		"Nothing else on the page matters.":                      "rules everything else out",
		"nothing here\nis needed":                                "rules a page out",
		"The sheet is modal; clicks behind it fail as covered.":  "",
		"Nothingness is a word, and kneaded dough is not needed": "",
	} {
		if got := lintNote(note); got != reason {
			t.Errorf("%q: reason %q, want %q", note, got, reason)
		}
	}
}

func TestLintMemoryCleanCorpus(t *testing.T) {
	c := &sightmap.Corpus{
		Memory: []string{"ikea.com hides its sticky header when the page is scrolled down."},
		Views:  []sightmap.ViewDef{{Name: "Bag", Memory: []string{"The page also renders a grid of suggested add-ons that look like bag rows."}}},
	}
	if got := LintMemory(c); len(got) != 0 {
		t.Fatalf("clean corpus flagged: %+v", got)
	}
	if got := LintMemory(nil); len(got) != 0 || len(MemoryNotes(nil)) != 0 {
		t.Fatalf("nil corpus: %+v", got)
	}
	// A global reused in a view is one component, so its note is one finding.
	dup := &sightmap.Corpus{
		GlobalComponents: []sightmap.ComponentDef{{Name: "X", Memory: []string{"Never click it twice."}}},
		Views:            []sightmap.ViewDef{{Name: "V", Components: []sightmap.ComponentDef{{Name: "X", Memory: []string{"Never click it twice."}}}}},
	}
	if got := LintMemory(dup); len(got) != 1 || !strings.HasPrefix(got[0].Where, "component X") {
		t.Fatalf("a global reused in a view is one component: %+v", got)
	}
}
