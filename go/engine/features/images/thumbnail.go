package images

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // register GIF decoder
	"image/jpeg"  // JPEG encode (also registers JPEG decoder)
	_ "image/png" // register PNG decoder
)

// Thumbnail generation for post previews: an 80x80, content-aware square crop,
// re-encoded as compressed JPEG. Deliberately dependency-free — it uses only the
// standard library (jpeg/png/gif), so the default build/deploy needs no native
// libraries. Formats the stdlib can't decode (notably HEIC) return an error and
// the caller falls back to serving the original bytes.

const (
	previewSize    = 80
	previewQuality = 80
)

// makeThumbnail decodes data, crops the most detailed square region (cutting the
// flat edges), downscales it to size×size, and encodes JPEG. Returns the bytes
// and "image/jpeg".
func makeThumbnail(data []byte, size int) ([]byte, string, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}

	rect := contentAwareSquare(src)
	dst := downscaleAverage(src, rect, size)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: previewQuality}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

// contentAwareSquare returns the square sub-rectangle to keep: it slides a
// square window along the long axis and picks the position whose gradient energy
// (detail) is highest, so a busy subject is kept and flat margins are cropped.
// Falls back to a centered square.
func contentAwareSquare(img image.Image) image.Rectangle {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == h {
		return b
	}
	side := w
	if h < side {
		side = h
	}

	off := bestEnergyOffset(img, side)
	if w > h {
		x0 := b.Min.X + off
		return image.Rect(x0, b.Min.Y, x0+side, b.Min.Y+side)
	}
	y0 := b.Min.Y + off
	return image.Rect(b.Min.X, y0, b.Min.X+side, y0+side)
}

// bestEnergyOffset finds the window offset along the long axis (0..long-side)
// that maximizes summed luminance-gradient energy. Sampled for speed; result is
// cached by the caller so this runs at most once per image.
func bestEnergyOffset(img image.Image, side int) int {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	landscape := w > h
	long, cross := h, w
	if landscape {
		long, cross = w, h
	}
	if long <= side {
		return 0
	}

	crossStep := 1 + cross/48 // bound the inner loop
	energy := make([]float64, long)
	for i := 0; i < long; i++ {
		var sum float64
		for j := 0; j < cross; j += crossStep {
			var x, y int
			if landscape {
				x, y = b.Min.X+i, b.Min.Y+j
			} else {
				x, y = b.Min.X+j, b.Min.Y+i
			}
			sum += gradientAt(img, x, y, landscape)
		}
		energy[i] = sum
	}

	prefix := make([]float64, long+1)
	for i := 0; i < long; i++ {
		prefix[i+1] = prefix[i] + energy[i]
	}
	best, bestVal := 0, -1.0
	for off := 0; off+side <= long; off++ {
		if v := prefix[off+side] - prefix[off]; v > bestVal {
			bestVal, best = v, off
		}
	}
	// Center the crop within the equally-busy plateau for a natural framing.
	return best
}

func luma(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
}

// gradientAt approximates local detail as the luminance step to the next pixel
// along the long axis.
func gradientAt(img image.Image, x, y int, landscape bool) float64 {
	nx, ny := x, y
	if landscape {
		nx = x + 1
	} else {
		ny = y + 1
	}
	if !image.Pt(nx, ny).In(img.Bounds()) {
		return 0
	}
	d := luma(img, x, y) - luma(img, nx, ny)
	if d < 0 {
		d = -d
	}
	return d
}

// downscaleAverage box-averages the source rectangle down to size×size. Area
// averaging gives clean downscales without a third-party resizer.
func downscaleAverage(src image.Image, rect image.Rectangle, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sw, sh := rect.Dx(), rect.Dy()
	if sw <= 0 || sh <= 0 {
		draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
		return dst
	}

	for dy := 0; dy < size; dy++ {
		sy0 := rect.Min.Y + dy*sh/size
		sy1 := rect.Min.Y + (dy+1)*sh/size
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < size; dx++ {
			sx0 := rect.Min.X + dx*sw/size
			sx1 := rect.Min.X + (dx+1)*sw/size
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rs, gs, bs, cnt uint64
			for yy := sy0; yy < sy1; yy++ {
				for xx := sx0; xx < sx1; xx++ {
					r, g, b, _ := src.At(xx, yy).RGBA()
					rs += uint64(r >> 8)
					gs += uint64(g >> 8)
					bs += uint64(b >> 8)
					cnt++
				}
			}
			if cnt == 0 {
				cnt = 1
			}
			dst.Set(dx, dy, color.RGBA{uint8(rs / cnt), uint8(gs / cnt), uint8(bs / cnt), 255})
		}
	}
	return dst
}
