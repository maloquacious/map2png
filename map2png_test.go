// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/maloquacious/hmz2map"
)

// testMap returns a small map: land in the middle of salt water, with a
// river along the land hex's sides.
func testMap(columns, rows int) *hmz2map.Map {
	m := &hmz2map.Map{SchemaVersion: hmz2map.SchemaVersion, Layout: hmz2map.Layout, Columns: columns, Rows: rows}
	for r := range rows {
		for c := range columns {
			m.Hexes = append(m.Hexes, hmz2map.Hex{Col: c, Row: r, Landform: hmz2map.LandformSaltWater,
				Surface: hmz2map.SurfaceClear, Biome: hmz2map.BiomeClear, Depth: hmz2map.DepthDeep})
		}
	}
	return m
}

func TestSize(t *testing.T) {
	for _, tc := range []struct{ columns, rows, apothem, w, h int }{
		{107, 222, 24, 4462, 10680},
		{115, 230, 24, 4795, 11064},
		{107, 222, 20, 3719, 8900},
		{107, 222, 12, 2231, 5340},
		{1, 1, 24, 56, 72},
	} {
		w, h := NewGeometry(tc.columns, tc.rows, tc.apothem).Size()
		if w != tc.w || h != tc.h {
			t.Errorf("%d × %d at %d: got %d × %d, want %d × %d", tc.columns, tc.rows, tc.apothem, w, h, tc.w, tc.h)
		}
	}
}

func TestParity(t *testing.T) {
	g := NewGeometry(3, 2, 24)
	x0, y0 := g.Center(0, 0)
	x1, y1 := g.Center(1, 0)
	x2, y2 := g.Center(2, 0)
	if y1 != y0+g.A || y1 != y2+g.A {
		t.Errorf("(1, 0) at y %v; want half a hex below (0, 0) at %v and (2, 0) at %v", y1, y0, y2)
	}
	if !(x0 < x1 && x1 < x2) {
		t.Errorf("centers not left to right: %v %v %v", x0, x1, x2)
	}
}

