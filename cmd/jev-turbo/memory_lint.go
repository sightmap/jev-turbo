package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sightmap/jev-turbo/explore"
	"github.com/sightmap/sightmap/go/sightmap"
)

// errMemoryFlagged ends memory-lint with exit code 1 once the findings are
// printed; there is nothing more to say on stderr.
var errMemoryFlagged = fmt.Errorf("memory notes flagged")

/* ---------------- memory-lint ---------------- */

// runMemoryLint reads the corpus at DIR and prints every memory note that
// reads as a prescription rather than a description, one line per note with
// the reason, then a count. A note that tells the picker where to go was
// written for one goal and steers every other goal the same way.
func runMemoryLint(args []string) error {
	fs := flag.NewFlagSet("memory-lint", flag.ContinueOnError)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("memory-lint needs a corpus directory")
	}
	dir := pos[0]
	c, err := sightmap.Load(dir)
	if err != nil {
		return fmt.Errorf("load corpus %s: %w", dir, err)
	}
	notes := explore.MemoryNotes(c)
	findings := explore.LintMemory(c)
	for _, f := range findings {
		fmt.Fprintf(os.Stdout, "%s: %s: \"%s\"\n", f.Where, f.Reason, firstRunes(f.Text, 80))
	}
	fmt.Fprintf(os.Stdout, "%d notes, %d flagged\n", len(notes), len(findings))
	if len(findings) > 0 {
		return errMemoryFlagged
	}
	return nil
}

// firstRunes keeps the first n characters of s, on one line.
func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	for i, c := range r {
		if c == '\n' || c == '\r' || c == '\t' {
			r[i] = ' '
		}
	}
	return string(r)
}
