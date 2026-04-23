package httputil

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	gethcommon "github.com/ethereum/go-ethereum/common"
)

// ValidateEthAddress checks if the given string is a valid Ethereum hex address.
func ValidateEthAddress(addr string) bool {
	return gethcommon.IsHexAddress(addr)
}

// ValidateName checks that a name field is between minLen and maxLen runes
// and contains only printable characters (no control chars).
func ValidateName(s string, minLen, maxLen int) bool {
	n := utf8.RuneCountInString(s)
	if n < minLen || n > maxLen {
		return false
	}
	for _, r := range s {
		if r < 0x20 { // control characters
			return false
		}
	}
	return true
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail performs basic email format validation.
func ValidateEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// ValidateDateOfBirth checks format and that age is within [minAge, maxAge].
func ValidateDateOfBirth(dob string, minAge, maxAge int) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", dob)
	if err != nil {
		return time.Time{}, false
	}
	now := time.Now()
	age := now.Year() - t.Year()
	if now.YearDay() < t.YearDay() {
		age--
	}
	if age < minAge || age > maxAge {
		return time.Time{}, false
	}
	return t, true
}

// ValidateCountryCode checks ISO 3166-1 alpha-2 format (2 uppercase letters).
func ValidateCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	return code == strings.ToUpper(code) && code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z'
}
