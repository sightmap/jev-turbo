package explore

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// DistilledCheck is a finish check proposed from a run that reached its goal:
// the facts on the final page that the picker says prove the goal, composed
// as a DoneWhen, and how that check fares against the pages the run passed
// through on the way. A check that also holds on an earlier page would have
// stopped the run early, so the pages it fails on are the measure.
type DistilledCheck struct {
	Spec         DoneWhen `json:"spec"`
	URLFact      string   `json:"url_fact,omitempty"`
	URLProb      float64  `json:"url_prob,omitempty"`
	TextFact     string   `json:"text_fact,omitempty"`
	TextProb     float64  `json:"text_prob,omitempty"`
	HoldsOnFinal bool     `json:"holds_on_final"`
	FailsEarlier int      `json:"fails_on_earlier_pages"`
	EarlierPages int      `json:"earlier_pages"`
}

const distillURLInstructions = "The GOAL was just accomplished and the browser is on the final page. Which of these facts about the final URL proves the goal is done, and would not have been true on the pages before it? Answer none if the URL alone proves nothing."
const distillTextInstructions = "The GOAL was just accomplished and the browser is on the final page. Which of these texts, shown on the final page, proves the goal is done, and would not have shown on the pages before it? Prefer the text that names what the goal asked for. Answer none if no text proves it."

// DistillCheck proposes a finish check for a goal from the final page of a
// run that reached it, and scores it against the earlier pages.
func DistillCheck(ctx context.Context, picker Picker, goal string, earlier []*Page, final *Page) (*DistilledCheck, error) {
	j, ok := picker.(Judger)
	if !ok || final == nil {
		return nil, nil
	}
	out := &DistilledCheck{}
	state := fmt.Sprintf("GOAL: %s\nFINAL URL: %s\n", goal, shortURL(final.URL))

	// Only a URL part that no earlier page carried can prove the goal, so the
	// others are not offered, and a text no earlier page showed is offered
	// before one that some did.
	if facts := discriminatingURLFacts(final.URL, earlier); len(facts) > 0 {
		crit := Criteria{Options: append(factOptions("u", facts), Criterion{"none", "the URL alone proves nothing"})}
		pick, err := j.Judge(ctx, state, crit, distillURLInstructions)
		if err != nil {
			return nil, err
		}
		if i := factIndex(pick.Next, "u"); i >= 0 {
			out.URLFact, out.URLProb = facts[i], pick.Probs[pick.Next]
		}
	}
	if facts := orderByAbsence(textFacts(final, goal), earlier); len(facts) > 0 {
		crit := Criteria{Options: append(factOptions("t", facts), Criterion{"none", "no text on the page proves it"})}
		pick, err := j.Judge(ctx, state, crit, distillTextInstructions)
		if err != nil {
			return nil, err
		}
		if i := factIndex(pick.Next, "t"); i >= 0 {
			out.TextFact, out.TextProb = facts[i], pick.Probs[pick.Next]
		}
	}
	if out.URLFact != "" {
		out.Spec.All = append(out.Spec.All, DoneWhen{URLContains: out.URLFact})
	}
	if out.TextFact != "" {
		out.Spec.All = append(out.Spec.All, DoneWhen{TextContains: out.TextFact})
	}
	if len(out.Spec.All) == 0 {
		return out, nil
	}
	out.score(earlier, final)
	// A check that still holds on an earlier page would have stopped the run
	// there. Ask for a text that page does not show, up to twice; each such
	// fact rules that page out, so the loop can only tighten the check.
	used := map[string]bool{out.TextFact: true}
	for round := 0; round < 2 && out.HoldsOnFinal && out.FailsEarlier < out.EarlierPages; round++ {
		var still *Page
		for _, p := range earlier {
			if out.Spec.Check(p, nil) {
				still = p
				break
			}
		}
		var facts []string
		for _, f := range textFacts(final, goal) {
			if !used[f] && !pageHasText(still, f) {
				facts = append(facts, f)
			}
		}
		if len(facts) == 0 {
			break
		}
		crit := Criteria{Options: append(factOptions("t", facts), Criterion{"none", "none of these"})}
		q := state + fmt.Sprintf("The check so far also holds on an earlier page, %s. Which of these texts, shown on the final page and not on that one, tells the final page apart?\n", shortURL(still.URL))
		pick, err := j.Judge(ctx, q, crit, distillTextInstructions)
		if err != nil {
			return nil, err
		}
		i := factIndex(pick.Next, "t")
		if i < 0 {
			break
		}
		used[facts[i]] = true
		out.Spec.All = append(out.Spec.All, DoneWhen{TextContains: facts[i]})
		out.score(earlier, final)
	}
	return out, nil
}

