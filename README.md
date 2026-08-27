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

`go-projectusat` exposes two extension points through
`parser.AddressParsingOptions`:

```go
type AddressParsingOptions struct {
	Verifier     AddressVerifier // func(*address.Address) (*address.Address, error)
	CustomParser ParsingFunc     // Parse(source string) (*address.Address, error)
}
```

`CustomParser` replaces parsing wholesale — that is the seam
`parser/libpostalhttp` uses. `Verifier` judges a finished reading. Everything
in this repository targets one of the two.

## Packages

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

Early. `zipcityverify` is the first strategy and exercises the `Verifier` seam
end to end.

The seam that is *not* yet settled is the interesting one. `zipcity`'s value is
largest mid-parse — deciding whether `3253 W 9200 S` ends at `S` or `SW`, or
ranking competing `CandidateAddress` readings — and neither `Verifier` nor
`CustomParser` reaches there. Wiring reference data in as a `CustomParser` would
mean reimplementing the claim machinery outside `go-projectusat`, which is the
wrong trade. See
[go-projectusat#61](https://github.com/PortobelloAuth/go-projectusat/issues/61).

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
