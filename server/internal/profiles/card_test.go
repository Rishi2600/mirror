package profiles

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// syntheticCV is made-up CV number i, with contact details planted in the
// free-text fields, the way people write them in real CVs.
func syntheticCV(i int) CV {
	contacts := []string{
		fmt.Sprintf("Contact: person%d@example.com", i),
		fmt.Sprintf("Phone +1 (555) 010-%04d", i),
		fmt.Sprintf("Portfolio https://example.com/person%d", i),
		fmt.Sprintf("linkedin.com/in/person-%d", i),
		fmt.Sprintf("Call 555.010.%04d any time", i),
		fmt.Sprintf("github.com/person%d", i),
		fmt.Sprintf("www.person%d.example.org", i),
		fmt.Sprintf("+44 7700 900%03d", i),
	}
	c := func(n int) string { return contacts[(i+n)%len(contacts)] }
	return CV{
		FullName:        fmt.Sprintf("Test Person %d", i),
		Email:           fmt.Sprintf("person%d@example.com", i),
		Phone:           fmt.Sprintf("+1 555 010 %04d", i),
		Headline:        "Backend engineer " + c(0),
		Summary:         fmt.Sprintf("Engineer since 2015 - 2019 at two firms. %s. %s.", c(1), c(2)),
		CurrentTitle:    "Senior Engineer",
		CurrentEmployer: fmt.Sprintf("Fictional Co %d", i),
		Location:        "Example City",
		YearsExperience: 3 + i%10,
		Seniority:       "senior",
		Skills:          []string{"Go", "SQL"},
		Experience: []Experience{
			{Title: "Senior Engineer", Company: fmt.Sprintf("Fictional Co %d", i), Start: "2021", End: "present", Summary: "Built APIs. " + c(3)},
			{Title: "Engineer", Company: "Sample Ltd", Start: "2017", End: "2021", Summary: c(4)},
		},
		Education: []Education{{Degree: "BSc Computer Science", Institution: "Example University", Year: "2016"}},
	}
}

var leaks = regexp.MustCompile(`@|https?://|www\.|linkedin\.com|github\.com|\+\d|\d{3}[ .-]\d{3}[ .-]?\d{3,4}|\(\d{3}\)`)

// The done check for this chunk: 20 synthetic CVs give 20 cards with the
// name and photo, and no email, phone, link or CV file anywhere.
func TestCardsNeverLeakContactDetails(t *testing.T) {
	for i := 1; i <= 20; i++ {
		p := newProfile("user")
		p.CV = syntheticCV(i)
		p.ShowPay = true
		p.MinPay = 80000

		card, removed := BuildCard(p, "/v1/profile/photo")
		data, _ := json.Marshal(card)
		text := string(data)

		if m := leaks.FindString(text); m != "" {
			t.Errorf("card %d leaks %q:\n%s", i, m, text)
		}
		if strings.Contains(text, "cv_file") || strings.Contains(text, p.CV.Email) {
			t.Errorf("card %d has a CV file or email field: %s", i, text)
		}
		if card.Name != p.CV.FullName || card.PhotoURL == "" {
			t.Errorf("card %d is missing the name or photo: %+v", i, card)
		}
		if removed.Emails+removed.Phones+removed.Links == 0 {
			t.Errorf("card %d: nothing was reported as removed", i)
		}
		if !strings.Contains(card.Summary, "2015 - 2019") {
			t.Errorf("card %d: the scrub removed a year range: %q", i, card.Summary)
		}
	}
}

func TestCurrentEmployerDisplay(t *testing.T) {
	p := newProfile("user")
	p.CV = syntheticCV(1)

	for _, tt := range []struct{ display, description, want string }{
		{"name", "", "Fictional Co 1"},
		{"description", "a payments company", "a payments company"},
		{"hidden", "", ""},
	} {
		p.EmployerDisplay, p.EmployerDescription = tt.display, tt.description
		card, _ := BuildCard(p, "")
		if card.CurrentEmployer != tt.want || card.Experience[0].Company != tt.want {
			t.Errorf("%s: current employer %q, first job's company %q; want %q",
				tt.display, card.CurrentEmployer, card.Experience[0].Company, tt.want)
		}
		if card.Experience[1].Company != "Sample Ltd" {
			t.Errorf("%s: a past employer changed to %q", tt.display, card.Experience[1].Company)
		}
	}
}

func TestHiddenPay(t *testing.T) {
	p := newProfile("user")
	p.MinPay, p.ShowPay = 90000, false
	if card, _ := BuildCard(p, ""); card.MinPay != 0 {
		t.Errorf("hidden pay shows as %d", card.MinPay)
	}
}

func TestMissing(t *testing.T) {
	p := newProfile("user")
	if got := len(Missing(p)); got != 4 {
		t.Errorf("an empty profile misses %d things, want 4: %v", got, Missing(p))
	}
	p.ParseStatus = StatusDone
	p.CV.FullName, p.CV.Headline = "Test Person", "Engineer"
	photo := "photo-id"
	p.PhotoFileID = &photo
	if missing := Missing(p); len(missing) != 0 {
		t.Errorf("a complete profile misses %v", missing)
	}
}
