package cvtext

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// The sample CVs in testdata describe a made-up person.
func TestExtractSampleCVs(t *testing.T) {
	for _, name := range []string{"sample-cv.pdf", "sample-cv.docx"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			text, err := Extract(data)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			for _, want := range []string{"Jordan Example", "Fictional Payments Ltd", "PostgreSQL"} {
				if !strings.Contains(text, want) {
					t.Errorf("text is missing %q:\n%s", want, text)
				}
			}
		})
	}
}

func TestDetectType(t *testing.T) {
	pdfData, _ := os.ReadFile("testdata/sample-cv.pdf")
	docxData, _ := os.ReadFile("testdata/sample-cv.docx")

	if kind, _ := DetectType(pdfData); kind != TypePDF {
		t.Errorf("PDF detected as %q", kind)
	}
	if kind, _ := DetectType(docxData); kind != TypeDOCX {
		t.Errorf("DOCX detected as %q", kind)
	}
	for _, data := range [][]byte{[]byte("hello"), []byte("PK\x03\x04not a docx"), nil} {
		if _, err := DetectType(data); !errors.Is(err, ErrUnsupported) {
			t.Errorf("DetectType(%q) error = %v, want ErrUnsupported", data, err)
		}
	}
}
