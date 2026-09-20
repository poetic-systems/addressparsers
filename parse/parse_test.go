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
		// A private mailbox only comes out if privatemailbox is among the
		// vocabularies consulted; without its Detail claim the line reads
		// as a street named MAIN STREET PMB 4545.
		{"123 MAIN STREET PMB 4545\nHERNDON VA 22071", "123 MAIN ST PMB 4545"},
		{"PMB 234\n123 MAIN ST\nHERNDON VA 22071", "123 MAIN ST PMB 234"},
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

// The case #7 was opened for: a reading rated Exact that the data contradicts
// must not beat a reading rated Strong that the data agrees with. The old
// scale stepped Confidence itself and stopped short of Exact, so the two tied
// at Strong and the grammar-rating tie-break handed it to the reading the data
// had just spoken against. Uncapped, Exact contradicted is 3-1=2 and Strong
// agreed is 2+1=3.
func TestAgreementLiftsAPresentReadingPastAnAbsentOne(t *testing.T) {
	exactContradicted := parse.Score(claim.ConfidenceExact, []parse.Agreement{parse.Contradicts})
	strongAgreed := parse.Score(claim.ConfidenceStrong, []parse.Agreement{parse.Agrees})
	if exactContradicted >= strongAgreed {
		t.Errorf("Score(Exact, Contradicts) = %d, Score(Strong, Agrees) = %d; want the agreed reading ahead",
			exactContradicted, strongAgreed)
	}
}

// A single agreement moves a reading by one rung, which is enough to pass a
// reading one rung above it that the data spoke against — that inversion is
// the whole point of asking — but not enough to pass a reading two rungs
// above it that nothing was asked about at all. The data orders readings
// against each other; it does not manufacture a gap the grammar never gave.
func TestAgreementCannotCarryAReadingAcrossATwoRungGap(t *testing.T) {
	likelyAgreed := parse.Score(claim.ConfidenceLikely, []parse.Agreement{parse.Agrees})
	exactUntouched := parse.Score(claim.ConfidenceExact, nil)
	if likelyAgreed >= exactUntouched {
		t.Errorf("Score(Likely, Agrees) = %d, Score(Exact, nil) = %d; want the untouched exact reading ahead",
			likelyAgreed, exactUntouched)
	}
}

// Two readings of one street line that share a zipcity key ask it once and
// get back the same answer, so agreement shifts both of their scores by the
// same amount. Whatever gap the grammar put between two rungs, applying the
// same answers to both must leave the sign of that gap alone — that is the
// fact that lets choose retire the grammar-rating tie-break and rely on the
// score's own ordering instead.
func TestSharedAnswersPreserveTheGrammarsGap(t *testing.T) {
	rungs := []claim.Confidence{
		claim.ConfidenceWeak, claim.ConfidenceLikely, claim.ConfidenceStrong, claim.ConfidenceExact,
	}
	for _, answers := range [][]parse.Agreement{{parse.Agrees}, {parse.Contradicts}} {
		for _, lo := range rungs {
			for _, hi := range rungs {
				if parse.Rung(lo) >= parse.Rung(hi) {
					continue
				}
				loScore := parse.Score(lo, answers)
				hiScore := parse.Score(hi, answers)
				if loScore >= hiScore {
					t.Errorf("answers %v: Score(%d) = %d, Score(%d) = %d; want the higher rung still ahead",
						answers, lo, loScore, hi, hiScore)
				}
			}
		}
	}
}

// missingInZip, missingInCity, and unknown are not evidence — see agreement's
// doc comment for why a split street answer or a declined question must not
// read as either an agreement or a contradiction — so none of them may move
// a reading off the rung the grammar gave it.
func TestSplitsAndDeclinesAddNothing(t *testing.T) {
	rungs := []claim.Confidence{
		claim.ConfidenceWeak, claim.ConfidenceLikely, claim.ConfidenceStrong, claim.ConfidenceExact,
	}
	for _, ans := range []parse.Agreement{parse.MissingInZip, parse.MissingInCity, parse.Unknown} {
		for _, c := range rungs {
			if got, want := parse.Score(c, []parse.Agreement{ans}), parse.Rung(c); got != want {
				t.Errorf("Score(%d, %v) = %d, want %d (the bare rung)", c, ans, got, want)
			}
		}
	}
}

// choose builds one reference per call so that two readings asking the same
// key cost zipcity a single query. A query that keeps a counter is the only
// way to see that from outside the package.
func TestReferenceAsksEachKeyOnce(t *testing.T) {
	r := parse.Reference{}
	calls := 0
	query := func() (bool, error) {
		calls++
		return true, nil
	}

	first, err := parse.Check(r, "key", query)
	if err != nil {
		t.Fatalf("first check: %v", err)
	}
	second, err := parse.Check(r, "key", query)
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if calls != 1 {
		t.Errorf("query ran %d times, want 1", calls)
	}
	if first != second {
		t.Errorf("first = %v, second = %v, want the cached answer both times", first, second)
	}
}

