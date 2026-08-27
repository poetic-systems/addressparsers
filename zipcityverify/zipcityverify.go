// Package zipcityverify rejects address readings that the zipcity reference
// data says cannot exist.
//
// It implements parser.AddressVerifier from go-projectusat, so it plugs into
// AddressParsingOptions without that library knowing this one exists.
//
// # What a bloom filter can and cannot tell you
//
// zipcity answers from bloom filters, and a bloom filter has one-sided error.
// A false is definitive: the key was never added, so the pairing does not
// appear in the reference data. A true is not: it means the key may be
// present, and some fraction of trues are collisions for keys that were never
// added at all.
//
// So this package only ever rejects. There is no Confirm, no boost, and no
// "verified" flag, because the data cannot support one. Reading a true as
// confirmation is the mistake this package is shaped to prevent.
//
// # Absent is not the same as invalid
//
// A definitive false still does not mean the address is wrong. It means the
// pairing is absent from a dataset built from Census TIGER files, and zipcity
// documents real gaps in that dataset: several TIGER archives serve an error
// page under a 200 status, and there are no address range files for the
// Marshall Islands or the Northern Marianas. New construction is missing for
// the same reason any snapshot is.
//
// A patient living at a real address that the Census missed is exactly the
// person a matching system must not drop. Rejection is therefore opt-in per
// check and off by default, and the address itself is never the thing that
// gets discarded — only a reading of it.
package zipcityverify

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/PortobelloAuth/go-projectusat/pkg/address"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser"
	"github.com/poetic-systems/zipcity"
)

// ErrAbsent reports that a reading names a pairing the reference data does not
// contain.
//
// It carries no part of the address. Addresses handled by this library are
// protected health information, and an error is the value most likely to reach
// a log, a crash report, or a bug tracker — the three places PHI must never
// arrive. Callers that need to know which address failed already hold it.
var ErrAbsent = errors.New("zipcityverify: the reference data has no such pairing")

// zip5 matches the five digit form zipcity requires. A ZIP+4 is narrowed to
// its first five digits before the check; anything else is not checked at all.
var zip5 = regexp.MustCompile(`^(\d{5})(?:-?\d{4})?$`)

// Options selects which pairings are checked. The zero value checks nothing
// and passes every address, which is the safe default for a library whose
// false answers can be right about the data and wrong about the world.
type Options struct {
	// RejectUnknownZipCity rejects a reading whose ZIP and city are absent
	// from the reference data.
	//
	// This is the check to reach for first. A city name is chosen from a small
	// set for any given ZIP, so a wrong reading is likely to be absent rather
	// than to collide with something present.
	RejectUnknownZipCity bool

	// RejectUnknownZipStreet rejects a reading whose ZIP and street name are
	// absent from the reference data.
	//
	// Weaker than it looks, and slower: it loads one of the sharded street
	// filters. Street coverage is where TIGER's gaps concentrate, so a false
	// here is more often a hole in the data than a wrong reading.
	RejectUnknownZipStreet bool
}

// Verifier returns a parser.AddressVerifier applying opts.
//
// The verifier returns the address unchanged when every enabled check passes
// or cannot be run, and ErrAbsent when one of them is definitively false. It
// never modifies the address.
func Verifier(opts Options) parser.AddressVerifier {
	return func(a *address.Address) (*address.Address, error) {
		if a == nil {
			return nil, fmt.Errorf("zipcityverify: %w", errors.New("nil address"))
		}

		zip, ok := zip5FromPostal(a.Postal)
		if !ok {
			// No usable ZIP means no check to run. An address with no postal
			// code is not thereby a bad address.
			return a, nil
		}

		if opts.RejectUnknownZipCity && a.City != "" {
			present, err := zipcity.CheckZipAndCity(zip, a.City)
			if err != nil {
				// zipcity rejected the inputs rather than the pairing. That is
				// a question this verifier cannot answer, not a failed answer.
				return a, nil
			}
			if !present {
				return nil, ErrAbsent
			}
		}

		if opts.RejectUnknownZipStreet && a.StreetName != "" {
			present, err := zipcity.CheckZipAndStreet(zip, a.StreetName)
			if err != nil {
				return a, nil
			}
			if !present {
				return nil, ErrAbsent
			}
		}

		return a, nil
	}
}

// zip5FromPostal narrows a postal code to the five digit form zipcity takes,
// reporting whether it could. A Canadian postal code cannot, and gets no
// check rather than a failed one.
func zip5FromPostal(postal string) (string, bool) {
	m := zip5.FindStringSubmatch(postal)
	if m == nil {
		return "", false
	}
	return m[1], true
}
