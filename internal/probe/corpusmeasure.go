//go:build ignore

// corpusmeasure.go scores parse.New (no reference data) against the shared
// parsertest corpus (go-projectusat/pkg/address/parser/parsertest) and
// prints pass/settled/total per suite, plus every failing row's Source and
// Input, so two runs (before/after a go-projectusat bump) can be diffed row
// by row rather than compared by total alone. Run with:
//
//	go run internal/probe/corpusmeasure.go
package main

import (
	"fmt"

	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/parsertest"
	"github.com/poetic-systems/addressparsers/parse"
)

func main() {
	p := parse.New(parse.Options{})

	suites := []struct {
		name  string
		cases []parsertest.Case
	}{
		{"SpecCases", parsertest.SpecCases},
		{"HistoricalCases", parsertest.HistoricalCases},
		{"OursCases", parsertest.OursCases},
	}

	for _, s := range suites {
		results := parsertest.Run(p, s.cases)
		pass, settled, total := parsertest.CountPass(results)
		fmt.Printf("%s: %d/%d/%d (pass/settled/total)\n", s.name, pass, settled, total)
		for _, r := range results {
			if r.Settled() && !r.Pass() {
				fmt.Printf("  FAIL [%s] %q\n    want: %q\n    got:  %q (err=%v)\n", r.Case.Source, r.Case.Input, r.Case.Want, r.Got, r.Err)
			}
		}
	}
}