// An error means zipcity was not actually consulted, so it must not be
// cached — caching it would silently turn "we don't know" into a permanent
// false for the rest of the call. check's own logic only stores the answer
// when err is nil; this pins that down from outside the package.
func TestReferenceDoesNotCacheAnError(t *testing.T) {
	r := parse.Reference{}
	calls := 0
	flaky := func() (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("zipcity unavailable")
		}
		return true, nil
	}

	if _, err := parse.Check(r, "key", flaky); err == nil {
		t.Fatal("want the first call's error")
	}
	if _, err := parse.Check(r, "key", flaky); err != nil {
		t.Fatalf("second check: %v", err)
	}
	if calls != 2 {
		t.Errorf("query ran %d times, want 2 — an erroring query must not be cached", calls)
	}
}

// A candidate with a ZIP, a city, and a two-letter region asks both
// CheckZipAndStreet and CheckCityStateAndStreet and folds the two answers —
// see foldStreetAnswers. W 9200 S is real in zipcity's West Jordan data on
// both shards (verified by probe), so the fold lands on Agrees.
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

// Two readings of one street line that differ only in which field holds a
// word ask the data the same question, so agreement lifts both by the same
// amount and the grammar's gap between them survives. Under the old capped
// scale they landed together at the ceiling and the winner was candidate
// order; this pins the end-to-end result now that the score carries the gap.
func TestAgreementCannotLiftAnAbsorbedReadingPastTheSplitOne(t *testing.T) {
	source := "1600 PENNSYLVANIA AVE NW\nWASHINGTON DC 20500"

	for _, opts := range []parse.Options{{}, {UseReferenceData: true}} {
		a, err := parse.New(opts).Parse(source)
		if err != nil {
			t.Fatalf("parsing with %+v: %v", opts, err)
		}
		if a.StreetName != "PENNSYLVANIA" || a.StreetSuffix != "AVE" || a.Postdirectional != "NW" {
			t.Errorf("with %+v: name=%q suffix=%q post=%q, want PENNSYLVANIA AVE NW split into its fields",
				opts, a.StreetName, a.StreetSuffix, a.Postdirectional)
		}
	}
}

// foldStreetAnswers is the whole fold: both true is agrees, both false is
// contradicts, and the two split combinations each name the side that was
// absent.
func TestFoldStreetAnswers(t *testing.T) {
	cases := []struct {
		zipPresent, cityPresent bool
		want                    parse.Agreement
	}{
		{true, true, parse.Agrees},
		{false, false, parse.Contradicts},
		{false, true, parse.MissingInZip},
		{true, false, parse.MissingInCity},
	}
	for _, tc := range cases {
		if got := parse.FoldStreetAnswers(tc.zipPresent, tc.cityPresent); got != tc.want {
			t.Errorf("FoldStreetAnswers(%v, %v) = %v, want %v",
				tc.zipPresent, tc.cityPresent, got, tc.want)
		}
	}
}

// A reading with a ZIP, a city, and a two-letter region asks both street
// questions rather than just one. 1600 Pennsylvania Ave NW is real in
// zipcity's Washington, DC data on both the ZIP shard and the city-state
// shard (verified by probe), so both questions come back true and the fold
// lands on Agrees.
func TestStreetAgreementAsksBothQuestionsWhenItCan(t *testing.T) {
	a := &address.Address{
		Type:            &ordinarystreet.OrdinaryStreetAddress{},
		StreetName:      "PENNSYLVANIA",
		StreetSuffix:    "AVE",
		Postdirectional: "NW",
		City:            "WASHINGTON",
		Region:          "DC",
		Postal:          "20500",
	}

	ans, ok := parse.StreetAgreement(a)
	if !ok {
		t.Fatal("want a question asked")
	}
	if ans != parse.Agrees {
		t.Errorf("StreetAgreement = %v, want Agrees", ans)
	}
}

// A split street answer takes no step, in either direction: the fold's
// missingInZip and missingInCity cases must leave a reading exactly where it
// would sit with reference data off.
//
// Wisconsin Ave NW is real in zipcity's Washington, DC city-state data, but
// not for ZIP 20500 (verified by probe): Wisconsin Avenue runs through upper
// northwest DC, nowhere near the White House ZIP. That is a genuine split —
// the street is real, the pairing with this particular ZIP is not — rather
// than a manufactured one.
func TestASplitStreetAnswerTakesNoStep(t *testing.T) {
	a := &address.Address{
		Type:            &ordinarystreet.OrdinaryStreetAddress{},
		StreetName:      "WISCONSIN",
		StreetSuffix:    "AVE",
		Postdirectional: "NW",
		City:            "WASHINGTON",
		Region:          "DC",
		Postal:          "20500",
	}
	if ans, ok := parse.StreetAgreement(a); !ok || ans != parse.MissingInZip {
		t.Fatalf("StreetAgreement = %v, %v, want MissingInZip, true", ans, ok)
	}

	source := "1600 WISCONSIN AVE NW\nWASHINGTON DC 20500"
	plain, err := parse.New(parse.Options{}).Parse(source)
	if err != nil {
		t.Fatalf("parsing without reference data: %v", err)
	}
	withData, err := parse.New(parse.Options{UseReferenceData: true}).Parse(source)
	if err != nil {
		t.Fatalf("parsing with reference data: %v", err)
	}
	if !plain.Equals(withData) {
		t.Errorf("a split street answer moved a reading:\n without = %+v\n with    = %+v", plain, withData)
	}
}
