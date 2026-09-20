package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/sightmap/jev-turbo/explore"
)

/* ---------------- same-names ---------------- */

// runSameNames observes the live page once and reports the controls that
// share a role and an accessible name. They are the picks a raw tree cannot
// make and a screen reader cannot announce apart; the report says which
// entry or component tells each one apart, and the name that would say so.
func runSameNames(args []string) error {
	fs := flag.NewFlagSet("same-names", flag.ContinueOnError)
	lf := addLiveFlags(fs)
	top := fs.Int("top", 10, "Show the N largest groups (0 = all)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := lf.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	corpus, err := lf.corpus()
	if err != nil {
		return err
	}
	drv := explore.NewCDPDriver(conn, corpus)
	drv.Settle(ctx, "")
	page, err := drv.Observe(ctx)
	if err != nil {
		return err
	}
	groups := explore.SameNames(page, nil)
	shown := groups
	if *top > 0 && len(shown) > *top {
		shown = shown[:*top]
	}
	fmt.Fprintf(os.Stdout, "%s\n%s", page.URL, explore.DescribeSameNames(shown))
	if len(shown) < len(groups) {
		fmt.Fprintf(os.Stdout, "(%d smaller groups not shown; --top 0 shows all)\n", len(groups)-len(shown))
	}
	return nil
}
