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

**Reference data moves a candidate one step, in either direction, and never
settles it.** `zipcity`'s filters are built at a 0.01 false positive rate, so a
`false` is definitive while a `true` is roughly 100:1 evidence rather than a
confirmation. Both are worth acting on. Neither is proof: agreement stops below
`ConfidenceExact`, because a reading that is certain is a claim about the
grammar that reference data is in no position to make.

**Coverage today:** ordinary street, PO box, rural route, general delivery,
and military addresses parse. Puerto Rico addresses are waiting on
[go-projectusat#60](https://github.com/PortobelloAuth/go-projectusat/issues/60).
A missing address type produces no candidate, which is exactly how a type
declines to read an address it does not fit.

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
A true is still evidence, but a verifier is the wrong place to spend it — it
returns an address or an error, so the only thing it could do with a true is
call the address real. Weighing a true instead of trusting it is `parse`'s job.

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
then on coverage, with a one-step adjustment either way from reference data. Cases like `3253 W 9200 S` — does the street name end at `S` or `SW`? —
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
