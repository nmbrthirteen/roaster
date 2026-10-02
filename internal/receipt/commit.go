package receipt

import (
	"image"
	"strings"
)

// Commit draws one commit as it sits in a git graph: a node on a branch line,
// its hash and repo beside it, and the message framed underneath.
type Commit struct {
	Ref     string // short hash
	Where   string // repo
	When    string
	Message string
}

// Dots, at 203 dpi.
const (
	branchX   = Gutter*dotsPerCol + 8 // the branch line's centre
	nodeR     = 7
	branchW   = 2
	textX     = branchX + 20 // where the hash, the time and the box start
	boxBorder = 2
	boxPad    = 8
)

func (c Commit) lines() []Line {
	drawMu.Lock()
	defer drawMu.Unlock()

	width := Width() * dotsPerCol
	right := width - Gutter*dotsPerCol
	boxCols := (right - textX - 2*boxBorder - 2*boxPad) / dotsPerCol
	msg := wrap(c.Message, boxCols)

	headY := 0
	whenY := headY + rowDots
	boxY := whenY + rowDots + 6
	boxH := 2*boxBorder + 2*boxPad + len(msg)*rowDots
	tailY := boxY + boxH + 10 + nodeR // the parent's node centre
	img := image.NewAlpha(image.Rect(0, 0, width, tailY+nodeR+1))

	fill := func(x0, y0, x1, y1 int) {
		for y := max(y0, 0); y < min(y1, img.Rect.Dy()); y++ {
			for x := max(x0, 0); x < min(x1, width); x++ {
				img.Pix[y*img.Stride+x] = 0xFF
			}
		}
	}
	// ring inks the dots within r of the centre, leaving a hole of radius hole.
	ring := func(cx, cy, r, hole int) {
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				if d := x*x + y*y; d <= r*r && d >= hole*hole {
					img.Pix[(cy+y)*img.Stride+cx+x] = 0xFF
				}
			}
		}
	}

	regular, bold := facesFor(false, emDots), facesFor(true, emDots)

	headMid := headY + rowDots/2
	fill(branchX-branchW/2, headMid, branchX+branchW/2, tailY)
	ring(branchX, headMid, nodeR, 0)
	// The parent is drawn hollow: it is only there to show the line goes on.
	for y := tailY - nodeR + 1; y <= tailY+nodeR; y++ {
		for x := branchX - nodeR; x <= branchX+nodeR; x++ {
			img.Pix[y*img.Stride+x] = 0
		}
	}
	ring(branchX, tailY, nodeR-1, nodeR-1-branchW)

	x := textX
	if c.Ref != "" {
		bold.draw(img, []rune(c.Ref), x, headY, rowDots, dotsPerCol)
		x += (len([]rune(c.Ref)) + 2) * dotsPerCol
	}
	if c.Where != "" {
		where := truncate(c.Where, (right-x)/dotsPerCol)
		regular.draw(img, []rune(where), x, headY, rowDots, dotsPerCol)
	}
	regular.draw(img, []rune(truncate(c.When, (right-textX)/dotsPerCol)), textX, whenY, rowDots, dotsPerCol)

	fill(textX, boxY, right, boxY+boxBorder)
	fill(textX, boxY+boxH-boxBorder, right, boxY+boxH)
	fill(textX, boxY, textX+boxBorder, boxY+boxH)
	fill(right-boxBorder, boxY, right, boxY+boxH)
	for i, s := range msg {
		regular.draw(img, []rune(s), textX+boxBorder+boxPad, boxY+boxBorder+boxPad+i*rowDots, rowDots, dotsPerCol)
	}

	r := threshold(img)
	alt := strings.TrimSpace(c.Ref + " in " + c.Where)
	if c.Ref == "" {
		alt = c.Where
	}
	return []Line{{Bitmap: &r, Alt: strings.Join(append([]string{alt, c.When}, msg...), "\n")}}
}
