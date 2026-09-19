package parse_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/PortobelloAuth/go-projectusat/pkg/address"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/claim"
	"github.com/PortobelloAuth/go-projectusat/pkg/addresstypes/ordinarystreet"
	"github.com/PortobelloAuth/go-projectusat/pkg/addresstypes/pobox"
	"github.com/poetic-systems/addressparsers/parse"
)

// Every address below is either published in the Project US@ specification or
// invented. None is a real person's address.

// The parser must be usable through go-projectusat's own seam. This is the
// whole point of the package, so it is asserted at compile time rather than
// left to a caller to discover.
var _ parser.ParsingFunc = (*parse.Parser)(nil)

func TestItReadsTheSpecialAddressFormats(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		streetName string
		primary    string
		city       string
		region     string
		postal     string
	}{
		{
			name:       "post office box",
			source:     "PO BOX 11890\nWEST JORDAN UT 84088",
			streetName: "PO BOX", primary: "11890",
			city: "WEST JORDAN", region: "UT", postal: "84088",
		},
		{
			name:       "rural route",
			source:     "RR 4 BOX 125\nWEST JORDAN UT 84088",
			streetName: "RR 4", primary: "BOX 125",
			city: "WEST JORDAN", region: "UT", postal: "84088",
		},
		{
			name:       "military",
			source:     "PSC 3 BOX 4120\nAPO AE 09021",
			streetName: "PSC 3", primary: "BOX 4120",
			city: "APO", region: "AE", postal: "09021",
		},
	}

	p := parse.New(parse.Options{})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := p.Parse(c.source)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if a.StreetName != c.streetName || a.PrimaryNumber != c.primary {
				t.Errorf("street line = %q %q, want %q %q",
					a.StreetName, a.PrimaryNumber, c.streetName, c.primary)
			}
			if a.City != c.city || a.Region != c.region || a.Postal != c.postal {
				t.Errorf("last line = %q %q %q, want %q %q %q",
					a.City, a.Region, a.Postal, c.city, c.region, c.postal)
			}
			if a.Type == nil {
				t.Error("the winning candidate must carry the address type that read it")
			}
		})
	}
}

// The ordinary street line is assembled from the shared vocabularies rather
// than recognized, so what matters is that every element comes out in its
// place: the formatted line is the whole decomposition in one string.
func TestItReadsTheOrdinaryStreetLine(t *testing.T) {
	cases := []struct {
		source string
		street string
	}{
		{"123 MAIN ST\nWEST JORDAN UT 84088", "123 MAIN ST"},
		{"123 N MAIN ST APT 4\nWEST JORDAN UT 84088", "123 N MAIN ST APT 4"},
		{"1600 PENNSYLVANIA AVE NW\nWASHINGTON DC 20500", "1600 PENNSYLVANIA AVE NW"},
		{"GENERAL DELIVERY\nFAIRHAVEN MA 02719", "GENERAL DELIVERY"},
	}

	p := parse.New(parse.Options{})
	for _, c := range cases {
		t.Run(c.street, func(t *testing.T) {
			a, err := p.Parse(c.source)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if got := a.FormatStreetLine(); got != c.street {
				t.Errorf("street line = %q, want %q", got, c.street)
			}
		})
	}
}

func TestEmptyInputHasNoReading(t *testing.T) {
	p := parse.New(parse.Options{})

	for _, source := range []string{"", "   ", "\n"} {
		if _, err := p.Parse(source); !errors.Is(err, parse.ErrNoReading) {
			t.Errorf("source %q: want ErrNoReading, got %v", source, err)
		}
	}
}

// Reference data demotes a reading; it never discards one. A ZIP and city that
// the data does not pair still parse, because zipcity's gaps are real and a
// patient at an address the Census missed must not be dropped.
func TestReferenceDataDemotesButDoesNotReject(t *testing.T) {
	source := "PO BOX 11890\nNOT A REAL MUNICIPALITY UT 84088"

	withData := parse.New(parse.Options{UseReferenceData: true})
	a, err := withData.Parse(source)
	if err != nil {
		t.Fatalf("a contradicted reading must still parse: %v", err)
	}
	if a.StreetName != "PO BOX" || a.PrimaryNumber != "11890" {
		t.Errorf("street line = %q %q, want %q %q",
			a.StreetName, a.PrimaryNumber, "PO BOX", "11890")
	}
}

