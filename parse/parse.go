// Package parse is an address parser built from go-projectusat's vocabularies,
// consulting zipcity where the grammar alone cannot settle a reading.
//
// go-projectusat implements the standard: it tokenizes, every vocabulary says
// what tokens mean, and every address type offers its reading of the whole. All
// of that is exported and composable, so this package assembles those parts
// rather than reimplementing them. What it adds is the step the standard cannot
// specify — choosing among readings that are all grammatically valid — because
// that choice is data dependent and the data lives outside the standard.
//
// The resulting Parser satisfies parser.ParsingFunc, so it drops into
// go-projectusat through AddressParsingOptions.CustomParser.
package parse

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/PortobelloAuth/go-projectusat/pkg/address"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/claim"
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser/token"
	"github.com/PortobelloAuth/go-projectusat/pkg/addresstypes/military"
	"github.com/PortobelloAuth/go-projectusat/pkg/addresstypes/pobox"
	"github.com/PortobelloAuth/go-projectusat/pkg/addresstypes/ruralroute"
	"github.com/PortobelloAuth/go-projectusat/pkg/country"
	"github.com/PortobelloAuth/go-projectusat/pkg/directionals"
	"github.com/PortobelloAuth/go-projectusat/pkg/highways"
	"github.com/PortobelloAuth/go-projectusat/pkg/lastline"
	"github.com/PortobelloAuth/go-projectusat/pkg/postalcode"
	"github.com/PortobelloAuth/go-projectusat/pkg/region"
	"github.com/PortobelloAuth/go-projectusat/pkg/secondaryunit"
	"github.com/PortobelloAuth/go-projectusat/pkg/streetsuffixes"
	"github.com/poetic-systems/zipcity"
)

// vocabularies are the Claims functions consulted for every address, in no
// significant order: each says what it recognizes and none of them rank against
// the others. Listing them once here is what keeps "which vocabularies run" a
// single fact rather than a sequence of calls to keep in step.
var vocabularies = []func([]token.Token) []claim.Claim{
	country.Claims,
	region.Claims,
	postalcode.Claims,
	directionals.Claims,
	streetsuffixes.Claims,
	secondaryunit.Claims,
	highways.Claims,
	pobox.Claims,
	military.Claims,
	ruralroute.Claims,
}

// addressTypes are the Candidates functions, each offering that type's reading
// of the whole address or offering none.
//
// puertorico is absent because it has no Candidates function yet, and there is
// no ordinary street type at all. Both are tracked upstream — go-projectusat
// #60 and #56 — and until they land this parser reads special formats and
// returns ErrNoReading for an ordinary street address. That is a gap in
// coverage, not a defect here: a missing address type produces no candidate,
// which is exactly how a type declines to read an address it does not fit.
var addressTypes = []func([]token.Token, []claim.Claim, lastline.LineClaim) []*address.CandidateAddress{
	pobox.Candidates,
	military.Candidates,
	ruralroute.Candidates,
}

// ErrNoReading reports that no address type offered a reading of the source.
//
// It carries no part of the input. Addresses handled here may be protected
// health information, and an error is the value most likely to reach a log, a
// crash report, or a bug tracker.
var ErrNoReading = fmt.Errorf("parse: no address type offered a reading")

// zip5 matches the five digit form zipcity requires, narrowing a ZIP+4.
var zip5 = regexp.MustCompile(`^(\d{5})(?:-?\d{4})?$`)

// Options controls how much the parser leans on reference data.
type Options struct {
	// UseReferenceData lets zipcity break ties among candidates.
	//
	// Off by default. With it off the parser is a pure reading of the standard
	// and its answers depend only on the input, which is what makes its
	// behaviour reproducible from the specification alone.
	UseReferenceData bool
}

// Parser reads a free text address.
type Parser struct {
	opts Options
}

// New returns a Parser applying opts.
func New(opts Options) *Parser { return &Parser{opts: opts} }

// Parse reads source into a structured address, returning ErrNoReading when no
// address type recognizes it.
//
// The stages are go-projectusat's, in the order that library defines: tokenize,
// let every vocabulary claim what it recognizes, read the last line from those
// claims, then ask each address type for its reading of the whole. Only the
// final choice among readings belongs to this package.
func (p *Parser) Parse(source string) (*address.Address, error) {
	tokens := token.Tokenize(source)
	if len(tokens) == 0 {
		return nil, ErrNoReading
	}

	var claims []claim.Claim
	for _, vocabulary := range vocabularies {
		claims = append(claims, vocabulary(tokens)...)
	}

	// Every last line reading is carried forward rather than the best one
	// chosen here. A weaker last line can still be the one the winning address
	// type reads, and picking early would hide that reading from the choice.
	var candidates []*address.CandidateAddress
	for _, line := range lastline.LineClaims(tokens, claims) {
		for _, addressType := range addressTypes {
			candidates = append(candidates, addressType(tokens, claims, line)...)
		}
	}
	if len(candidates) == 0 {
		return nil, ErrNoReading
	}

	best := p.choose(candidates)
	if best == nil {
		return nil, ErrNoReading
	}
	return best.Address, nil
}

// choose ranks candidates and returns the best, or nil when none survive.
//
// Confidence decides, and coverage breaks ties: between two equally confident
// readings the one stranding fewer tokens is the better account of the input.
// Reference data enters only as a demotion, never as a promotion — see
// demoted.
func (p *Parser) choose(candidates []*address.CandidateAddress) *address.CandidateAddress {
	type ranked struct {
		candidate *address.CandidateAddress
		score     claim.Confidence
	}

	scored := make([]ranked, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Address == nil {
			continue
		}
		score := c.Confidence
		if p.opts.UseReferenceData && p.demoted(c.Address) {
			score = weaken(score)
		}
		scored = append(scored, ranked{candidate: c, score: score})
	}
	if len(scored) == 0 {
		return nil
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return len(scored[i].candidate.Leftover) < len(scored[j].candidate.Leftover)
	})
	return scored[0].candidate
}

// demoted reports whether the reference data positively contradicts a reading.
//
// Only a false answer from zipcity is actionable. Its filters are bloom
// filters, so a false is definitive — the key was never added — while a true
// merely means the key may be present and some trues are collisions. A parser
// that promoted on true would be ranking on noise, and would produce confident
// wrong answers rather than uncertain right ones. So this function can lower a
// candidate and has no way to raise one.
//
// A contradiction is still not proof the address is wrong. zipcity is built
// from Census TIGER files with documented gaps, so a real address the Census
// missed lands here too. That is why the answer is one step of confidence
// rather than rejection, and why UseReferenceData is off by default.
func (p *Parser) demoted(a *address.Address) bool {
	m := zip5.FindStringSubmatch(a.Postal)
	if m == nil || a.City == "" {
		// Nothing to ask about. An address the data cannot be consulted for is
		// not thereby a worse reading.
		return false
	}

	present, err := zipcity.CheckZipAndCity(m[1], a.City)
	if err != nil {
		// zipcity declined the inputs rather than answering about them. That is
		// a question this parser could not ask, not an answer it received.
		return false
	}
	return !present
}

// weaken lowers a confidence by one step on the shared scale, stopping at the
// bottom. A demoted reading is worse than it claimed but is still a reading.
func weaken(c claim.Confidence) claim.Confidence {
	switch {
	case c > claim.ConfidenceStrong:
		return claim.ConfidenceStrong
	case c > claim.ConfidenceLikely:
		return claim.ConfidenceLikely
	case c > claim.ConfidenceWeak:
		return claim.ConfidenceWeak
	default:
		return c
	}
}
