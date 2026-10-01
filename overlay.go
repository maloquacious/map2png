// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"github.com/maloquacious/hmz2map"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Vertical is where an overlay sits in its panel.
type Vertical int

const (
	Top Vertical = iota
	Middle
	Bottom
)

// Side is the side of the map a panel is added to.
type Side int

const (
	Left Side = iota
	Right
)

// Position places an overlay: a side of the map and a place in that side's
// panel.
type Position struct {
	Vertical Vertical
	Side     Side
}

// Positions lists every position, in the order the flags document them.
var Positions = []string{"top-left", "middle-left", "bottom-left", "top-right", "middle-right", "bottom-right"}

// ParsePosition parses a position such as "top-left" or "middle-right".
func ParsePosition(s string) (Position, error) {
	v, h, ok := strings.Cut(s, "-")
	var p Position
	switch v {
	case "top":
		p.Vertical = Top
	case "middle":
		p.Vertical = Middle
	case "bottom":
		p.Vertical = Bottom
	default:
		ok = false
	}
	switch h {
	case "left":
		p.Side = Left
	case "right":
		p.Side = Right
	default:
		ok = false
	}
	if !ok {
		return Position{}, fmt.Errorf("position %q: want one of %s", s, strings.Join(Positions, ", "))
	}
	return p, nil
}

func (p Position) String() string {
	return [...]string{"top", "middle", "bottom"}[p.Vertical] + "-" + [...]string{"left", "right"}[p.Side]
}

// Overlays are the optional legend and compass. A nil position leaves the
// overlay out.
type Overlays struct {
	Legend, Compass *Position
}

// Placement says where Decorate put the map and the overlays.
type Placement struct {
	Width, Height int
	// Map is the rectangle the map occupies: the map image shifted right by
	// the left panel's width.
	Map image.Rectangle
	// LeftPanel and RightPanel are the added panels; empty if not added.
	LeftPanel, RightPanel image.Rectangle
	// Legend and Compass are the overlays' rectangles; empty if left out.
	Legend, Compass image.Rectangle
}

// Overlay colors.
var (
	TextColor       = color.RGBA{0x20, 0x20, 0x20, 0xff}
	ArrowColor      = color.RGBA{0x30, 0x30, 0x30, 0xff}
	NorthColor      = color.RGBA{0xc0, 0x10, 0x10, 0xff}
	CompassHexColor = color.RGBA{0xe8, 0xe8, 0xe8, 0xff}
	TintSwatchColor = color.RGBA{0xa8, 0xa8, 0xa8, 0xff}
)

// HexOrientation is the way a map's hexes point.
type HexOrientation int

const (
	FlatTop HexOrientation = iota
	PointyTop
)

// OrientationOf returns the hex orientation of a map layout.
func OrientationOf(layout string) (HexOrientation, error) {
	switch {
	case strings.HasPrefix(layout, "flat-top"):
		return FlatTop, nil
	case strings.HasPrefix(layout, "pointy-top"):
		return PointyTop, nil
	}
	return 0, fmt.Errorf("layout %q: unknown hex orientation", layout)
}

// metrics are the overlay sizes, all derived from the map's apothem.
type metrics struct {
	a        int     // the map's apothem
	l        int     // swatch and compass hex apothem: 2a
	s        float64 // swatch side: 2l / sqrt(3)
	pad      int     // l / 2
	fontPx   int     // text size: l
	marginX  int     // panel margin, two hex widths: ceil(4 × map side)
	marginY  int     // panel margin, two hex heights: 4a
	swatchW  int     // ceil(2s)
	swatchH  int     // 2l
	face     font.Face
	capH     int // cap height of face, rounded
	outlines bool
}

func newMetrics(apothem int, outlines bool) (metrics, error) {
	l := 2 * apothem
	m := metrics{
		a: apothem, l: l, s: 2 * float64(l) / sqrt3, pad: l / 2, fontPx: l,
		marginX: int(math.Ceil(4 * 2 * float64(apothem) / sqrt3)), marginY: 4 * apothem,
		outlines: outlines,
	}
	m.swatchW, m.swatchH = int(math.Ceil(2*m.s)), 2*l
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return m, err
	}
	m.face, err = opentype.NewFace(f, &opentype.FaceOptions{Size: float64(m.fontPx), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return m, err
	}
	m.capH = m.face.Metrics().CapHeight.Round()
	return m, nil
}

