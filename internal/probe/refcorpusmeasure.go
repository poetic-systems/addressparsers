//go:build ignore

// refcorpusmeasure.go is corpusmeasure.go with reference data ON. It exists
// because a change to how the reference-data questions are asked is invisible
// to corpusmeasure, which deliberately runs the pure-grammar parser. Run with:
//
//	go run internal/probe/refcorpusmeasure.go
package main

import (
	"fmt"

	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/parsertest"
	"github.com/poetic-systems/addressparsers/parse"
)

func main() {
	p := parse.New(parse.Options{UseReferenceData: true})

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
