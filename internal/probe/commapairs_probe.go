//go:build ignore

// commapairs_probe.go normalizes the same address written with and without
// its commas, and reports the pairs that come out as two different strings.
// Run with:
//
//	go run internal/probe/commapairs_probe.go
//
// Two hashes for one patient is the whole of #20, so the measurement it wants
// is not which reading is right but whether the two readings agree. It runs
// with reference data on, since an unmarked last line is exactly the case
// only the data can close, and is ignore-tagged out of every normal build and
// test run like the probes beside it.
package main

import (
	"fmt"
	"strings"

	goprojectusat "github.com/PortobelloAuth/go-projectusat"
	"github.com/poetic-systems/addressparsers/parse"
)

func main() {
	opts := []goprojectusat.USAtNormalizeOption{
		goprojectusat.WithCustomAddressParser(parse.New(parse.Options{UseReferenceData: true})),
		goprojectusat.WithContentNormalization(),
	}

	split := 0
	for _, marked := range []string{
		"123 OCEAN BLVD, WEST PALM BEACH, FL",
		"123 OCEAN BOULEVARD, WEST PALM BEACH, FL",
		"123 MAIN ST, WEST PALM BEACH, FL",
		"123 MAIN STREET, WEST PALM BEACH, FL",
		"123 NORTH PARK ST, ST PAUL, MN",
		"3253 W 9200 S, WEST JORDAN, UT",
		"1600 PENNSYLVANIA AVE NW, WASHINGTON, DC 20500",
		"100 E ST NW, WASHINGTON, DC 20004",
	} {
		unmarked := strings.ReplaceAll(marked, ",", "")
		a, errA := goprojectusat.Normalize(marked, opts...)
		b, errB := goprojectusat.Normalize(unmarked, opts...)
		agree := a == b && errA == nil && errB == nil
		if !agree {
			split++
		}
		fmt.Printf("%-6v %-48s %q\n", agree, marked, a)
		fmt.Printf("%-6s %-48s %q\n", "", unmarked, b)
	}
	fmt.Printf("\n%d of the pairs above hash differently\n", split)
}