// TestNeighborsShareEdges checks that two hexes own 4-adjacent pixels
// exactly when hmz2map.Neighbor says they are neighbors, and that every
// side's corners are the neighbor's corners.
func TestNeighborsShareEdges(t *testing.T) {
	const columns, rows = 7, 6
	for _, apothem := range []int{4, 13, 24} {
		g := NewGeometry(columns, rows, apothem)
		w, h := g.Size()
		var ties int
		owner := Owners(g, &ties)
		if ties != 0 {
			t.Errorf("apothem %d: %d ties", apothem, ties)
		}
		touching := map[[2]int32]bool{}
		add := func(a, b int32) {
			if a >= 0 && b >= 0 && a != b {
				touching[[2]int32{min(a, b), max(a, b)}] = true
			}
		}
		for py := range h {
			for px := range w {
				if px+1 < w {
					add(owner[py*w+px], owner[py*w+px+1])
				}
				if py+1 < h {
					add(owner[py*w+px], owner[(py+1)*w+px])
				}
			}
		}
		want := map[[2]int32]bool{}
		for r := range rows {
			for c := range columns {
				for _, s := range hmz2map.Sides {
					nc, nr := hmz2map.Neighbor(c, r, s)
					if nc < 0 || nr < 0 || nc >= columns || nr >= rows {
						continue
					}
					a, b := int32(r*columns+c), int32(nr*columns+nc)
					want[[2]int32{min(a, b), max(a, b)}] = true

					c1, c2 := s.Corners()
					o1, o2 := s.Opposite().Corners()
					ax, ay := g.Corner(c, r, c1)
					bx, by := g.Corner(c, r, c2)
					// The neighbor sees the side the other way round.
					px, py := g.Corner(nc, nr, o2)
					qx, qy := g.Corner(nc, nr, o1)
					if !near(ax, px) || !near(ay, py) || !near(bx, qx) || !near(by, qy) {
						t.Errorf("apothem %d: side %s of (%d, %d) doesn't match (%d, %d)", apothem, s, c, r, nc, nr)
					}
				}
			}
		}
		for p := range want {
			if !touching[p] {
				t.Errorf("apothem %d: neighbors %v don't share an edge", apothem, p)
			}
		}
		for p := range touching {
			if !want[p] {
				t.Errorf("apothem %d: hexes %v touch but aren't neighbors", apothem, p)
			}
		}
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFillColor(t *testing.T) {
	land := func(l hmz2map.Landform, s hmz2map.Surface, b hmz2map.Biome, flags ...hmz2map.Flag) *hmz2map.Hex {
		return &hmz2map.Hex{Landform: l, Surface: s, Biome: b, Flags: flags}
	}
	water := func(d hmz2map.Depth, flags ...hmz2map.Flag) *hmz2map.Hex {
		return &hmz2map.Hex{Landform: hmz2map.LandformSaltWater, Surface: hmz2map.SurfaceClear, Biome: hmz2map.BiomeClear, Depth: d, Flags: flags}
	}
	for _, tc := range []struct {
		name string
		h    *hmz2map.Hex
		want color.RGBA
	}{
		{"rolling savanna", land(hmz2map.LandformRollingPlains, hmz2map.SurfaceClear, hmz2map.BiomeSavanna), color.RGBA{0xd9, 0xb4, 0x4a, 0xff}},
		// 0xd9 + (255 − 0xd9)·15/100 = 217 + 5 = 222.
		{"flats savanna", land(hmz2map.LandformFlats, hmz2map.SurfaceClear, hmz2map.BiomeSavanna), color.RGBA{222, 191, 101, 0xff}},
		// 0x1f·75/100 = 23, 0x5f·75/100 = 71.
		{"mountain rainforest", land(hmz2map.LandformMountains, hmz2map.SurfaceClear, hmz2map.BiomeTropicalRainforest), color.RGBA{23, 71, 23, 0xff}},
		// Surface wins over biome: mangroves, lightened 15.
		{"mangroves", land(hmz2map.LandformFlats, hmz2map.SurfaceMangroves, hmz2map.BiomeSavanna, hmz2map.FlagCoast), color.RGBA{38, 174, 147, 0xff}},
		// Hills scrubland 0xc9a86a darkened 12 → (176, 147, 93), then 40% to the cliff red.
		{"impassable hills", land(hmz2map.LandformHills, hmz2map.SurfaceClear, hmz2map.BiomeScrubland, hmz2map.FlagImpassable), color.RGBA{182, 94, 62, 0xff}},
		{"volcano", land(hmz2map.LandformMountains, hmz2map.SurfaceClear, hmz2map.BiomeSteppe, hmz2map.FlagVolcano), VolcanoColor},
		{"inland coast", water(hmz2map.DepthShallow, hmz2map.FlagCoast, hmz2map.FlagInlandSea), InlandSeaColor},
		{"coast", water(hmz2map.DepthShallow, hmz2map.FlagCoast), CoastWaterColor},
		{"open", water(hmz2map.DepthOpen), DepthColors[hmz2map.DepthOpen]},
		{"lake", &hmz2map.Hex{Landform: hmz2map.LandformFreshWater}, FreshWaterColor},
		{"cliffs", &hmz2map.Hex{Landform: hmz2map.LandformCliffs}, CliffsColor},
	} {
		got, err := FillColor(tc.h)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	if _, err := FillColor(land(hmz2map.LandformHills, hmz2map.SurfaceClear, hmz2map.BiomeClear)); err == nil {
		t.Errorf("land with clear surface and biome: no error")
	}
	if _, err := FillColor(&hmz2map.Hex{Landform: "swamp"}); err == nil {
		t.Errorf("unknown landform: no error")
	}
}

func TestRiverWidth(t *testing.T) {
	for _, tc := range []struct {
		apothem int
		s, r, g int
	}{{24, 1, 2, 3}, {20, 1, 2, 3}, {12, 1, 1, 2}, {4, 1, 1, 1}, {48, 2, 4, 6}, {36, 2, 3, 5}} {
		s, _ := RiverWidth(hmz2map.SizeStream, tc.apothem)
		r, _ := RiverWidth(hmz2map.SizeRiver, tc.apothem)
		g, _ := RiverWidth(hmz2map.SizeGreatRiver, tc.apothem)
		if s != tc.s || r != tc.r || g != tc.g {
			t.Errorf("apothem %d: got %d %d %d, want %d %d %d", tc.apothem, s, r, g, tc.s, tc.r, tc.g)
		}
	}
}

// TestRivers checks the draw-once, shore, and water-to-water rules on a
// 3 × 3 map with land at (1, 0) and (1, 1).
func TestRivers(t *testing.T) {
	m := testMap(3, 3)
	mid := m.At(1, 1)
	mid.Landform, mid.Biome, mid.Depth = hmz2map.LandformHills, hmz2map.BiomeSavanna, ""
	top := m.At(1, 0)
	top.Landform, top.Biome, top.Depth = hmz2map.LandformPlains, hmz2map.BiomeSavanna, ""
	river := func(s hmz2map.Side, size hmz2map.RiverSize) hmz2map.River {
		c1, _ := s.Corners()
		return hmz2map.River{Side: s, Flow: c1, Size: size}
	}
	// Land (1, 1)'s n side is (1, 0)'s s side: drawn once, by (1, 1).
	mid.Rivers = []hmz2map.River{river(hmz2map.SideN, hmz2map.SizeGreatRiver)}
	m.At(1, 0).Rivers = []hmz2map.River{river(hmz2map.SideS, hmz2map.SizeGreatRiver)}
	// (0, 0)'s n side is off the map, between water and nothing: skipped.
	m.At(0, 0).Rivers = []hmz2map.River{river(hmz2map.SideN, hmz2map.SizeStream)}
	// (2, 2)'s s side is off the map, drawn by (2, 2), but it's water: skipped.
	// (2, 0)'s sw side is (1, 0)'s ne side, a shore: skipped once.
	m.At(1, 0).Rivers = []hmz2map.River{river(hmz2map.SideNE, hmz2map.SizeRiver), river(hmz2map.SideS, hmz2map.SizeGreatRiver)}
	m.At(2, 0).Rivers = []hmz2map.River{river(hmz2map.SideSW, hmz2map.SizeRiver)}
	m.At(2, 2).Rivers = []hmz2map.River{river(hmz2map.SideS, hmz2map.SizeStream)}

	img, rep, err := Render(m, Options{Apothem: 24, Outlines: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.EdgesDrawn[hmz2map.SizeGreatRiver] != 1 || rep.EdgesDrawn[hmz2map.SizeStream] != 0 || rep.EdgesDrawn[hmz2map.SizeRiver] != 0 || rep.ShoreEdgesSkipped != 1 || rep.WaterEdgesSkipped != 2 {
		t.Errorf("drew %v, skipped %d shore and %d water; want 1 great river, 1 shore, 2 water", rep.EdgesDrawn, rep.ShoreEdgesSkipped, rep.WaterEdgesSkipped)
	}
	// The edge is horizontal at a whole-pixel y, between two rows of pixel
	// centers: a 3-pixel line covers the two rows beside it and the row
	// above them.
	g := NewGeometry(3, 3, 24)
	ax, ay := g.Corner(1, 1, hmz2map.CornerNW)
	bx, _ := g.Corner(1, 1, hmz2map.CornerNE)
	mx, y := int((ax+bx)/2), int(ay)
	for py := y - 4; py < y+4; py++ {
		want := py >= y-2 && py <= y
		if got := img.RGBAAt(mx, py) == RiverColor; got != want {
			t.Errorf("pixel (%d, %d): river %v, want %v", mx, py, got, want)
		}
	}
}

// landMap returns a map of grassland plains.
func landMap(columns, rows int) *hmz2map.Map {
	m := testMap(columns, rows)
	for i := range m.Hexes {
		m.Hexes[i].Landform, m.Hexes[i].Biome, m.Hexes[i].Depth = hmz2map.LandformPlains, hmz2map.BiomeGrassland, ""
	}
	return m
}

// addRiver lists a river on side s of (col, row), flowing to corner flow,
// and on the neighbor across it, as hmz2map does.
func addRiver(m *hmz2map.Map, col, row int, s hmz2map.Side, flow hmz2map.Corner, size hmz2map.RiverSize) {
	m.At(col, row).Rivers = append(m.At(col, row).Rivers, hmz2map.River{Side: s, Flow: flow, Size: size})
	nc, nr := hmz2map.Neighbor(col, row, s)
	o := m.At(nc, nr)
	if o == nil {
		return
	}
	// The neighbor's corners along the edge are this hex's, two steps round.
	c1, _ := s.Corners()
	d := 2
	if flow == c1 {
		d = -2
	}
	for k, c := range hmz2map.Corners {
		if c == flow {
			flow = hmz2map.Corners[(k+d+6)%6]
			break
		}
	}
	o.Rivers = append(o.Rivers, hmz2map.River{Side: s.Opposite(), Flow: flow, Size: size})
}

// TestMouths uses a 5 × 5 land map with a special hex at (2, 2). The edge
// between (2, 1) and (3, 1), (2, 1)'s se side, ends at (2, 2)'s ne corner,
// which is (2, 1)'s se corner.
func TestMouths(t *testing.T) {
	const apothem = 24 // a great river is 3 px wide, its mouth 9 px in radius
	g := NewGeometry(5, 5, apothem)
	vx, vy := g.Corner(2, 1, hmz2map.CornerSE)
	// at returns whether the pixel d px from the vertex toward the center
	// of (col, row) is river.
	at := func(img interface{ RGBAAt(int, int) color.RGBA }, col, row int, d float64) bool {
		cx, cy := g.Center(col, row)
		n := math.Hypot(cx-vx, cy-vy)
		return img.RGBAAt(int(vx+d*(cx-vx)/n), int(vy+d*(cy-vy)/n)) == RiverColor
	}
	type counts struct{ drawn, shore, mouths int }
	for _, tc := range []struct {
		name    string
		special func(h *hmz2map.Hex)
		flow    hmz2map.Corner
		through bool // the river also runs along (2, 2)'s n side and out of its nw corner
		opt     Options
		want    counts
	}{
		{"lake inflow", lake, hmz2map.CornerSE, false, Options{}, counts{1, 0, 1}},
		{"coast inflow", sea, hmz2map.CornerSE, false, Options{}, counts{1, 0, 1}},
		{"lake outflow", lake, hmz2map.CornerE, false, Options{}, counts{1, 0, 0}},
		{"through lake", lake, hmz2map.CornerSE, true, Options{}, counts{2, 1, 1}},
		{"marsh", surface(hmz2map.SurfaceMarshes), hmz2map.CornerSE, true, Options{}, counts{2, 1, 1}},
		{"swamp", surface(hmz2map.SurfaceSwamps), hmz2map.CornerSE, false, Options{}, counts{1, 0, 1}},
		{"mangroves", surface(hmz2map.SurfaceMangroves), hmz2map.CornerSE, false, Options{}, counts{1, 0, 1}},
		{"marsh as land", surface(hmz2map.SurfaceMarshes), hmz2map.CornerSE, true, Options{WetlandsAsLand: true}, counts{3, 0, 0}},
		{"salt flats", surface(hmz2map.SurfaceSaltFlats), hmz2map.CornerSE, true, Options{}, counts{3, 0, 0}},
	} {
		m := landMap(5, 5)
		tc.special(m.At(2, 2))
		addRiver(m, 2, 1, hmz2map.SideSE, tc.flow, hmz2map.SizeGreatRiver)
		if tc.through {
			// Along the shore from (2, 2)'s ne corner to its nw corner,
			// then out between (1, 1) and (2, 1).
			addRiver(m, 2, 2, hmz2map.SideN, hmz2map.CornerNW, hmz2map.SizeGreatRiver)
			addRiver(m, 2, 1, hmz2map.SideSW, hmz2map.CornerW, hmz2map.SizeGreatRiver)
		}
		opt := tc.opt
		opt.Apothem = apothem
		img, rep, err := Render(m, opt)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := counts{rep.EdgesDrawn[hmz2map.SizeGreatRiver], rep.ShoreEdgesSkipped, rep.Mouths[hmz2map.SizeGreatRiver]}
		if got != tc.want {
			t.Errorf("%s: drew %d, skipped %d shore edges, drew %d mouths; want %v", tc.name, got.drawn, got.shore, got.mouths, tc.want)
		}
		mouth := tc.want.mouths > 0
		if got := at(img, 2, 2, 6); got != mouth {
			t.Errorf("%s: 6 px into (2, 2): river %v, want %v", tc.name, got, mouth)
		}
		if at(img, 2, 2, 12) {
			t.Errorf("%s: 12 px into (2, 2): river, want fill", tc.name)
		}
		// The mouth doesn't spill into the land hexes at the vertex.
		if at(img, 3, 1, 6) {
			t.Errorf("%s: 6 px into (3, 1): river, want fill", tc.name)
		}
	}
}

func lake(h *hmz2map.Hex) { h.Landform, h.Biome = hmz2map.LandformFreshWater, hmz2map.BiomeClear }

func sea(h *hmz2map.Hex) {
	h.Landform, h.Biome, h.Depth = hmz2map.LandformSaltWater, hmz2map.BiomeClear, hmz2map.DepthShallow
}

func surface(s hmz2map.Surface) func(*hmz2map.Hex) {
	return func(h *hmz2map.Hex) { h.Surface = s }
}

// TestHorizontalWidth checks that a line along a horizontal side covers
// exactly its width in rows.
func TestHorizontalWidth(t *testing.T) {
	for w := 1; w <= 6; w++ {
		n := 0
		for py := range 20 {
			if onSegment(10.5, float64(py)+0.5, 0, 10, 20, 10, w) {
				n++
			}
		}
		if n != w {
			t.Errorf("width %d: %d rows", w, n)
		}
	}
}

func TestDeterministic(t *testing.T) {
	m := testMap(9, 8)
	for i := range m.Hexes {
		if i%3 == 0 {
			m.Hexes[i].Landform, m.Hexes[i].Biome, m.Hexes[i].Depth = hmz2map.LandformPlains, hmz2map.BiomeGrassland, ""
			m.Hexes[i].Rivers = []hmz2map.River{{Side: hmz2map.SideNE, Flow: hmz2map.CornerE, Size: hmz2map.SizeRiver}}
		}
	}
	encode := func() []byte {
		img, _, err := Render(m, Options{Apothem: 17, Outlines: true})
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if !bytes.Equal(encode(), encode()) {
		t.Error("two renders differ")
	}
}

func TestCheck(t *testing.T) {
	m := testMap(2, 2)
	m.SchemaVersion = 2
	if err := Check(m); err == nil {
		t.Error("schema version 2: no error")
	}
	m = testMap(2, 2)
	m.Hexes[1], m.Hexes[2] = m.Hexes[2], m.Hexes[1]
	if err := Check(m); err == nil {
		t.Error("hexes out of order: no error")
	}
}
