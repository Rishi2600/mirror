// Package cvtext pulls plain text out of CV files (PDF and DOCX).
package cvtext

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

const (
	TypePDF  = "application/pdf"
	TypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

var (
	ErrUnsupported = errors.New("the CV must be a PDF or DOCX file")
	ErrNoText      = errors.New("no text found in the CV (is it a scanned image?)")
)

// DetectType returns the file's type from its first bytes, not its name.
func DetectType(data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return TypePDF, nil
	case bytes.HasPrefix(data, []byte("PK\x03\x04")) && hasDocumentXML(data):
		return TypeDOCX, nil
	}
	return "", ErrUnsupported
}

// Extract returns the text of a PDF or DOCX file.
func Extract(data []byte) (string, error) {
	kind, err := DetectType(data)
	if err != nil {
		return "", err
	}
	var text string
	if kind == TypePDF {
		text, err = pdfText(data)
	} else {
		text, err = docxText(data)
	}
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ErrNoText
	}
	return text, nil
}

func pdfText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	plain, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(plain)
	return string(b), err
}

// docxText reads word/document.xml and keeps the text of each paragraph
// (<w:p>) on its own line.
func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	f, err := zr.Open("word/document.xml")
	if err != nil {
		return "", err
	}
	defer f.Close()

	var out strings.Builder
	dec := xml.NewDecoder(f)
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			inText = t.Name.Local == "t"
			if t.Name.Local == "tab" {
				out.WriteString("\t")
			}
		case xml.EndElement:
			inText = false
			if t.Name.Local == "p" {
				out.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				out.Write(t)
			}
		}
	}
	return out.String(), nil
}

func hasDocumentXML(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			return true
		}
	}
	return false
}
