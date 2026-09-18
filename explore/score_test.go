package explore

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestScoreResult(t *testing.T) {
	res := &SuiteResult{Suite: "s", Picker: "jev", Condition: "map", Runs: []GoalResult{
		{Name: "a", Run: &Run{OK: true, Steps: []Step{
			{URL: "/x", Action: "clicked", Candidates: 10, Coverage: &CovStat{Interactive: 10, T1: 2}},
			{URL: "/y", Action: "clicked", Candidates: 30, Wasted: true, Coverage: &CovStat{Interactive: 10, T1: 8}},
			{Action: "done"}}}},
		{Name: "b", Run: &Run{OK: false, Steps: []Step{
			{URL: "/x", Action: "clicked", Candidates: 20, Fallback: true, Confidence: 0.3, Coverage: &CovStat{Interactive: 10, T1: 2}}}}},
	}}
	for i := range res.Runs {
		res.Runs[i].Metrics = Metrics(res.Runs[i].Steps)
	}
	s := ScoreResult(res)
	if s.Label != "s/map/jev" || s.Goals != 2 || s.Reached != 1 || s.StepsMedian != 2 {
		t.Fatalf("got %+v", s)
	}
	if s.Wasted != 1 || s.Fallback != 1 || s.LowConfidence != 1 || s.CandidatesMedian != 20 || s.LowCoveragePages != 1 {
		t.Fatalf("got %+v", s)
	}
	out := FormatScores([]Score{s, {Label: "s/no-map/jev", Goals: 2}})
	if !strings.Contains(out, "s/map/jev") || !strings.Contains(out, "s/no-map/jev") || !strings.Contains(out, "reached") {
		t.Fatalf("format:\n%s", out)
	}
}

func TestScoreOldRunFile(t *testing.T) {
	// Files written before metrics existed still score: counts fall back to zero, steps come from the steps list.
	b, err := os.ReadFile("../bench/results/saucedemo-jev.json")
	if err != nil {
		t.Skip("fixture missing")
	}
	var res SuiteResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	s := ScoreResult(&res)
	if s.Goals != 10 || s.Reached != 10 || s.StepsMedian == 0 {
		t.Fatalf("got %+v", s)
	}
}
