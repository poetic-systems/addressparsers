# addressparsers

Pluggable address parsing and verification strategies for
[go-projectusat](https://github.com/PortobelloAuth/go-projectusat).

## Why this is a separate repository

`go-projectusat` implements Project US@ and USPS Publication 28: it knows the
grammar of an address and the vocabulary the standard defines. What it
deliberately does not carry is *data about the world* — which cities exist in a
ZIP code, which street names are real, which of two readings a human would
recognize.

That separation is the point. The standard changes slowly and by committee.
Reference data changes with every Census release. Bundling them would tie a
specification implementation to a dataset's release cadence, and would make
`go-projectusat` heavier for every caller who does not need disambiguation.

So the strategies live here, the data lives in
[zipcity](https://github.com/poetic-systems/zipcity), and `go-projectusat`
stays a clean implementation of the standard with seams for both.

## The seams

`go-projectusat` implements the standard and exports every part of it:
`token.Tokenize`, a `Claims` function on each vocabulary (`country`, `region`,
`postalcode`, `directionals`, `streetsuffixes`, `secondaryunit`, `highways`),
`lastline.LineClaims`, a `Candidates` function on each address type, and
`claim.Compare` / `Overlaps` / `Gaps` for reasoning about the results.

So a parser here is not a reimplementation. It composes those parts and adds
the one step the standard cannot specify: choosing among readings that are all
grammatically valid. That choice is data dependent, which is why it lives with
the data rather than with the specification.

The finished parser plugs back in through `parser.AddressParsingOptions`:

```go
type AddressParsingOptions struct {
	Verifier     AddressVerifier // func(*address.Address) (*address.Address, error)
	CustomParser ParsingFunc     // Parse(source string) (*address.Address, error)
}
```

## Packages

### `parse`

The parser. It runs go-projectusat's pipeline — tokenize, let every vocabulary
claim what it recognizes, read the last line from those claims, ask each address
type for its reading of the whole — and then chooses among the readings,
consulting `zipcity` when the grammar alone cannot settle one.

```go
import (
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser"
	"github.com/poetic-systems/addressparsers/parse"
)

p := parser.New(parser.AddressParsingOptions{
	CustomParser: parse.New(parse.Options{UseReferenceData: true}),
})
```

`UseReferenceData` is off by default. With it off the parser is a pure reading
of the standard, and its answers depend only on its input — which is what makes
its behaviour reproducible from the specification alone.

**Reference data can only demote a candidate, never promote one.** See the
bloom filter note below; a parser that ranked readings upward on a possible
false positive would produce confident wrong answers instead of uncertain right
ones.

**Coverage today:** PO box, rural route, and military addresses parse. An
ordinary street address returns `ErrNoReading`, because the address type that
would read it is decided but unbuilt
([go-projectusat#56](https://github.com/PortobelloAuth/go-projectusat/issues/56)),
and Puerto Rico addresses are waiting on
[#60](https://github.com/PortobelloAuth/go-projectusat/issues/60). A missing
address type produces no candidate, which is exactly how a type declines to
read an address it does not fit.

### `zipcityverify`

An `AddressVerifier` that rejects readings the `zipcity` reference data says
cannot exist.

```go
import (
	"github.com/PortobelloAuth/go-projectusat/pkg/address/parser"
	"github.com/poetic-systems/addressparsers/zipcityverify"
)

p := parser.New(parser.AddressParsingOptions{
	Verifier: zipcityverify.Verifier(zipcityverify.Options{
		RejectUnknownZipCity: true,
	}),
})
```

**It only ever rejects.** `zipcity` answers from bloom filters, which have
one-sided error: a false is definitive, a true only means "possibly present".
There is no confirmation to be had from this data, and the package offers none
so that no caller can mistake a true for one.

**Absent is not invalid.** A definitive false means a pairing is missing from a
dataset built from Census TIGER files, and `zipcity` documents real gaps in
those files. A patient at a real address the Census missed is precisely the
person a matching system must not drop, so every check is opt-in and the zero
`Options` value verifies nothing.

## Status

Early, and the interesting work is the adjudicator in `parse`.

Assembling the pipeline is arrangement of parts that already exist. Choosing
among candidates is not, and it is the step go-projectusat leaves open
([#61](https://github.com/PortobelloAuth/go-projectusat/issues/61)) precisely
because the standard cannot specify it. Ranking currently goes on confidence,
then on coverage, then on a one-step demotion from contradicting reference
data. Cases like `3253 W 9200 S` — does the street name end at `S` or `SW`? —
need the adjudicator to consult the data mid-reading rather than after it, and
that is the next real piece.

## Handling addresses

Addresses processed here may be protected health information. Treat every value
flowing through this library as sensitive:

- Test fixtures use invented data, or public reference data such as a city
  paired with its ZIP code. No fixture names a residence.
- Do not log or print address values.
- Errors must not embed any part of an address. `zipcityverify.ErrAbsent`
  carries none, and there is a test that keeps it that way — an error is the
  value most likely to reach a log, a crash report, or a bug tracker.

## Copyright and License

Copyright 2026 Poetic Systems

Unless otherwise specified all code and related artifacts in this repository are
made available under the Apache 2 License. See the [license](./LICENSE.md) for
details.
