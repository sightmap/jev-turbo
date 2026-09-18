package explore

import "testing"

func TestMetrics(t *testing.T) {
	steps := []Step{
		{N: 1, Action: "clicked [A]", Candidates: 10, Options: 12, Confidence: 0.9},
		{N: 2, Action: "clicked [B]", Candidates: 20, Options: 22, Confidence: 0.4, Fallback: true},
		{N: 3, Action: "stale element, skipped", Candidates: 30, Options: 32, Confidence: 0.7, Wasted: true},
		{N: 4, Action: "done"},
	}
	m := Metrics(steps)
	if m.Steps != 3 || m.Wasted != 1 || m.Fallback != 1 || m.LowConfidence != 1 {
		t.Fatalf("got %+v", m)
	}
	if m.CandidatesMedian != 20 || m.OptionsMedian != 22 {
		t.Fatalf("medians %v %v", m.CandidatesMedian, m.OptionsMedian)
	}
	if median(nil) != 0 || median([]float64{1, 4}) != 2.5 {
		t.Fatal("median")
	}
}

func TestSummarizeAggregatesMetrics(t *testing.T) {
	runs := []GoalResult{
		{Name: "a", Run: &Run{OK: true, Steps: []Step{{Action: "x", Candidates: 4, Wasted: true}, {Action: "done"}}}},
		{Name: "b", Run: &Run{OK: false, Steps: []Step{{Action: "y", Candidates: 8, Fallback: true, Confidence: 0.2}, {Action: "done(judged)", Candidates: 100}}}},
	}
	for i := range runs {
		runs[i].Run.Metrics = Metrics(runs[i].Run.Steps)
	}
	s := Summarize(runs)
	if s.Wasted != 1 || s.Fallback != 1 || s.LowConfidence != 1 || s.CandidatesMedian != 6 {
		t.Fatalf("got %+v", s)
	}
}

func TestIsAction(t *testing.T) {
	if IsAction(Step{Action: "done"}) || IsAction(Step{Action: "done(judged)"}) || IsAction(Step{Action: "no-candidates"}) {
		t.Fatal("closing steps should not count as actions")
	}
	if !IsAction(Step{Action: "clicked [A]"}) {
		t.Fatal("an ordinary step should count as an action")
	}
}
