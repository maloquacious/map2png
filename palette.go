// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"fmt"
	"image/color"

	"github.com/maloquacious/hmz2map"
)

var (
	BackgroundColor = color.RGBA{0xff, 0xff, 0xff, 0xff}
	InlandSeaColor  = color.RGBA{0x2a, 0xa0, 0x98, 0xff}
	CoastWaterColor = color.RGBA{0x8e, 0xc9, 0xe8, 0xff}
	FreshWaterColor = color.RGBA{0x1f, 0x78, 0xd1, 0xff}
	CliffsColor     = color.RGBA{0xc0, 0x10, 0x10, 0xff}
	BadlandsColor   = color.RGBA{0x70, 0x20, 0x40, 0xff}
	VolcanoColor    = color.RGBA{0xff, 0x00, 0xc8, 0xff}
	RiverColor      = color.RGBA{0x10, 0x3a, 0xc8, 0xff}

	DepthColors = map[hmz2map.Depth]color.RGBA{
		hmz2map.DepthShallow: {0x4f, 0x93, 0xc4, 0xff},
		hmz2map.DepthOpen:    {0x2b, 0x57, 0x8c, 0xff},
		hmz2map.DepthDeep:    {0x14, 0x2c, 0x55, 0xff},
	}
	SurfaceColors = map[hmz2map.Surface]color.RGBA{
		hmz2map.SurfaceGlacialIce: {0xe8, 0xf4, 0xff, 0xff},
		hmz2map.SurfaceMarshes:    {0x7a, 0xd0, 0xc0, 0xff},
		hmz2map.SurfaceSwamps:     {0x3c, 0x8c, 0x7c, 0xff},
		hmz2map.SurfaceBogs:       {0x8a, 0x7a, 0x9a, 0xff},
		hmz2map.SurfaceMangroves:  {0x00, 0xa0, 0x80, 0xff},
		hmz2map.SurfaceSaltFlats:  {0xff, 0xf0, 0xf5, 0xff},
	}
	BiomeColors = map[hmz2map.Biome]color.RGBA{
		hmz2map.BiomeTundra:              {0x9f, 0xb3, 0xa6, 0xff},
		hmz2map.BiomeAlpine:              {0xbd, 0xb6, 0xad, 0xff},
		hmz2map.BiomeDesert:              {0xf2, 0xe2, 0xa8, 0xff},
		hmz2map.BiomeScrubland:           {0xc9, 0xa8, 0x6a, 0xff},
		hmz2map.BiomeGrassland:           {0xa9, 0xcf, 0x6e, 0xff},
		hmz2map.BiomeSteppe:              {0xd6, 0xcf, 0x98, 0xff},
		hmz2map.BiomeSavanna:             {0xd9, 0xb4, 0x4a, 0xff},
		hmz2map.BiomeBorealForest:        {0x3f, 0x6a, 0x5a, 0xff},
		hmz2map.BiomeTemperateForest:     {0x4f, 0x8f, 0x3a, 0xff},
		hmz2map.BiomeTemperateRainforest: {0x2f, 0x6f, 0x4f, 0xff},
		hmz2map.BiomeTropicalDryForest:   {0x8f, 0xaa, 0x3c, 0xff},
		hmz2map.BiomeTropicalRainforest:  {0x1f, 0x5f, 0x1f, 0xff},
		hmz2map.BiomeCloudForest:         {0x5a, 0x8f, 0x9a, 0xff},
	}
	// LandTints is each land landform's tint, in percent: positive lightens,
	// negative darkens.
	LandTints = map[hmz2map.Landform]int{
		hmz2map.LandformFlats:             15,
		hmz2map.LandformPlains:            8,
		hmz2map.LandformRollingPlains:     0,
		hmz2map.LandformHills:             -12,
		hmz2map.LandformMountains:         -25,
		hmz2map.LandformPlateaus:          -8,
		hmz2map.LandformVolcanicHighlands: -18,
	}
)

// ImpassableBlend is how far, in percent, impassable land is blended toward
// the cliff color.
const ImpassableBlend = 40

// FillColor returns the fill color of a hex, or an error if the hex has a
// value the palette doesn't cover.
func FillColor(h *hmz2map.Hex) (color.RGBA, error) {
	switch h.Landform {
	case hmz2map.LandformSaltWater:
		c, ok := DepthColors[h.Depth]
		if !ok {
			return c, fmt.Errorf("hex (%d, %d): salt water with depth %q", h.Col, h.Row, h.Depth)
		}
		switch {
		case h.HasFlag(hmz2map.FlagInlandSea):
			c = InlandSeaColor
		case h.HasFlag(hmz2map.FlagCoast):
			c = CoastWaterColor
		}
		return c, nil
	case hmz2map.LandformFreshWater:
		return FreshWaterColor, nil
	case hmz2map.LandformCliffs:
		return CliffsColor, nil
	case hmz2map.LandformBadlands:
		return BadlandsColor, nil
	}
	tint, ok := LandTints[h.Landform]
	if !ok {
		return color.RGBA{}, fmt.Errorf("hex (%d, %d): unknown landform %q", h.Col, h.Row, h.Landform)
	}
	if h.HasFlag(hmz2map.FlagVolcano) {
		return VolcanoColor, nil
	}
	var c color.RGBA
	if h.Surface == hmz2map.SurfaceClear {
		if c, ok = BiomeColors[h.Biome]; !ok {
			return c, fmt.Errorf("hex (%d, %d): land with surface %q and biome %q", h.Col, h.Row, h.Surface, h.Biome)
		}
	} else if c, ok = SurfaceColors[h.Surface]; !ok {
		return c, fmt.Errorf("hex (%d, %d): land with surface %q", h.Col, h.Row, h.Surface)
	}
	c = mapChannels(c, func(v int) int {
		if tint >= 0 {
			return v + (255-v)*tint/100
		}
		return v * (100 + tint) / 100
	})
	if h.HasFlag(hmz2map.FlagImpassable) {
		c = blend(c, CliffsColor, ImpassableBlend)
	}
	return c, nil
}

// OutlineColor returns the outline color for a fill color.
func OutlineColor(c color.RGBA) color.RGBA {
	return mapChannels(c, func(v int) int { return v * 3 / 4 })
}

// blend moves c pct percent of the way toward d.
func blend(c, d color.RGBA, pct int) color.RGBA {
	mix := func(v, w uint8) uint8 { return uint8((int(v)*(100-pct) + int(w)*pct) / 100) }
	return color.RGBA{mix(c.R, d.R), mix(c.G, d.G), mix(c.B, d.B), 0xff}
}

func mapChannels(c color.RGBA, f func(int) int) color.RGBA {
	return color.RGBA{uint8(f(int(c.R))), uint8(f(int(c.G))), uint8(f(int(c.B))), 0xff}
}