// score records whether the check holds at the end and how many of the
// earlier pages it rules out.
func (d *DistilledCheck) score(earlier []*Page, final *Page) {
	d.HoldsOnFinal = d.Spec.Check(final, nil)
	d.EarlierPages = len(earlier)
	d.FailsEarlier = 0
	for _, p := range earlier {
		if !d.Spec.Check(p, nil) {
			d.FailsEarlier++
		}
	}
}

// urlFacts lists the path segments of a URL, each as a url_contains candidate.
func urlFacts(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	var facts []string
	for _, seg := range strings.Split(u.Path, "/") {
		if seg != "" {
			facts = append(facts, "/"+seg)
		}
	}
	if u.RawQuery != "" {
		for _, kv := range strings.Split(u.RawQuery, "&") {
			if k, _, ok := strings.Cut(kv, "="); ok && k != "" {
				facts = append(facts, k+"=")
			}
		}
	}
	return facts
}

// textFacts lists the visible texts of the final page that mention a goal
// word, shortest and most specific first, capped so the question stays small.
func textFacts(page *Page, goal string) []string {
	tokens := GoalTokens(goal)
	seen := map[string]bool{}
	var facts []string
	for _, n := range page.Nodes {
		if !n.Visible {
			continue
		}
		for _, s := range []string{n.Name, n.Text} {
			s = strings.Join(strings.Fields(s), " ")
			if len(s) < 3 || len(s) > 60 || seen[s] {
				continue
			}
			low := strings.ToLower(s)
			hits := 0
			for _, t := range tokens {
				if strings.Contains(low, t) {
					hits++
				}
			}
			if hits == 0 {
				continue
			}
			seen[s] = true
			facts = append(facts, s)
		}
	}
	sort.SliceStable(facts, func(i, j int) bool {
		hi, hj := factHits(facts[i], tokens), factHits(facts[j], tokens)
		if hi != hj {
			return hi > hj
		}
		return len(facts[i]) < len(facts[j])
	})
	if len(facts) > 20 {
		facts = facts[:20]
	}
	return facts
}

func factHits(s string, tokens []string) int {
	low := strings.ToLower(s)
	n := 0
	for _, t := range tokens {
		if strings.Contains(low, t) {
			n++
		}
	}
	return n
}

func factOptions(prefix string, facts []string) []Criterion {
	opts := make([]Criterion, 0, len(facts))
	for i, f := range facts {
		opts = append(opts, Criterion{fmt.Sprintf("%s%d", prefix, i), f})
	}
	return opts
}

func factIndex(key, prefix string) int {
	if !strings.HasPrefix(key, prefix) {
		return -1
	}
	var i int
	if _, err := fmt.Sscanf(key[len(prefix):], "%d", &i); err != nil {
		return -1
	}
	return i
}

// DescribeDistilled renders a proposal for a person: the check as JSON, then
// how it scored against the run's own pages.
func DescribeDistilled(d *DistilledCheck) string {
	if d == nil || len(d.Spec.All) == 0 {
		return "nothing on the final page proved the goal on its own"
	}
	var parts []string
	for _, c := range d.Spec.All {
		switch {
		case c.URLContains != "":
			parts = append(parts, fmt.Sprintf(`{"url_contains": %q}`, c.URLContains))
		case c.TextContains != "":
			parts = append(parts, fmt.Sprintf(`{"text_contains": %q}`, c.TextContains))
		}
	}
	verdict := "does not hold on the final page"
	if d.HoldsOnFinal {
		verdict = fmt.Sprintf("holds on the final page, fails on %d of %d earlier pages", d.FailsEarlier, d.EarlierPages)
	}
	return fmt.Sprintf(`{"all": [%s]}  (%s)`, strings.Join(parts, ", "), verdict)
}

// discriminatingURLFacts keeps the parts of the final URL that no earlier
// page's URL contained.
func discriminatingURLFacts(final string, earlier []*Page) []string {
	var out []string
	for _, f := range urlFacts(final) {
		seen := false
		for _, p := range earlier {
			if strings.Contains(p.URL, f) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, f)
		}
	}
	return out
}

// orderByAbsence lists the facts no earlier page showed first, keeping the
// order within each half.
func orderByAbsence(facts []string, earlier []*Page) []string {
	var absent, present []string
	for _, f := range facts {
		shown := false
		for _, p := range earlier {
			if pageHasText(p, f) {
				shown = true
				break
			}
		}
		if shown {
			present = append(present, f)
		} else {
			absent = append(absent, f)
		}
	}
	return append(absent, present...)
}