func (m metrics) textWidth(s string) int {
	return font.MeasureString(m.face, s).Ceil()
}

// drawText draws s with its left end at x and its cap height centered on y.
func (m metrics) drawText(img draw.Image, s string, x, y int, c color.RGBA) {
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: m.face, Dot: fixed.P(x, y+m.capH/2)}
	d.DrawString(s)
}

// Decorate adds the panels and overlays to a rendered map. With no overlays
// it returns the map image unchanged.
func Decorate(mapImg *image.RGBA, m *hmz2map.Map, rep Report, opt Options, ov Overlays) (*image.RGBA, Placement, error) {
	mw, mh := mapImg.Bounds().Dx(), mapImg.Bounds().Dy()
	pl := Placement{Width: mw, Height: mh, Map: mapImg.Bounds()}
	if ov.Legend == nil && ov.Compass == nil {
		return mapImg, pl, nil
	}
	if ov.Legend != nil && ov.Compass != nil && *ov.Legend == *ov.Compass {
		return nil, pl, fmt.Errorf("the legend and the compass are both at %s", ov.Legend)
	}
	met, err := newMetrics(opt.Apothem, opt.Outlines)
	if err != nil {
		return nil, pl, err
	}
	orient, err := OrientationOf(m.Layout)
	if err != nil && ov.Compass != nil {
		return nil, pl, err
	}

	type item struct {
		pos  Position
		w, h int
		draw func(img *image.RGBA, x, y int)
		rect *image.Rectangle
	}
	var items []item
	if ov.Legend != nil {
		lg := newLegend(m, rep, met)
		items = append(items, item{*ov.Legend, lg.w, lg.h, lg.draw, &pl.Legend})
	}
	if ov.Compass != nil {
		cw := compassSize(met, orient)
		items = append(items, item{*ov.Compass, cw, cw, func(img *image.RGBA, x, y int) { drawCompass(img, met, orient, x, y, cw) }, &pl.Compass})
	}

	// Each side's panel is as wide as its widest overlay plus a margin on
	// each side.
	var panelW [2]int
	for _, it := range items {
		panelW[it.pos.Side] = max(panelW[it.pos.Side], it.w+2*met.marginX)
	}
	pl.Width = panelW[Left] + mw + panelW[Right]
	pl.Map = image.Rect(panelW[Left], 0, panelW[Left]+mw, mh)
	if panelW[Left] > 0 {
		pl.LeftPanel = image.Rect(0, 0, panelW[Left], mh)
	}
	if panelW[Right] > 0 {
		pl.RightPanel = image.Rect(pl.Map.Max.X, 0, pl.Width, mh)
	}

	for i := range items {
		it := &items[i]
		if it.h+2*met.marginY > mh {
			return nil, pl, fmt.Errorf("the %s at %s is %d px tall; the map is only %d px tall", itemName(it.rect == &pl.Legend), it.pos, it.h, mh)
		}
		x := met.marginX
		if it.pos.Side == Right {
			x += pl.Map.Max.X
		}
		var y int
		switch it.pos.Vertical {
		case Top:
			y = met.marginY
		case Middle:
			y = (mh - it.h) / 2
		case Bottom:
			y = mh - met.marginY - it.h
		}
		*it.rect = image.Rect(x, y, x+it.w, y+it.h)
	}
	if len(items) == 2 && items[0].pos.Side == items[1].pos.Side {
		a, b := *items[0].rect, *items[1].rect
		if a.Min.Y > b.Min.Y {
			a, b = b, a
		}
		if a.Max.Y+met.marginY > b.Min.Y {
			return nil, pl, fmt.Errorf("the legend at %s and the compass at %s overlap", ov.Legend, ov.Compass)
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, pl.Width, mh))
	draw.Draw(img, img.Bounds(), image.NewUniform(BackgroundColor), image.Point{}, draw.Src)
	draw.Draw(img, pl.Map, mapImg, image.Point{}, draw.Src)
	for _, it := range items {
		it.draw(img, it.rect.Min.X, it.rect.Min.Y)
	}
	return img, pl, nil
}

func itemName(legend bool) string {
	if legend {
		return "legend"
	}
	return "compass"
}

// legendEntry is one row of the legend: a hex swatch, or a river line of
// width river when river is set.
type legendEntry struct {
	label string
	fill  color.RGBA
	river int
}

