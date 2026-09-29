// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/maloquacious/hmz2map"
)

// Options are the rendering choices.
type Options struct {
	// Apothem is a hex's apothem in pixels; at least MinApothem.
	Apothem int
	// Outlines draws hex outlines.
	Outlines bool
	// WetlandsAsLand treats hexes with a marshes, swamps, or mangroves
	// surface as land for rivers. By default they count as water, so no
	// river is drawn along their sides.
	WetlandsAsLand bool
}

// Report counts what Render drew.
type Report struct {
	Width, Height int
	// HexPixels and BackgroundPixels count the pixels in a hex and in none.
	HexPixels, BackgroundPixels int
	// Ties counts the times a pixel was in a hex after the one that owns
	// it, under the tie rule.
	Ties int
	// OutlinePixels counts the pixels drawn as outlines.
	OutlinePixels int
	// EdgesDrawn counts the river edges drawn, by size. ShoreEdgesSkipped
	// counts the edges between land and water, and WaterEdgesSkipped those
	// with no land on either side.
	EdgesDrawn                           map[hmz2map.RiverSize]int
	ShoreEdgesSkipped, WaterEdgesSkipped int
	// Mouths counts the river mouths drawn, by size.
	Mouths map[hmz2map.RiverSize]int
	// RiverPixels counts the pixels colored as river.
	RiverPixels int
}

// Check returns an error if the map isn't one the renderer can draw: the
// wrong schema version or layout, or hexes not in row-then-column order.
func Check(m *hmz2map.Map) error {
	if m.SchemaVersion != hmz2map.SchemaVersion {
		return fmt.Errorf("schema_version is %d; map2png %s reads only version %d", m.SchemaVersion, Version(), hmz2map.SchemaVersion)
	}
	if m.Layout != hmz2map.Layout {
		return fmt.Errorf("layout is %q, not %q", m.Layout, hmz2map.Layout)
	}
	if m.Columns <= 0 || m.Rows <= 0 {
		return fmt.Errorf("map is %d × %d", m.Columns, m.Rows)
	}
	if len(m.Hexes) != m.Columns*m.Rows {
		return fmt.Errorf("map is %d × %d but lists %d hexes", m.Columns, m.Rows, len(m.Hexes))
	}
	for i, h := range m.Hexes {
		if h.Col != i%m.Columns || h.Row != i/m.Columns {
			return fmt.Errorf("hex %d is (%d, %d), not (%d, %d)", i, h.Col, h.Row, i%m.Columns, i/m.Columns)
		}
	}
	return nil
}

// Render draws the map. The map must pass Check.
func Render(m *hmz2map.Map, opt Options) (*image.RGBA, Report, error) {
	if opt.Apothem < MinApothem {
		return nil, Report{}, fmt.Errorf("apothem %d px is below the minimum of %d", opt.Apothem, MinApothem)
	}
	if err := Check(m); err != nil {
		return nil, Report{}, err
	}
	fills := make([]color.RGBA, len(m.Hexes))
	for i := range m.Hexes {
		c, err := FillColor(&m.Hexes[i])
		if err != nil {
			return nil, Report{}, err
		}
		fills[i] = c
	}

	g := NewGeometry(m.Columns, m.Rows, opt.Apothem)
	w, h := g.Size()
	rep := Report{Width: w, Height: h, EdgesDrawn: map[hmz2map.RiverSize]int{}, Mouths: map[hmz2map.RiverSize]int{}}
	owner := Owners(g, &rep.Ties)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for py := range h {
		for px := range w {
			i := py*w + px
			s := owner[i]
			c := BackgroundColor
			if s < 0 {
				rep.BackgroundPixels++
			} else {
				rep.HexPixels++
				c = fills[s]
				if opt.Outlines && ((px+1 < w && owner[i+1] != s) || (py+1 < h && owner[i+w] != s)) {
					c = OutlineColor(c)
					rep.OutlinePixels++
				}
			}
			img.SetRGBA(px, py, c)
		}
	}

	wet := func(h *hmz2map.Hex) bool { return h != nil && isWet(h, opt.WetlandsAsLand) }
	riverPixel := make([]bool, w*h)
	for i := range m.Hexes {
		hx := &m.Hexes[i]
		for _, r := range hx.Rivers {
			nc, nr := hmz2map.Neighbor(hx.Col, hx.Row, r.Side)
			other := m.At(nc, nr)
			switch r.Side {
			case hmz2map.SideS, hmz2map.SideSW, hmz2map.SideNW:
				if other != nil {
					continue // drawn by the other hex
				}
			}
			switch {
			case wet(hx) && (other == nil || wet(other)):
				rep.WaterEdgesSkipped++
				continue
			case wet(hx) || wet(other):
				rep.ShoreEdgesSkipped++
				continue
			}
			width, err := RiverWidth(r.Size, opt.Apothem)
			if err != nil {
				return nil, Report{}, fmt.Errorf("hex (%d, %d): %w", hx.Col, hx.Row, err)
			}
			rep.EdgesDrawn[r.Size]++
			c1, c2 := r.Side.Corners()
			ax, ay := g.Corner(hx.Col, hx.Row, c1)
			bx, by := g.Corner(hx.Col, hx.Row, c2)
			drawSegment(img, riverPixel, ax, ay, bx, by, width)

			// A mouth where the water flows into a wet hex at the
			// downstream corner.
			var across hmz2map.Side
			switch r.Flow {
			case c1:
				across = sideAt(r.Side, -1)
			case c2:
				across = sideAt(r.Side, +1)
			default:
				return nil, Report{}, fmt.Errorf("hex (%d, %d): river on side %s flows to corner %q", hx.Col, hx.Row, r.Side, r.Flow)
			}
			mc, mr := hmz2map.Neighbor(hx.Col, hx.Row, across)
			if mouth := m.At(mc, mr); wet(mouth) {
				vx, vy := g.Corner(hx.Col, hx.Row, r.Flow)
				drawMouth(img, riverPixel, owner, int32(mr*m.Columns+mc), vx, vy, MouthScale*width)
				rep.Mouths[r.Size]++
			}
		}
	}
	for _, p := range riverPixel {
		if p {
			rep.RiverPixels++
		}
	}
	return img, rep, nil
}

