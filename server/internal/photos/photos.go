// Package photos prepares profile photos: it decodes the upload, shrinks it
// and saves it again as JPEG. Saving again drops all metadata (EXIF, which
// can hold the GPS location where the photo was taken).
package photos

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoding

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoding
)

// MaxSide is the longest side, in pixels, of a stored photo.
const MaxSide = 800

var ErrNotImage = errors.New("the photo must be a JPEG, PNG or WebP image")

// Prepare returns the photo as a metadata-free JPEG, at most MaxSide pixels
// on its longest side.
func Prepare(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotImage
	}

	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if longest := max(w, h); longest > MaxSide {
		w, h = w*MaxSide/longest, h*MaxSide/longest
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