type legendSection struct {
	title   string
	entries []legendEntry
}

type legend struct {
	columns [2][]legendSection
	colW    [2]int
	w, h    int
	met     metrics
}

func label(s string) string { return strings.ReplaceAll(s, "-", " ") }

// newLegend lays out the legend: the colors that appear on the map, in the
// README's order. Column 1 holds the base colors; column 2 the land tints,
// impassable land, and the river sizes drawn.
func newLegend(m *hmz2map.Map, rep Report, met metrics) legend {
	seenColor := map[string]bool{}
	seenSurface := map[hmz2map.Surface]bool{}
	seenBiome := map[hmz2map.Biome]bool{}
	seenLandform := map[hmz2map.Landform]bool{}
	impassable := false
	for i := range m.Hexes {
		h := &m.Hexes[i]
		switch h.Landform {
		case hmz2map.LandformSaltWater:
			switch {
			case h.HasFlag(hmz2map.FlagInlandSea):
				seenColor["inland"] = true
			case h.HasFlag(hmz2map.FlagCoast):
				seenColor["coast"] = true
			default:
				seenColor[string(h.Depth)] = true
			}
		case hmz2map.LandformFreshWater:
			seenColor["fresh"] = true
		case hmz2map.LandformCliffs:
			seenColor["cliffs"] = true
		case hmz2map.LandformBadlands:
			seenColor["badlands"] = true
		default:
			if h.HasFlag(hmz2map.FlagVolcano) {
				seenColor["volcano"] = true
				continue
			}
			if h.Surface == hmz2map.SurfaceClear {
				seenBiome[h.Biome] = true
			} else {
				seenSurface[h.Surface] = true
			}
			seenLandform[h.Landform] = true
			impassable = impassable || h.HasFlag(hmz2map.FlagImpassable)
		}
	}
	add := func(sec *legendSection, key, text string, c color.RGBA) {
		if seenColor[key] {
			sec.entries = append(sec.entries, legendEntry{label: text, fill: c})
		}
	}
	water := legendSection{title: "Water"}
	add(&water, "inland", "inland sea", InlandSeaColor)
	add(&water, "coast", "coast water", CoastWaterColor)
	for _, d := range hmz2map.Depths {
		add(&water, string(d), string(d)+" water", DepthColors[d])
	}
	add(&water, "fresh", "lake", FreshWaterColor)
	terrain := legendSection{title: "Terrain"}
	add(&terrain, "cliffs", "cliffs", CliffsColor)
	add(&terrain, "badlands", "badlands", BadlandsColor)
	add(&terrain, "volcano", "volcano", VolcanoColor)
	surfaces := legendSection{title: "Surface"}
	for _, s := range hmz2map.Surfaces {
		if seenSurface[s] {
			surfaces.entries = append(surfaces.entries, legendEntry{label: label(string(s)), fill: SurfaceColors[s]})
		}
	}
	biomes := legendSection{title: "Biome"}
	for _, b := range hmz2map.Biomes {
		if seenBiome[b] {
			biomes.entries = append(biomes.entries, legendEntry{label: label(string(b)), fill: BiomeColors[b]})
		}
	}
	relief := legendSection{title: "Relief"}
	for _, lf := range hmz2map.Landforms {
		if t, ok := LandTints[lf]; ok && seenLandform[lf] {
			relief.entries = append(relief.entries, legendEntry{label: label(string(lf)), fill: tint(TintSwatchColor, t)})
		}
	}
	if impassable {
		relief.entries = append(relief.entries, legendEntry{label: "impassable", fill: blend(TintSwatchColor, CliffsColor, ImpassableBlend)})
	}
	rivers := legendSection{title: "Rivers"}
	for _, s := range hmz2map.RiverSizes {
		if rep.EdgesDrawn[s] > 0 {
			// At the swatches' scale: twice the map's width.
			w, _ := RiverWidth(s, met.l)
			rivers.entries = append(rivers.entries, legendEntry{label: label(string(s)), fill: RiverColor, river: w})
		}
	}

	lg := legend{met: met}
	for _, sec := range []legendSection{water, terrain, surfaces, biomes} {
		if len(sec.entries) > 0 {
			lg.columns[0] = append(lg.columns[0], sec)
		}
	}
	for _, sec := range []legendSection{relief, rivers} {
		if len(sec.entries) > 0 {
			lg.columns[1] = append(lg.columns[1], sec)
		}
	}
	for c, secs := range lg.columns {
		h := 0
		for i, sec := range secs {
			if i > 0 {
				h += met.l
			}
			h += lg.titleH()
			lg.colW[c] = max(lg.colW[c], met.textWidth(sec.title))
			for _, e := range sec.entries {
				h += lg.rowH()
				lg.colW[c] = max(lg.colW[c], met.swatchW+met.pad+met.textWidth(e.label))
			}
		}
		lg.h = max(lg.h, h)
	}
	lg.w = lg.colW[0]
	if len(lg.columns[1]) > 0 {
		lg.w += 2*met.l + lg.colW[1]
	}
	return lg
}

