//go:build ignore

// measure.go probes parse.New with and without reference data across a table
// of addresses, and reports which readings changed and why. Run with:
//
//	go run internal/probe/measure.go
//
// It exists to produce the measurement table for the addressparsers PR that
// added the street question to agreement (go-projectusat#71 "step 2"), per
// CONTRIBUTING's "measure, don't assert." Kept under internal/ rather than
// deleted, since it is a reusable tool for the next reference-data question
// (CONTRIBUTING's "measure the hard cases" applies again whenever one is
// added) and is ignore-tagged out of every normal build and test run.
package main

import (
	"fmt"

	"github.com/PortobelloAuth/go-projectusat/pkg/address"
	"github.com/poetic-systems/addressparsers/parse"
	"github.com/poetic-systems/zipcity"
)

type tc struct {
	name   string
	source string
}

func main() {
	cases := []tc{
		{"spec: 1600 Pennsylvania Ave NW (public, non-residential)", "1600 PENNSYLVANIA AVE NW\nWASHINGTON DC 20500"},
		{"spec p.26: 1234 Ave Ashford, Puerto Rico", "1234 AVE ASHFORD\nSAN JUAN PR 00907"},
		{"go-projectusat#82: North Decatur Rd, predirectional-vs-name (no premise)", "NORTH DECATUR RD\nDECATUR GA 30033"},
		{"street definitely absent (invented, real zip+city)", "123 ZQXVBORK\nWEST JORDAN UT 84088"},
		{"mainland known street", "PLEASANT HILL RD\nPLEASANT HILL CA 94523"},
		{"mainland known street, no zip (city+state fallback)", "PLEASANT HILL RD\nPLEASANT HILL CA"},
		{"unambiguous single-token street (control)", "BROADWAY\nNEW YORK NY 10012"},
	}

	for _, c := range cases {
		fmt.Printf("=== %s ===\nsource: %q\n", c.name, c.source)

		plain, errPlain := parse.New(parse.Options{}).Parse(c.source)
		withData, errData := parse.New(parse.Options{UseReferenceData: true}).Parse(c.source)

		fmt.Printf("  plain:     %s (err=%v)\n", describe(plain), errPlain)
		fmt.Printf("  reference: %s (err=%v)\n", describe(withData), errData)
		if errPlain == nil && errData == nil && !plain.Equals(withData) {
			fmt.Printf("  CHANGED (fields differ; see comment below for which cases this is expected)\n")
		}
		fmt.Println()
	}

	// Direct zipcity probes for the discriminating pairs, showing the raw
	// evidence the table above is built from.
	fmt.Println("=== direct zipcity probes ===")
	probeZipStreet("00907", "AVE ASHFORD")      // spec form: false
	probeZipStreet("00907", "AVENIDA ASHFORD")  // Pub28/data form: true
	probeZipStreet("00907", "AVENUE ASHFORD")   // go-projectusat#95's output: false
	probeZipStreet("30033", "N DECATUR RD")     // true: predirectional reading
	probeZipStreet("30033", "NORTH DECATUR RD") // false: name-absorbed reading
	probeZipStreet("84088", "ZQXVBORK")         // false: invented street
	probeCityStateStreet("PLEASANT HILL", "CA", "PLEASANT HILL RD")
}

func probeZipStreet(zip, street string) {
	present, err := zipcity.CheckZipAndStreet(zip, street)
	fmt.Printf("  CheckZipAndStreet(%q, %q) = %v (err=%v)\n", zip, street, present, err)
}

func probeCityStateStreet(city, state, street string) {
	present, err := zipcity.CheckCityStateAndStreet(city, state, street)
	fmt.Printf("  CheckCityStateAndStreet(%q, %q, %q) = %v (err=%v)\n", city, state, street, present, err)
}

func describe(a *address.Address) string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q [predir=%q name=%q suffix=%q postdir=%q] / %s",
		a.FormatStreetLine(), a.Predirectional, a.StreetName, a.StreetSuffix, a.Postdirectional, a.FormatLastLine())
}
