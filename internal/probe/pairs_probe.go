//go:build ignore

// pairs_probe.go parses the four pairs from #17 — each direction, with and
// without a ZIP Code, with and without the comma — through parse.New with
// reference data on, and prints the fields that decide each one. Run with:
//
//	go run internal/probe/pairs_probe.go
//
// It is the pass/fail set for #17's slices, kept here so each slice is
// measured on the same lines, and ignore-tagged out of every normal build
// and test run like measure.go beside it.
package main

import (
	"fmt"

	"github.com/poetic-systems/addressparsers/parse"
)

func main() {
	p := parse.New(parse.Options{UseReferenceData: true})
	for _, in := range []string{
		"123 MAIN ST WEST, PALM BEACH, FL 33480",
		"123 MAIN ST, WEST PALM BEACH, FL 33401",
		"123 MAIN ST WEST PALM BEACH FL 33480",
		"123 MAIN ST WEST PALM BEACH FL 33401",
		"123 MAIN ST WEST, PALM BEACH, FL",
		"123 MAIN ST, WEST PALM BEACH, FL",
		"123 MAIN ST WEST PALM BEACH FL",
		"3253 W 9200 S, WEST JORDAN, UT 84088",
		"3253 W 9200 SW JORDAN, UT 84088",
		"3253 W 9200 S WEST JORDAN UT 84088",
		"3253 W 9200 S, WEST JORDAN, UT",
		"3253 W 9200 S WEST JORDAN UT",
		// amadsen on #19: the same split with no ST/SAINT ambiguity in it,
		// so only the WEST directional is in question.
		"123 OCEAN BLVD WEST PALM BEACH, FL",
		"123 OCEAN BLVD WEST PALM BEACH FL",
		"123 OCEAN BLVD, WEST PALM BEACH, FL",
		"123 OCEAN BOULEVARD WEST PALM BEACH FL",
		"123 NORTH PARK ST, PAUL, MN 55102",
		"123 NORTH PARK, ST PAUL, MN 55102",
		"123 NORTH PARK ST PAUL MN 55102",
		"123 NORTH PARK ST PAUL MN",
		// amadsen on #19: the readings that reach NORTH PARK ST in PAUL.
		"123 NORTH PARK STREET, PAUL, MN",
		"123 NORTH PARK ST, PAUL, ID",
		"100 E ST NW, WASHINGTON, DC 20004",
		"100 EAST ST NW, WASHINGTON, DC 20004",
		"100 E ST, WASHINGTON, DC 20004",
		"100 EAST ST, WASHINGTON, DC 20004",
	} {
		a, err := p.Parse(in)
		if err != nil {
			fmt.Printf("%-42s ERR %v\n", in, err)
			continue
		}
		fmt.Printf("%-42s | num=%q pre=%q name=%q suf=%q post=%q | city=%q reg=%q zip=%q\n", in, a.PrimaryNumber, a.Predirectional, a.StreetName, a.StreetSuffix, a.Postdirectional, a.City, a.Region, a.Postal)
	}
}