func (lg legend) titleH() int { return lg.met.fontPx + lg.met.pad }
func (lg legend) rowH() int   { return lg.met.swatchH + lg.met.pad }

func (lg legend) draw(img *image.RGBA, x0, y0 int) {
	met := lg.met
	x := x0
	for c, secs := range lg.columns {
		y := y0
		for i, sec := range secs {
			if i > 0 {
				y += met.l
			}
			met.drawText(img, sec.title, x, y+met.fontPx/2, TextColor)
			y += lg.titleH()
			for _, e := range sec.entries {
				cx, cy := float64(x)+met.s, float64(y+met.l)
				if e.river > 0 {
					drawSegmentColor(img, cx-met.s, cy, cx+met.s, cy, e.river, e.fill)
				} else {
					fillHex(img, FlatTop, cx, cy, met.l, e.fill, met.outlines)
				}
				met.drawText(img, e.label, x+met.swatchW+met.pad, y+met.l, TextColor)
				y += lg.rowH()
			}
		}
		x += lg.colW[c] + 2*met.l
	}
}

// tint applies a land tint to a color, as FillColor does.
func tint(c color.RGBA, t int) color.RGBA {
	return mapChannels(c, func(v int) int {
		if t >= 0 {
			return v + (255-v)*t/100
		}
		return v * (100 + t) / 100
	})
}

// inHex reports whether the pixel center (px, py) is in the hex of the
// given orientation and apothem centered on (cx, cy).
func inHex(o HexOrientation, px, py, cx, cy float64, apothem int) bool {
	dx, dy := math.Abs(px-cx), math.Abs(py-cy)
	if o == PointyTop {
		dx, dy = dy, dx
	}
	return snap(max(dy, (float64(sqrt3*dx)+dy)/2)/float64(apothem)) <= snapUnit
}

// fillHex fills the hex centered on (cx, cy), with outline pixels as on the
// map when outlines is set.
func fillHex(img *image.RGBA, o HexOrientation, cx, cy float64, apothem int, c color.RGBA, outlines bool) {
	r := int(math.Ceil(2*float64(apothem)/sqrt3)) + 1
	in := func(px, py int) bool { return inHex(o, float64(px)+0.5, float64(py)+0.5, cx, cy, apothem) }
	for py := int(cy) - r; py <= int(cy)+r; py++ {
		for px := int(cx) - r; px <= int(cx)+r; px++ {
			if !in(px, py) {
				continue
			}
			fill := c
			if outlines && (!in(px+1, py) || !in(px, py+1)) {
				fill = OutlineColor(c)
			}
			img.SetRGBA(px, py, fill)
		}
	}
}

// compassDirections are the eight labels with their bearings in degrees
// clockwise from north.
var compassDirections = []struct {
	label   string
	bearing float64
}{{"N", 0}, {"NE", 45}, {"E", 90}, {"SE", 135}, {"S", 180}, {"SW", 225}, {"W", 270}, {"NW", 315}}

// compassArrows returns, for each label, the bearing of its arrow, or false
// if the direction faces a corner and has no neighbor. A flat-top hex's
// neighbors are N, NE, SE, S, SW, NW, at bearings 0°, 60°, 120°, …; a
// pointy-top hex's are NE, E, SE, SW, W, NW, at 30°, 90°, 150°, ….
// Players name the diagonal neighbors NE, SE, SW, NW either way.
func compassArrows(o HexOrientation) map[string]float64 {
	if o == PointyTop {
		return map[string]float64{"NE": 30, "E": 90, "SE": 150, "SW": 210, "W": 270, "NW": 330}
	}
	return map[string]float64{"N": 0, "NE": 60, "SE": 120, "S": 180, "SW": 240, "NW": 300}
}

