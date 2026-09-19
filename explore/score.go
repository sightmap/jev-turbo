package explore

import (
	"fmt"
	"strconv"
	"strings"
)

// Score is what a run file says about the map it ran over: a cheap driver's
// success, in the terms a curator can act on.
type Score struct {
	Label            string  `json:"label"`
	Condition        string  `json:"condition,omitempty"` // "map", "no-map" or "tools", when the file records one
	Goals            int     `json:"goals"`
	Reached          int     `json:"reached"`
	StepsMedian      float64 `json:"steps_median"` // acted steps per reached goal
	Wasted           int     `json:"wasted"`
	NoEffect         int     `json:"no_effect"` // steps whose next observation showed nothing had happened
	Fallback         int     `json:"fallback"`
	LowConfidence    int     `json:"low_confidence"`
	CandidatesMedian float64 `json:"candidates_median"`
	AmbiguousMedian  float64 `json:"ambiguous_median"`   // candidates per step that share their description with another
	LowCoveragePages int     `json:"low_coverage_pages"` // distinct URLs where named controls are under half of the interactive ones
	Ms               int     `json:"ms"`

	// HasCoverage and HasMetrics say whether those rows could be measured at
	// all. A file run without a map carries no coverage, and a file written
	// before the metrics existed carries no counts. Either would otherwise
	// read as a real zero.
	HasCoverage bool `json:"has_coverage"`
	HasMetrics  bool `json:"has_metrics"`
}

// ScoreResult folds one suite result into a Score. Runs written before
// metrics existed are re-folded from their steps.
func ScoreResult(res *SuiteResult) Score {
	label := res.Suite
	if res.Condition != "" {
		label += "/" + res.Condition
	}
	if res.Picker != "" {
		label += "/" + res.Picker
	}
	s := Score{Label: label, Condition: res.Condition}
	var stepsPerGoal, cands, ambig []float64
	lowCov := map[string]bool{}
	for _, r := range res.Runs {
		if r.Run == nil {
			continue
		}
		m := r.Metrics
		if m.Steps > 0 {
			s.HasMetrics = true
		} else if len(r.Steps) > 0 {
			m = Metrics(r.Steps)
		}
		s.Goals++
		s.Ms += r.Ms
		if r.OK {
			s.Reached++
			stepsPerGoal = append(stepsPerGoal, float64(m.Steps))
		}
		s.Wasted += m.Wasted
		s.NoEffect += m.NoEffect
		s.Fallback += m.Fallback
		s.LowConfidence += m.LowConfidence
		for _, st := range r.Steps {
			if !IsAction(st) {
				continue
			}
			if st.Candidates > 0 {
				s.HasMetrics = true
				cands = append(cands, float64(st.Candidates))
				ambig = append(ambig, float64(st.Ambiguous))
			}
			if c := st.Coverage; c != nil && c.Interactive > 0 {
				s.HasCoverage = true
				if 2*(c.T1+c.T2) < c.Interactive {
					lowCov[st.URL] = true
				}
			}
		}
	}
	s.StepsMedian = median(stepsPerGoal)
	s.CandidatesMedian = median(cands)
	s.AmbiguousMedian = median(ambig)
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
	count := func(n int, measured bool) string {
		if !measured {
			return "-"
		}
		return fmt.Sprintf("%d", n)
	}
	add("reached", func(s Score) string { return fmt.Sprintf("%d/%d", s.Reached, s.Goals) })
	add("steps / goal", func(s Score) string { return num(s.StepsMedian) })
	add("wasted steps", func(s Score) string { return count(s.Wasted, s.HasMetrics) })
	add("no-effect steps", func(s Score) string { return count(s.NoEffect, s.HasMetrics) })
	// A fallback pick is a pick that carries no component, so it needs a map to fire.
	add("fallback picks", func(s Score) string { return count(s.Fallback, s.HasMetrics && s.Condition != "no-map") })
	add("low-confidence picks", func(s Score) string { return count(s.LowConfidence, s.HasMetrics) })
	add("candidates offered", func(s Score) string {
		if !s.HasMetrics {
			return "-"
		}
		return num(s.CandidatesMedian)
	})
	add("same-name candidates", func(s Score) string {
		if !s.HasMetrics {
			return "-"
		}
		return num(s.AmbiguousMedian)
	})
	add("low-coverage pages", func(s Score) string { return count(s.LowCoveragePages, s.HasCoverage) })
	add("seconds", func(s Score) string { return fmt.Sprintf("%.1f", float64(s.Ms)/1000) })
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if l := len([]rune(c)); l > widths[i] {
				widths[i] = l
			}
		}
	}
	var b strings.Builder
	for _, r := range rows {
		for i, c := range r {
			b.WriteString(c)
			b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(c))+2))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// num prints a median without inventing or dropping a digit: 4.5 stays 4.5, 13 stays 13.
func num(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