// Owners returns, for every pixel in row-major order, the index of the hex
// that owns it, or -1 for background. A pixel belongs to every hex whose
// snapped hex distance from its center is at most 1, and of those to the one
// with the lowest index. ties, if not nil, is increased once for each hex a
// pixel is in besides its owner.
func Owners(g Geometry, ties *int) []int32 {
	w, h := g.Size()
	owner := make([]int32, w*h)
	for i := range owner {
		owner[i] = -1
	}
	for row := range g.Rows {
		for col := range g.Columns {
			idx := int32(row*g.Columns + col)
			x0, y0 := g.Center(col, row)
			xlo, xhi := max(int(math.Floor(x0-g.S)), 0), min(int(math.Ceil(x0+g.S)), w-1)
			ylo, yhi := max(int(y0)-g.Apothem, 0), min(int(y0)+g.Apothem, h-1)
			for py := ylo; py <= yhi; py++ {
				for px := xlo; px <= xhi; px++ {
					if g.HexDistance(float64(px)+0.5, float64(py)+0.5, x0, y0) > snapUnit {
						continue
					}
					// Hexes are visited in index order, so the first to
					// claim a pixel has the lowest index.
					if owner[py*w+px] < 0 {
						owner[py*w+px] = idx
					} else if ties != nil {
						*ties++
					}
				}
			}
		}
	}
	return owner
}

// RiverWidth returns the width in pixels of a river line of the given size.
func RiverWidth(size hmz2map.RiverSize, apothem int) (int, error) {
	var k int
	switch size {
	case hmz2map.SizeStream:
		k = 1
	case hmz2map.SizeRiver:
		k = 2
	case hmz2map.SizeGreatRiver:
		k = 3
	default:
		return 0, fmt.Errorf("unknown river size %q", size)
	}
	return max(1, (2*k*apothem+24)/48), nil
}

// MouthScale is a river mouth's radius, in river widths.
const MouthScale = 3

func isWater(h *hmz2map.Hex) bool {
	return h.Landform == hmz2map.LandformSaltWater || h.Landform == hmz2map.LandformFreshWater
}

// isWet reports whether a river treats the hex as water: salt or fresh
// water, or, unless wetlandsAsLand, a marshes, swamps, or mangroves surface.
func isWet(h *hmz2map.Hex, wetlandsAsLand bool) bool {
	if isWater(h) {
		return true
	}
	switch h.Surface {
	case hmz2map.SurfaceMarshes, hmz2map.SurfaceSwamps, hmz2map.SurfaceMangroves:
		return !wetlandsAsLand
	}
	return false
}

// sideAt returns the side d steps clockwise from s.
func sideAt(s hmz2map.Side, d int) hmz2map.Side {
	for i, t := range hmz2map.Sides {
		if t == s {
			return hmz2map.Sides[(i+d+6)%6]
		}
	}
	panic("invalid side " + string(s))
}

// drawMouth colors every pixel owned by hex idx whose center is within
// radius of (vx, vy), under the rule of within: the part of the circle
// inside the hex, a 120° sector when the center is one of its corners.
func drawMouth(img *image.RGBA, mark []bool, owner []int32, idx int32, vx, vy float64, radius int) {
	b := img.Bounds()
	r := float64(radius) + 1
	xlo, xhi := max(int(math.Floor(vx-r)), 0), min(int(math.Ceil(vx+r)), b.Dx()-1)
	ylo, yhi := max(int(math.Floor(vy-r)), 0), min(int(math.Ceil(vy+r)), b.Dy()-1)
	for py := ylo; py <= yhi; py++ {
		for px := xlo; px <= xhi; px++ {
			i := py*b.Dx() + px
			if owner[i] == idx && within(float64(px)+0.5-vx, float64(py)+0.5-vy, 2*radius) {
				img.SetRGBA(px, py, RiverColor)
				mark[i] = true
			}
		}
	}
}

// drawSegment colors every pixel whose center is on the segment's line of
// the given width (see onSegment).
func drawSegment(img *image.RGBA, mark []bool, ax, ay, bx, by float64, width int) {
	b := img.Bounds()
	r := float64(width)/2 + 1
	xlo, xhi := max(int(math.Floor(min(ax, bx)-r)), 0), min(int(math.Ceil(max(ax, bx)+r)), b.Dx()-1)
	ylo, yhi := max(int(math.Floor(min(ay, by)-r)), 0), min(int(math.Ceil(max(ay, by)+r)), b.Dy()-1)
	for py := ylo; py <= yhi; py++ {
		for px := xlo; px <= xhi; px++ {
			if onSegment(float64(px)+0.5, float64(py)+0.5, ax, ay, bx, by, width) {
				img.SetRGBA(px, py, RiverColor)
				mark[py*b.Dx()+px] = true
			}
		}
	}
}