// compassRadii returns the distances from the compass center to an arrow's
// base, to its tip, and to a label's center.
func compassRadii(met metrics) (base, tip, labelR float64) {
	l := float64(met.l)
	base = l + l/4
	tip = 2*l + l/4
	labelR = tip + l/4 + float64(met.fontPx)*0.75
	return
}

// compassSize returns the compass's width and height: a square.
func compassSize(met metrics, o HexOrientation) int {
	_, _, labelR := compassRadii(met)
	half := 0.0
	for _, d := range compassDirections {
		b := d.bearing
		if ab, ok := compassArrows(o)[d.label]; ok {
			b = ab
		}
		sin, cos := math.Sincos(b * math.Pi / 180)
		x := math.Abs(sin*labelR) + float64(met.textWidth(d.label))/2
		y := math.Abs(cos*labelR) + float64(met.capH)/2
		half = max(half, x, y)
	}
	return 2 * (int(math.Ceil(half)) + met.pad)
}

// drawCompass draws the compass in the size × size square at (x0, y0).
func drawCompass(img *image.RGBA, met metrics, o HexOrientation, x0, y0, size int) {
	cx, cy := float64(x0)+float64(size)/2, float64(y0)+float64(size)/2
	fillHex(img, o, cx, cy, met.l, CompassHexColor, true)
	base, tip, labelR := compassRadii(met)
	arrows := compassArrows(o)
	shaft := max(2, met.l/8)
	head := float64(met.l) / 2
	for _, d := range compassDirections {
		c := ArrowColor
		if d.label == "N" {
			c = NorthColor
		}
		bearing, hasArrow := arrows[d.label]
		if !hasArrow {
			bearing = d.bearing
		}
		ux, uy := math.Sincos(bearing * math.Pi / 180)
		uy = -uy
		if hasArrow {
			bx, by := cx+ux*base, cy+uy*base
			tx, ty := cx+ux*tip, cy+uy*tip
			hx, hy := tx-ux*head, ty-uy*head
			drawSegmentColor(img, bx, by, hx, hy, shaft, c)
			// The head: a triangle from the tip back to a base as wide
			// as it is long.
			px, py := -uy*head/2, ux*head/2
			fillTriangle(img, tx, ty, hx+px, hy+py, hx-px, hy-py, c)
		}
		lx, ly := cx+ux*labelR, cy+uy*labelR
		w := met.textWidth(d.label)
		met.drawText(img, d.label, int(math.Round(lx))-w/2, int(math.Round(ly)), c)
	}
}

// drawSegmentColor is drawSegment in any color, without marking pixels.
func drawSegmentColor(img *image.RGBA, ax, ay, bx, by float64, width int, c color.RGBA) {
	r := float64(width)/2 + 1
	for py := int(math.Floor(min(ay, by) - r)); py <= int(math.Ceil(max(ay, by)+r)); py++ {
		for px := int(math.Floor(min(ax, bx) - r)); px <= int(math.Ceil(max(ax, bx)+r)); px++ {
			if onSegment(float64(px)+0.5, float64(py)+0.5, ax, ay, bx, by, width) {
				img.SetRGBA(px, py, c)
			}
		}
	}
}

// fillTriangle colors every pixel whose center is inside the triangle.
func fillTriangle(img *image.RGBA, ax, ay, bx, by, cx, cy float64, c color.RGBA) {
	edge := func(x0, y0, x1, y1, px, py float64) float64 { return (x1-x0)*(py-y0) - (y1-y0)*(px-x0) }
	for py := int(math.Floor(min(ay, by, cy))); py <= int(math.Ceil(max(ay, by, cy))); py++ {
		for px := int(math.Floor(min(ax, bx, cx))); px <= int(math.Ceil(max(ax, bx, cx))); px++ {
			x, y := float64(px)+0.5, float64(py)+0.5
			e1, e2, e3 := edge(ax, ay, bx, by, x, y), edge(bx, by, cx, cy, x, y), edge(cx, cy, ax, ay, x, y)
			if (e1 >= 0 && e2 >= 0 && e3 >= 0) || (e1 <= 0 && e2 <= 0 && e3 <= 0) {
				img.SetRGBA(px, py, c)
			}
		}
	}
}
