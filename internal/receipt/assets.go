package receipt

import (
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"path"
	"strings"
)

// Raster is a 1-bit image in the row-major, MSB-first packing GS v 0 expects.
type Raster struct {
	Width  int
	Height int
	Bits   []byte
}

type Assets map[string]Raster

// LoadAssets decodes every PNG in dir into a printable raster. Pixels darker
// than mid-grey print as black dots. Logos exported as white-on-transparent
// come out inverted, so those get flipped on load.
func LoadAssets(fsys fs.FS, dir string) (Assets, error) {
	out := Assets{}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return out, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		f, err := fsys.Open(path.Join(dir, e.Name()))
		if err != nil {
			return out, err
		}
		r, err := decodeRaster(f, strings.HasSuffix(e.Name(), ".invert.png"))
		f.Close()
		if err != nil {
			return out, err
		}
		name := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".png"), ".invert")
		out[name] = r
	}
	return out, nil
}

func decodeRaster(r io.Reader, invert bool) (Raster, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return Raster{}, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	xb := (w + 7) / 8
	bits := make([]byte, xb*h)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cr, cg, cb, ca := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			// Transparent pixels are paper, never ink.
			lum := (299*cr + 587*cg + 114*cb) / 1000
			on := ca > 0x7FFF && lum < 0x7FFF
			if invert {
				on = ca > 0x7FFF && lum >= 0x7FFF
			}
			if on {
				bits[y*xb+x/8] |= 0x80 >> (x % 8)
			}
		}
	}
	return Raster{Width: w, Height: h, Bits: bits}, nil
}
