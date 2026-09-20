package explore

import (
	"regexp"
	"strings"

	"github.com/sightmap/sightmap/go/sightmap"
)

// MemoryNote is one memory line of a corpus and where it was written.
type MemoryNote struct {
	Where string // "corpus", "view <Name>", or "component <Name>"
	Text  string
}

// MemoryFinding is a memory note that reads as a prescription: it tells the
// picker what to do or where not to go, rather than describing what the
// page is. A note like that was written for one goal and steers every other
// goal the same way, which is how the IKEA map talked the picker out of the
// route that worked.
type MemoryFinding struct {
	MemoryNote
	Reason string
}

// memoryPatterns are the phrasings that mark a note as a prescription, each
// with the reason the lint reports. A note is matched case-insensitively and
// across line breaks, since a YAML block keeps them.
var memoryPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`(?is)\bnothing\b.*\bneeded\b`), "rules a page out"},
	{regexp.MustCompile(`(?i)\bnothing else\b`), "rules everything else out"},
	{regexp.MustCompile(`(?i)\bonly way\b`), "prescribes a route"},
	{regexp.MustCompile(`(?i)\bthe flow is\b`), "prescribes a route"},
	{regexp.MustCompile(`(?i)\balways\b`), "commands an action"},
	{regexp.MustCompile(`(?i)\bnever\b`), "forbids an action"},
	{regexp.MustCompile(`(?i)\binstead of\b`), "prescribes a route"},
	{regexp.MustCompile(`(?i)\bdo not\b`), "forbids an action"},
	{regexp.MustCompile(`(?i)\bdon['’]?t\b`), "forbids an action"},
	{regexp.MustCompile(`(?i)\bmust\b`), "commands an action"},
	{regexp.MustCompile(`(?i)\bstart by\b`), "prescribes a route"},
	{regexp.MustCompile(`(?i)\bthe (only|right|correct) (way|route|path)\b`), "prescribes a route"},
}

// MemoryNotes lists every memory note of a corpus: the corpus-level lines,
// then each view's, then each component's, in corpus order. Components are
// taken once by name, the way AllComponents dedupes a global reused in a view.
func MemoryNotes(c *sightmap.Corpus) []MemoryNote {
	if c == nil {
		return nil
	}
	var out []MemoryNote
	add := func(where string, lines []string) {
		for _, l := range lines {
			out = append(out, MemoryNote{Where: where, Text: strings.TrimSpace(l)})
		}
	}
	add("corpus", c.Memory)
	for _, v := range c.Views {
		add("view "+v.Name, v.Memory)
	}
	for _, comp := range c.AllComponents() {
		add("component "+comp.Name, comp.Memory)
	}
	return out
}

// LintMemory returns the memory notes of c that read as prescriptions, one
// finding per note, with every reason that fired joined in pattern order.
func LintMemory(c *sightmap.Corpus) []MemoryFinding {
	var out []MemoryFinding
	for _, n := range MemoryNotes(c) {
		if reason := lintNote(n.Text); reason != "" {
			out = append(out, MemoryFinding{MemoryNote: n, Reason: reason})
		}
	}
	return out
}

// lintNote returns the reasons a note is flagged, or "" when it reads as a
// description.
func lintNote(text string) string {
	var reasons []string
	seen := map[string]bool{}
	for _, p := range memoryPatterns {
		if p.re.MatchString(text) && !seen[p.reason] {
			seen[p.reason] = true
			reasons = append(reasons, p.reason)
		}
	}
	return strings.Join(reasons, ", ")
}
