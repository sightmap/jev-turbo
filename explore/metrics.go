package explore

import "sort"

// LowConfidence is the pick probability under which a step counts as unsure.
const LowConfidence = 0.6

// RunMetrics summarises a run's steps in the terms the map score uses.
type RunMetrics struct {
	Steps            int     `json:"steps"`
	Wasted           int     `json:"wasted"`
	Fallback         int     `json:"fallback"`
	LowConfidence    int     `json:"low_confidence"`
	CandidatesMedian float64 `json:"candidates_median"`
	OptionsMedian    float64 `json:"options_median"`
}

// Metrics folds the acted steps of a run. The final "done" step is not an action.
func Metrics(steps []Step) RunMetrics {
	var m RunMetrics
	var cands, opts []float64
	for _, s := range steps {
		if s.Action == "done" || s.Action == "done(judged)" || s.Action == "no-candidates" {
			continue
		}
		m.Steps++
		if s.Wasted {
			m.Wasted++
		}
		if s.Fallback {
			m.Fallback++
		}
		if s.Confidence > 0 && s.Confidence < LowConfidence {
			m.LowConfidence++
		}
		cands = append(cands, float64(s.Candidates))
		opts = append(opts, float64(s.Options))
	}
	m.CandidatesMedian = median(cands)
	m.OptionsMedian = median(opts)
	return m
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	n := len(ys)
	if n%2 == 1 {
		return ys[n/2]
	}
	return (ys[n/2-1] + ys[n/2]) / 2
}
