// Package scrub removes contact details (emails, phone numbers and links)
// from free text, so they never reach an employer before a match.
package scrub

import (
	"regexp"
	"strings"
)

// Removed counts what was taken out of a text.
type Removed struct {
	Emails int `json:"emails"`
	Phones int `json:"phones"`
	Links  int `json:"links"`
}

// Add returns the sum of two counts.
func (r Removed) Add(o Removed) Removed {
	return Removed{Emails: r.Emails + o.Emails, Phones: r.Phones + o.Phones, Links: r.Links + o.Links}
}

var (
	email = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	// Links with a scheme or www., or bare domains with a path (linkedin.com/in/x).
	link = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+|\b[a-z0-9-]+(?:\.[a-z0-9-]+)*\.(?:com|net|org|io|dev|me|co|in|uk|ai|app)/\S*`)
	// Digit runs with phone-like separators; checked further in isPhone.
	phoneLike = regexp.MustCompile(`\+?\(?\d[\d\s().-]{6,}\d`)
	year      = regexp.MustCompile(`^(?:19|20)\d\d$`)
	digitRun  = regexp.MustCompile(`\d+`)
	spaces    = regexp.MustCompile(`[ \t]{2,}`)
)

// Text returns s without emails, links and phone numbers, and what it removed.
func Text(s string) (string, Removed) {
	var r Removed
	s = email.ReplaceAllStringFunc(s, func(string) string { r.Emails++; return "" })
	s = link.ReplaceAllStringFunc(s, func(string) string { r.Links++; return "" })
	s = phoneLike.ReplaceAllStringFunc(s, func(m string) string {
		if !isPhone(m) {
			return m
		}
		r.Phones++
		return ""
	})
	s = spaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s), r
}

// isPhone accepts 9 to 15 digits, but not runs that are only years
// ("2019 - 2022").
func isPhone(m string) bool {
	groups := digitRun.FindAllString(m, -1)
	digits := 0
	allYears := true
	for _, g := range groups {
		digits += len(g)
		if !year.MatchString(g) {
			allYears = false
		}
	}
	return digits >= 9 && digits <= 15 && !allYears
}
