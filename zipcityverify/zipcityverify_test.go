package zipcityverify_test

import (
	"errors"
	"testing"

	"github.com/PortobelloAuth/go-projectusat/pkg/address"
	"github.com/poetic-systems/addressparsers/zipcityverify"
)

// The ZIP and city used here are the ones zipcity's own README works through,
// and a city paired with its ZIP is public reference data rather than anyone's
// address. No fixture in this package names a residence.
const (
	realZip  = "84088"
	realCity = "WEST JORDAN"
)

func TestAPairingInTheDataIsNotRejected(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{RejectUnknownZipCity: true})

	a := &address.Address{City: realCity, Region: "UT", Postal: realZip}
	got, err := verify(a)
	if err != nil {
		t.Fatalf("verifying a pairing that is in the data: %v", err)
	}
	if got != a {
		t.Errorf("the verifier must return the address unchanged")
	}
}

func TestAPairingAbsentFromTheDataIsRejected(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{RejectUnknownZipCity: true})

	// An invented city name, so that the miss is a property of the reading and
	// not of a gap in the reference data.
	a := &address.Address{City: "NOT A REAL MUNICIPALITY ANYWHERE", Region: "UT", Postal: realZip}
	if _, err := verify(a); !errors.Is(err, zipcityverify.ErrAbsent) {
		t.Fatalf("want ErrAbsent, got %v", err)
	}
}

func TestTheZeroOptionsCheckNothing(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{})

	a := &address.Address{City: "NOT A REAL MUNICIPALITY ANYWHERE", Region: "UT", Postal: realZip}
	if _, err := verify(a); err != nil {
		t.Fatalf("the zero Options must check nothing, got %v", err)
	}
}

func TestAnAddressWithNoUsableZipIsNotChecked(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{
		RejectUnknownZipCity:   true,
		RejectUnknownZipStreet: true,
	})

	for _, postal := range []string{"", "K1A 0B1", "8408", "not a postal code"} {
		a := &address.Address{City: "NOT A REAL MUNICIPALITY ANYWHERE", Postal: postal}
		if _, err := verify(a); err != nil {
			t.Errorf("postal %q: want no check, got %v", postal, err)
		}
	}
}

func TestAZipPlusFourIsNarrowedToItsFirstFiveDigits(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{RejectUnknownZipCity: true})

	for _, postal := range []string{realZip + "-1234", realZip + "1234"} {
		a := &address.Address{City: realCity, Postal: postal}
		if _, err := verify(a); err != nil {
			t.Errorf("postal %q: want the ZIP+4 narrowed and accepted, got %v", postal, err)
		}
	}
}

func TestTheErrorCarriesNoPartOfTheAddress(t *testing.T) {
	verify := zipcityverify.Verifier(zipcityverify.Options{RejectUnknownZipCity: true})

	secret := "NOT A REAL MUNICIPALITY ANYWHERE"
	a := &address.Address{City: secret, Region: "UT", Postal: realZip}
	_, err := verify(a)
	if err == nil {
		t.Fatal("want a rejection")
	}
	for _, part := range []string{secret, realZip, "UT"} {
		if contains(err.Error(), part) {
			t.Errorf("the error text leaks %q: %v", part, err)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
