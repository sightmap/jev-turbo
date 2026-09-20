package explore

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Judger is a picker that can answer a one-off multiple-choice question and
// report how sure it was. The effect judgment needs the probability: a verdict
// at 0.4 is a different fact from one at 0.95.
type Judger interface {
	Judge(ctx context.Context, state string, crit Criteria, instructions string) (Pick, error)
}

// The effect verdicts. "none" is the one that matters: the action did not do
// what it was for, whatever else moved on the page.
var effectOptions = []Criterion{
	{"navigated", "the page went somewhere else: a different URL or a different view"},
	{"opened", "something opened over the page: a dialog, a sheet, a menu, a suggestion list, a section that expanded"},
	{"value", "the field that was typed into now holds the typed value"},
	{"changed", "the page changed the way the action intended: a count went up, an item was added or removed, a state toggled, a result appeared"},
	{"error", "an error, a validation message, or a blocked notice appeared instead of the intended result"},
	{"none", "the intended thing did not happen; anything that did change is unrelated to the action (a rotating rail, a clock, an ad, a recommendation)"},
}

const effectInstructions = "The ACTION was just performed on a web page. From the listed differences between the page before and after, say what the action did. Prefer the most specific verdict that the evidence supports. Answer none when the action's own effect is missing, even if unrelated parts of the page changed."

// effectState describes an action and what the next observation showed, in
// the form the judge reads.
func effectState(before *acted, summary string, page *Page, descs map[string]int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ACTION: %s\n", summary)
	fmt.Fprintf(&b, "URL BEFORE: %s\nURL AFTER:  %s\n", shortURL(before.url), shortURL(page.URL))
	if before.filled != nil {
		val := ""
		for _, n := range page.Nodes {
			same := stableDesc(n) == stableDesc(before.filled) || (before.filled.Comp != "" && n.Comp == before.filled.Comp)
			if same {
				val = n.Value
				break
			}
		}
		fmt.Fprintf(&b, "FIELD TYPED INTO NOW HOLDS: %q\n", val)
	}
	appeared, gone := diffDescs(before.descs, descs, summary)
	writeList(&b, "CONTROLS THAT APPEARED", appeared)
	writeList(&b, "CONTROLS THAT DISAPPEARED", gone)
	var notices []string
	for _, n := range page.Nodes {
		if !n.Visible {
			continue
		}
		switch n.Role {
		case "alert", "status", "alertdialog":
			if t := strings.TrimSpace(firstNonEmpty(n.Name, n.Text)); t != "" {
				notices = append(notices, trunc(t, 120))
			}
		}
		if len(notices) >= 3 {
			break
		}
	}
	writeList(&b, "NOTICES ON THE PAGE", notices)
	return b.String()
}

// diffDescs lists the descriptions that appeared and disappeared between two
// candidate multisets, capped so the judge reads a summary and not a page
// dump. Entries that share words with the action come first, so a bag row's
// own controls are listed before the forty recommendations that rendered at
// the same time and would otherwise push them past the cap.
func diffDescs(before, after map[string]int, action string) (appeared, gone []string) {
	const max = 12
	for d, n := range after {
		if n > before[d] {
			appeared = append(appeared, d)
		}
	}
	for d, n := range before {
		if n > after[d] {
			gone = append(gone, d)
		}
	}
	words := actionWords(action)
	byRelevance := func(xs []string) {
		sort.SliceStable(xs, func(i, j int) bool {
			si, sj := overlap(xs[i], words), overlap(xs[j], words)
			if si != sj {
				return si > sj
			}
			return xs[i] < xs[j]
		})
	}
	sort.Strings(appeared)
	sort.Strings(gone)
	byRelevance(appeared)
	byRelevance(gone)
	if len(appeared) > max {
		appeared = append(appeared[:max], fmt.Sprintf("… and %d more", len(appeared)-max))
	}
	if len(gone) > max {
		gone = append(gone[:max], fmt.Sprintf("… and %d more", len(gone)-max))
	}
	return appeared, gone
}

// actionWords are the words of an action summary worth matching: three
// letters or more, lowercased, without the verb and the punctuation.
func actionWords(action string) map[string]bool {
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(action), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		switch w {
		case "clicked", "filled", "toggled", "selected", "pressed", "with", "the", "and", "button", "link", "label":
			continue
		}
		if len(w) >= 3 {
			words[w] = true
		}
	}
	return words
}

func overlap(desc string, words map[string]bool) int {
	n := 0
	for _, w := range strings.FieldsFunc(strings.ToLower(desc), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		if words[w] {
			n++
		}
	}
	return n
}

func writeList(b *strings.Builder, head string, items []string) {
	fmt.Fprintf(b, "%s (%d):\n", head, len(items))
	if len(items) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, it := range items {
		fmt.Fprintf(b, "  - %s\n", trunc(it, 160))
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// judgeEffect asks the picker what the last action did. It returns the
// verdict and the probability the picker gave it; a picker that cannot judge
// returns "" so the caller keeps the rule's verdict.
func judgeEffect(ctx context.Context, picker Picker, before *acted, summary string, page *Page, descs map[string]int) (string, float64, int, string, error) {
	j, ok := picker.(Judger)
	if !ok {
		return "", 0, 0, "", nil
	}
	crit := Criteria{Options: effectOptions}
	state := effectState(before, summary, page, descs)
	pick, err := j.Judge(ctx, state, crit, effectInstructions)
	if err != nil {
		return "", 0, 0, "", err
	}
	// The evidence goes into the run file without the ACTION line, which the
	// step already carries, and capped so a page dump cannot bloat it.
	evidence := state
	if i := strings.Index(evidence, "\n"); i >= 0 {
		evidence = evidence[i+1:]
	}
	if len(evidence) > 1200 {
		evidence = evidence[:1200] + "…"
	}
	return pick.Next, pick.Probs[pick.Next], pick.Ms, evidence, nil
}
