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
			{URL: "/x", Action: "clicked", Candidates: 10, Ambiguous: 2, Effect: "navigated", Coverage: &CovStat{Interactive: 10, T1: 2}},
			{URL: "/y", Action: "clicked", Candidates: 30, Ambiguous: 8, Wasted: true, Effect: "none", Coverage: &CovStat{Interactive: 10, T1: 8}},
			{Action: "done"}}}},
		{Name: "b", Run: &Run{OK: false, Steps: []Step{
			{URL: "/x", Action: "clicked", Candidates: 20, Ambiguous: 4, Fallback: true, Confidence: 0.3, Effect: "none", Coverage: &CovStat{Interactive: 10, T1: 2}}}}},
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
	if s.NoEffect != 2 || s.AmbiguousMedian != 4 {
		t.Fatalf("got %+v", s)
	}
	out := FormatScores([]Score{s, {Label: "s/no-map/jev", Goals: 2}})
	if !strings.Contains(out, "s/map/jev") || !strings.Contains(out, "s/no-map/jev") || !strings.Contains(out, "reached") {
		t.Fatalf("format:\n%s", out)
	}
	if !cellRow(out, "no-effect steps", "2") || !cellRow(out, "same-name candidates", "4") {
		t.Fatalf("the no-effect and same-name rows should carry the counts:\n%s", out)
	}
	// The second column carries no metrics, so both rows read - there.
	if !cellRow(out, "no-effect steps", "-") || !cellRow(out, "same-name candidates", "-") {
		t.Fatalf("rows without metrics should read -:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "wasted steps") && !strings.HasPrefix(lines[i+1], "no-effect steps") {
			t.Fatalf("no-effect steps should follow wasted steps:\n%s", out)
		}
	}
}

func TestScoreOldRunFile(t *testing.T) {
	// Files written before metrics existed still score: steps come from the steps
	// list, but the counts were never recorded, so they are not measurable.
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
	if s.HasMetrics {
		t.Fatalf("a file with no metrics block should not claim metrics: %+v", s)
	}
	out := FormatScores([]Score{s})
	for _, row := range []string{"wasted steps", "no-effect steps", "fallback picks", "low-confidence picks", "candidates offered", "same-name candidates"} {
		if !cellRow(out, row, "-") {
			t.Fatalf("%s should read - on a file with no metrics:\n%s", row, out)
		}
	}
}

func TestScoreNoMapIsNotZero(t *testing.T) {
	// With no map there is no corpus to cover and no component to fall back from.
	// Both rows would otherwise read as a real zero.
	res := &SuiteResult{Suite: "s", Picker: "jev", Condition: "no-map", Runs: []GoalResult{
		{Name: "a", Run: &Run{OK: true, Steps: []Step{
			{URL: "/x", Action: "clicked", Candidates: 4, Coverage: &CovStat{}},
			{URL: "/y", Action: "clicked", Candidates: 6, Wasted: true, Coverage: &CovStat{}},
			{Action: "done"}}}},
	}}
	for i := range res.Runs {
		res.Runs[i].Metrics = Metrics(res.Runs[i].Steps)
	}
	s := ScoreResult(res)
	if s.HasCoverage {
		t.Fatalf("a run with no corpus has no coverage: %+v", s)
	}
	if !s.HasMetrics {
		t.Fatalf("the counts were recorded, only coverage was not: %+v", s)
	}
	out := FormatScores([]Score{s})
	if !cellRow(out, "low-coverage pages", "-") {
		t.Fatalf("low-coverage pages should read -:\n%s", out)
	}
	if !cellRow(out, "fallback picks", "-") {
		t.Fatalf("fallback picks should read -:\n%s", out)
	}
	if !cellRow(out, "wasted steps", "1") {
		t.Fatalf("wasted steps is still measurable:\n%s", out)
	}
}

func TestFormatScoresMedianHalf(t *testing.T) {
	// A median of 4.5 is a real half step. Rounding it to 4 would hide a step.
	out := FormatScores([]Score{{Label: "s/map/jev", Goals: 2, Reached: 2, StepsMedian: 4.5, CandidatesMedian: 13, HasMetrics: true}})
	if !cellRow(out, "steps / goal", "4.5") {
		t.Fatalf("steps / goal should read 4.5:\n%s", out)
	}
	if !cellRow(out, "candidates offered", "13") {
		t.Fatalf("candidates offered should read 13:\n%s", out)
	}
}

// cellRow reports whether the row named name has want as one of its cells.
func cellRow(out, name, want string) bool {
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		for _, f := range strings.Fields(strings.TrimPrefix(line, name)) {
			if f == want {
				return true
			}
		}
	}
	return false
}
