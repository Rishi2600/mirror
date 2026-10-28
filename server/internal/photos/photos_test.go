package photos

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// jpegWithGPS is a 1600x1200 JPEG carrying an EXIF block with GPS text,
// like a phone photo.
func jpegWithGPS(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1200))
	for x := range 1600 {
		img.Set(x, 600, color.RGBA{R: 200, A: 255})
	}
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, img, nil); err != nil {
		t.Fatal(err)
	}

	exif := []byte("Exif\x00\x00GPSLatitude 52.5200 GPSLongitude 13.4050")
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(exif)+2))
	segment = append(segment, exif...)

	// Put the EXIF segment right after the JPEG start marker.
	out := append([]byte{}, plain.Bytes()[:2]...)
	out = append(out, segment...)
	return append(out, plain.Bytes()[2:]...)
}

func TestPrepareStripsMetadataAndShrinks(t *testing.T) {
	in := jpegWithGPS(t)
	if !bytes.Contains(in, []byte("GPSLatitude")) {
		t.Fatal("test photo has no GPS data to strip")
	}

	out, err := Prepare(in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS")) {
		t.Error("the prepared photo still has EXIF/GPS data")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	if cfg.Width != 800 || cfg.Height != 600 {
		t.Errorf("size = %dx%d, want 800x600", cfg.Width, cfg.Height)
	}
}

func TestPrepareRejectsNonImages(t *testing.T) {
	if _, err := Prepare([]byte("%PDF-1.4 not a photo")); err != ErrNotImage {
		t.Errorf("error = %v, want ErrNotImage", err)
	}
}
