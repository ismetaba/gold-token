package httputil

import "testing"

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", "user.name+tag@example.com", "x@y.io"}
	invalid := []string{"", "no-at", "a@b", "a@@b.com", "a b@c.com", "@b.com"}
	for _, e := range valid {
		if !ValidateEmail(e) {
			t.Errorf("expected %q valid", e)
		}
	}
	for _, e := range invalid {
		if ValidateEmail(e) {
			t.Errorf("expected %q invalid", e)
		}
	}
}

func TestValidateName(t *testing.T) {
	if !ValidateName("Alice", 2, 20) {
		t.Error("Alice should be valid")
	}
	if ValidateName("A", 2, 20) {
		t.Error("too short should be invalid")
	}
	if ValidateName("with\x01control", 2, 50) {
		t.Error("control characters should be invalid")
	}
}

func TestValidateCountryCode(t *testing.T) {
	for _, c := range []string{"US", "TR", "GB"} {
		if !ValidateCountryCode(c) {
			t.Errorf("%q should be valid", c)
		}
	}
	for _, c := range []string{"us", "USA", "1A", "U", ""} {
		if ValidateCountryCode(c) {
			t.Errorf("%q should be invalid", c)
		}
	}
}

func TestValidateEthAddress(t *testing.T) {
	if !ValidateEthAddress("0x52908400098527886E0F7030069857D2E4169EE7") {
		t.Error("valid checksum address rejected")
	}
	if ValidateEthAddress("not-an-address") {
		t.Error("garbage accepted")
	}
}

func TestValidateDateOfBirth(t *testing.T) {
	if _, ok := ValidateDateOfBirth("1990-01-01", 18, 120); !ok {
		t.Error("adult DOB should be valid")
	}
	if _, ok := ValidateDateOfBirth("not-a-date", 18, 120); ok {
		t.Error("garbage date should be invalid")
	}
	if _, ok := ValidateDateOfBirth("2020-01-01", 18, 120); ok {
		t.Error("a child should fail the minimum-age check")
	}
}
