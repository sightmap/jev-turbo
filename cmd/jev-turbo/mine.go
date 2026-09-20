package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sightmap/jev-turbo/explore"
)

/* ---------------- mine ---------------- */

// runMine reads run files and prints the step sequences that recur across
// their successful runs as a sightkick tools.yaml draft. A journey that
// every run repeats by hand is a tool waiting to be written.
func runMine(args []string) error {
	fs := flag.NewFlagSet("mine", flag.ContinueOnError)
	minSupport := fs.Int("min-support", 2, "Runs a sequence must appear in")
	max := fs.Int("max", 12, "Tools to print (0 = all)")
	files, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("mine needs one or more run files")
	}
	var runs []*explore.Run
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var res explore.SuiteResult
		if err := json.Unmarshal(b, &res); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		for i := range res.Runs {
			if res.Runs[i].Run != nil {
				runs = append(runs, res.Runs[i].Run)
			}
		}
	}
	tools := explore.Mine(runs, *minSupport)
	if *max > 0 && len(tools) > *max {
		tools = tools[:*max]
	}
	if len(tools) == 0 {
		fmt.Fprintln(os.Stdout, "no sequence recurs across the successful runs")
		return nil
	}
	fmt.Fprint(os.Stdout, explore.DescribeMined(tools))
	return nil
}
