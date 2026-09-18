package parse_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/claim"
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