// Turning reference data on must not change a reading the data agrees with.
func TestAgreementDoesNotChangeTheChosenReading(t *testing.T) {
	source := "PO BOX 11890\nWEST JORDAN UT 84088"

	plain, err := parse.New(parse.Options{}).Parse(source)
	if err != nil {
		t.Fatalf("parsing without reference data: %v", err)
	}
	withData, err := parse.New(parse.Options{UseReferenceData: true}).Parse(source)
	if err != nil {
		t.Fatalf("parsing with reference data: %v", err)
	}
	if !plain.Equals(withData) {
		t.Errorf("reference data changed an agreed reading:\n without = %+v\n with    = %+v", plain, withData)
	}
}

func TestTheErrorCarriesNoPartOfTheInput(t *testing.T) {
	p := parse.New(parse.Options{})

	// No last line, so no address type can read it.
	source := "123 INVENTED LANE\nNOT A REAL MUNICIPALITY"
	_, err := p.Parse(source)
	if err == nil {
		t.Fatal("want a rejection")
	}
	for _, part := range []string{"INVENTED", "NOT A REAL MUNICIPALITY"} {
		if strings.Contains(err.Error(), part) {
			t.Errorf("the error text leaks %q: %v", part, err)
		}
	}
}

// Reference data may raise a reading, but it may not make one certain.
//
// ConfidenceExact means a vocabulary had exactly one reading of the tokens.
// zipcity's filters answer true by collision about one time in a hundred, so
// letting agreement reach Exact would let a collision manufacture the strongest
// claim the scale can express. See go-projectusat#71.
func TestAgreementCannotManufactureCertainty(t *testing.T) {
	for _, tc := range []struct {
		name string
		from claim.Confidence
		want claim.Confidence
	}{
		{"weak rises to likely", claim.ConfidenceWeak, claim.ConfidenceLikely},
		{"likely rises to strong", claim.ConfidenceLikely, claim.ConfidenceStrong},
		{"strong stops below exact", claim.ConfidenceStrong, claim.ConfidenceStrong},
		{"exact is kept, not lowered", claim.ConfidenceExact, claim.ConfidenceExact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parse.Strengthen(tc.from); got != tc.want {
				t.Errorf("Strengthen(%d) = %d, want %d", tc.from, got, tc.want)
			}
		})
	}
}

// A step up and a step down are not required to cancel, but neither may run
// away: repeated agreement must not climb past the cap, and repeated
// contradiction must not fall below the floor.
func TestTheStepsAreBounded(t *testing.T) {
	up := claim.ConfidenceWeak
	down := claim.ConfidenceExact
	for range 10 {
		up = parse.Strengthen(up)
		down = parse.Weaken(down)
	}
	if up != claim.ConfidenceStrong {
		t.Errorf("repeated agreement reached %d, want %d", up, claim.ConfidenceStrong)
	}
	if down != claim.ConfidenceWeak {
		t.Errorf("repeated contradiction reached %d, want %d", down, claim.ConfidenceWeak)
	}
}

// A candidate with a five digit ZIP asks CheckZipAndStreet, never
// CheckCityStateAndStreet — see go-projectusat#71's 2026-09-18T15:06Z comment,
// "step 2" this change implements. W 9200 S is real in zipcity's West Jordan
// data (verified by probe), so a true here can only have come from the ZIP
// path.
func TestStreetAgreementAsksZipAndStreetWhenAZipIsPresent(t *testing.T) {
	a := &address.Address{
		Type:            &ordinarystreet.OrdinaryStreetAddress{},
		Predirectional:  "W",
		StreetName:      "9200",
		Postdirectional: "S",
		City:            "WEST JORDAN",
		Region:          "UT",
		Postal:          "84088",
	}

	ans, ok := parse.StreetAgreement(a)
	if !ok {
		t.Fatal("want a question asked")
	}
	if ans != parse.Agrees {
		t.Errorf("StreetAgreement = %v, want Agrees", ans)
	}
}

