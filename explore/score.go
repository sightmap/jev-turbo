package explore

import (
	"fmt"
	"strings"
)

// Score is what a run file says about the map it ran over: a cheap driver's
// success, in the terms a curator can act on.
type Score struct {
	Label            string  `json:"label"`
	Goals            int     `json:"goals"`
	Reached          int     `json:"reached"`
	StepsMedian      float64 `json:"steps_median"` // acted steps per reached goal
	Wasted           int     `json:"wasted"`
	Fallback         int     `json:"fallback"`
	LowConfidence    int     `json:"low_confidence"`
	CandidatesMedian float64 `json:"candidates_median"`
	LowCoveragePages int     `json:"low_coverage_pages"` // distinct URLs where named controls are under half of the interactive ones
	Ms               int     `json:"ms"`
}

// ScoreResult folds one suite result into a Score. Runs written before
// metrics existed are re-folded from their steps.
func ScoreResult(res *SuiteResult) Score {
	label := res.Suite
	if res.Condition != "" {
		label += "/" + res.Condition
	}
	label += "/" + res.Picker
	s := Score{Label: label}
	var stepsPerGoal, cands []float64
	lowCov := map[string]bool{}
	for _, r := range res.Runs {
		if r.Run == nil {
			continue
		}
		m := r.Metrics
		if m.Steps == 0 && len(r.Steps) > 0 {
			m = Metrics(r.Steps)
		}
		s.Goals++
		s.Ms += r.Ms
		if r.OK {
			s.Reached++
			stepsPerGoal = append(stepsPerGoal, float64(m.Steps))
		}
		s.Wasted += m.Wasted
		s.Fallback += m.Fallback
		s.LowConfidence += m.LowConfidence
		for _, st := range r.Steps {
			if !IsAction(st) {
				continue
			}
			if st.Candidates > 0 {
				cands = append(cands, float64(st.Candidates))
			}
			if c := st.Coverage; c != nil && c.Interactive > 0 && 2*(c.T1+c.T2) < c.Interactive {
				lowCov[st.URL] = true
			}
		}
	}
	s.StepsMedian = median(stepsPerGoal)
	s.CandidatesMedian = median(cands)
	s.LowCoveragePages = len(lowCov)
	return s
}

// FormatScores prints one column per score, one row per measure.
func FormatScores(scores []Score) string {
	rows := [][]string{{""}}
	for _, s := range scores {
		rows[0] = append(rows[0], s.Label)
	}
	add := func(name string, f func(Score) string) {
		row := []string{name}
		for _, s := range scores {
			row = append(row, f(s))
		}
		rows = append(rows, row)
	}
	add("reached", func(s Score) string { return fmt.Sprintf("%d/%d", s.Reached, s.Goals) })
	add("steps / goal", func(s Score) string { return fmt.Sprintf("%.0f", s.StepsMedian) })
	add("wasted steps", func(s Score) string { return fmt.Sprintf("%d", s.Wasted) })
	add("fallback picks", func(s Score) string { return fmt.Sprintf("%d", s.Fallback) })
	add("low-confidence picks", func(s Score) string { return fmt.Sprintf("%d", s.LowConfidence) })
	add("candidates offered", func(s Score) string { return fmt.Sprintf("%.0f", s.CandidatesMedian) })
	add("low-coverage pages", func(s Score) string { return fmt.Sprintf("%d", s.LowCoveragePages) })
	add("seconds", func(s Score) string { return fmt.Sprintf("%.1f", float64(s.Ms)/1000) })
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	var b strings.Builder
	for _, r := range rows {
		for i, c := range r {
			fmt.Fprintf(&b, "%-*s  ", widths[i], c)
		}
		b.WriteString("\n")
	}
	return b.String()
}
