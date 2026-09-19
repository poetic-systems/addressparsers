package parse

// Strengthen and Weaken are the confidence steps, exposed for tests.
//
// Parse returns only an address, so neither step is observable through the
// public API — the confidence that decided between readings is discarded on the
// way out. That is the gap go-projectusat#61 asks about. Until it is settled,
// testing the arithmetic directly is the only way to hold the cap in place.
var (
	Strengthen = strengthen
	Weaken     = weaken
)

// Agreement and the functions that produce one are exposed for the same
// reason: which questions agreement asks, and which single street question a
// reading qualifies for, is discarded the same way the confidence steps are.
// Testing zipCityAgreement and streetAgreement directly is the only way to
// pin down that a candidate with no ZIP asks CheckCityStateAndStreet rather
// than silently asking nothing, without depending on which reading choose
// happens to rank first.
type Agreement = agreement

const (
	Unknown       = unknown
	Agrees        = agrees
	Contradicts   = contradicts
	MissingInZip  = missingInZip
	MissingInCity = missingInCity
)

var (
	ZipCityAgreement  = zipCityAgreement
	StreetAgreement   = streetAgreement
	StreetForQuery    = streetForQuery
	FoldStreetAnswers = foldStreetAnswers
)
