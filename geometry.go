// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package map2png renders an hmz2map hex map as a PNG.
package map2png

import (
	"math"

	"github.com/maloquacious/hmz2map"
)

// MinApothem is the smallest apothem, in pixels, the renderer accepts.
const MinApothem = 4

// snapUnit is the number of snapped units in one hex distance or one pixel:
// distances are snapped to 6 decimals before they are compared.
const snapUnit = 1_000_000

var sqrt3 = math.Sqrt(3)

// Geometry is the image layout of a flat-top map with odd columns pushed
// down half a hex. Coordinates are pixels, x right and y down.
//
// Products are wrapped in float64() so the compiler can't fuse them into
// multiply-adds: the results must match an independent calculation to the
// last bit.
type Geometry struct {
	Columns, Rows int
	// Apothem is the distance from a hex's center to the middle of a side.
	Apothem int
	// A and S are the apothem and side length.
	A, S float64
}

// NewGeometry returns the layout of a map of the given size.
func NewGeometry(columns, rows, apothem int) Geometry {
	a := float64(apothem)
	return Geometry{Columns: columns, Rows: rows, Apothem: apothem, A: a, S: 2 * a / sqrt3}
}

// Size returns the image's width and height in pixels.
func (g Geometry) Size() (int, int) {
	w := 2*g.S + float64(1.5*g.S*float64(g.Columns-1))
	return int(math.Ceil(w)), 2*g.Apothem*g.Rows + g.Apothem
}

// Center returns the center of hex (col, row).
func (g Geometry) Center(col, row int) (float64, float64) {
	x := g.S + float64(1.5*g.S*float64(col))
	y := g.Apothem + 2*g.Apothem*row
	if col&1 == 1 {
		y += g.Apothem
	}
	return x, float64(y)
}

// Corner returns the position of a corner of hex (col, row).
func (g Geometry) Corner(col, row int, c hmz2map.Corner) (float64, float64) {
	x, y := g.Center(col, row)
	h := g.S / 2
	switch c {
	case hmz2map.CornerE:
		return x + g.S, y
	case hmz2map.CornerSE:
		return x + h, y + g.A
	case hmz2map.CornerSW:
		return x - h, y + g.A
	case hmz2map.CornerW:
		return x - g.S, y
	case hmz2map.CornerNW:
		return x - h, y - g.A
	case hmz2map.CornerNE:
		return x + h, y - g.A
	}
	panic("invalid corner " + string(c))
}

// HexDistance returns the snapped hex distance of the point (x, y) from the
// center (x0, y0), in millionths: at most snapUnit inside the hex, sides
// included.
func (g Geometry) HexDistance(x, y, x0, y0 float64) int64 {
	dx, dy := math.Abs(x-x0), math.Abs(y-y0)
	d := max(dy, (float64(sqrt3*dx)+dy)/2) / g.A
	return snap(d)
}

// snap rounds a non-negative value to millionths.
func snap(v float64) int64 {
	return int64(math.Floor(float64(v*snapUnit) + 0.5))
}

// onSegment reports whether (px, py) is within half of width of the segment
// from (ax, ay) to (bx, by) (see within).
func onSegment(px, py, ax, ay, bx, by float64, width int) bool {
	ux, uy := bx-ax, by-ay
	t := (float64((px-ax)*ux) + float64((py-ay)*uy)) / (float64(ux*ux) + float64(uy*uy))
	t = min(max(t, 0), 1)
	return within(px-(ax+float64(t*ux)), py-(ay+float64(t*uy)), width)
}

// within reports whether a point at offset (ex, ey) from its nearest point
// on a shape is within half of width of it. The distance is snapped; a point
// exactly at the limit counts only if it is above the nearest point, or
// level with it and to its left.
func within(ex, ey float64, width int) bool {
	d, limit := snap(math.Sqrt(float64(ex*ex)+float64(ey*ey))), int64(width)*snapUnit/2
	return d < limit || d == limit && (ey < 0 || ey == 0 && ex < 0)
}
