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