// A candidate with no ZIP falls back to CheckCityStateAndStreet rather than
// asking nothing — the choice CONTRIBUTING calls out as worth deciding
// explicitly. Pleasant Hill Rd is real in zipcity's Pleasant Hill, CA
// city-street data (verified by probe); a made up street at the same city and
// state is absent, and the two together show the city-state path is what
// answered, not a decline that happened to look like one.
func TestStreetAgreementFallsBackToCityAndStateWithNoZip(t *testing.T) {
	real := &address.Address{
		Type:         &ordinarystreet.OrdinaryStreetAddress{},
		StreetName:   "PLEASANT HILL",
		StreetSuffix: "RD",
		City:         "PLEASANT HILL",
		Region:       "CA",
	}
	if ans, ok := parse.StreetAgreement(real); !ok || ans != parse.Agrees {
		t.Errorf("StreetAgreement(real street) = %v, %v, want Agrees, true", ans, ok)
	}

	absent := &address.Address{
		Type:         &ordinarystreet.OrdinaryStreetAddress{},
		StreetName:   "ZQXVBORK",
		StreetSuffix: "LN",
		City:         "PLEASANT HILL",
		Region:       "CA",
	}
	if ans, ok := parse.StreetAgreement(absent); !ok || ans != parse.Contradicts {
		t.Errorf("StreetAgreement(absent street) = %v, %v, want Contradicts, true", ans, ok)
	}
}

// The closed forms carry a fixed pseudo street name — "PO BOX" is not a
// street zipcity was ever asked about — so StreetAgreement must decline
// rather than manufacture a contradiction on every one of them.
func TestStreetAgreementDeclinesForClosedForms(t *testing.T) {
	a := &address.Address{
		Type:          &pobox.POBoxAddress{},
		StreetName:    "PO BOX",
		PrimaryNumber: "11890",
		City:          "WEST JORDAN",
		Region:        "UT",
		Postal:        "84088",
	}

	if _, ok := parse.StreetAgreement(a); ok {
		t.Error("want the question declined for a closed form")
	}
}

// A reading with no street name at all — an ordinarystreet candidate the
// vocabularies left empty — has nothing to ask zipcity about.
func TestStreetAgreementDeclinesWithNoStreetName(t *testing.T) {
	a := &address.Address{
		Type:   &ordinarystreet.OrdinaryStreetAddress{},
		City:   "WEST JORDAN",
		Region: "UT",
		Postal: "84088",
	}

	if _, ok := parse.StreetAgreement(a); ok {
		t.Error("want the question declined with no street name")
	}
}

// A street the data contradicts steps the reading down but the address still
// parses — the same guarantee TestReferenceDataDemotesButDoesNotReject makes
// for zip+city, now exercised for the street question. ZQXVBORK is invented
// and West Jordan, UT 84088 is real, so only the street half of agreement can
// be contradicting here.
//
// The street carries no suffix on purpose. ordinarystreet always offers a
// second reading beside "name ZQXVBORK" that absorbs a would-be suffix into
// the name instead — see candidate.go's streetConfidence — and that second
// reading renders to the identical zipcity query string as the first, so a
// zip+city agreement (West Jordan pairs with 84088) can promote it to tie
// the properly split reading before the street contradiction demotes both by
// the same step. A bare name has no suffix to absorb, so there is only one
// reading and no tie for the demotion to land ambiguously on.
func TestStreetContradictionDemotesButStillParses(t *testing.T) {
	source := "123 ZQXVBORK\nWEST JORDAN UT 84088"

	withData, err := parse.New(parse.Options{UseReferenceData: true}).Parse(source)
	if err != nil {
		t.Fatalf("a street-contradicted reading must still parse: %v", err)
	}
	if withData.StreetName != "ZQXVBORK" || withData.PrimaryNumber != "123" {
		t.Errorf("street = %q %q, want %q %q", withData.PrimaryNumber, withData.StreetName, "123", "ZQXVBORK")
	}
}

// Turning on the street question must not change a reading with only one
// candidate to choose among. BROADWAY has no suffix and no directional, so
// ordinarystreet offers exactly one reading of it — nothing for reference
// data to promote past another candidate — and it is real at this ZIP and
// city both, so this exercises two agreements (zip+city and zip+street) on
// the one reading without either one having a competing reading to distort.
func TestStreetAgreementDoesNotChangeAnUnambiguousReading(t *testing.T) {
	source := "BROADWAY\nNEW YORK NY 10012"

	plain, err := parse.New(parse.Options{}).Parse(source)
	if err != nil {
		t.Fatalf("parsing without reference data: %v", err)
	}
	withData, err := parse.New(parse.Options{UseReferenceData: true}).Parse(source)
	if err != nil {
		t.Fatalf("parsing with reference data: %v", err)
	}
	if !plain.Equals(withData) {
		t.Errorf("reference data changed an agreed reading:\n without = %+v\n with    = %+v", plain, withData)
	}
}
