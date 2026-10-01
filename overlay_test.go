// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/maloquacious/hmz2map"
)

func TestParsePosition(t *testing.T) {
	for _, s := range Positions {
		p, err := ParsePosition(s)
		if err != nil {
			t.Errorf("ParsePosition(%q): %v", s, err)
		} else if p.String() != s {
			t.Errorf("ParsePosition(%q).String() = %q", s, p)
		}
	}
	for _, s := range []string{"", "top", "left", "top-center", "upper-left", "top-left-right", "Top-Left"} {
		if _, err := ParsePosition(s); err == nil {
			t.Errorf("ParsePosition(%q) succeeded, want error", s)
		}
	}
}

// overlayMap is a small map of deep water with one savanna hills hex that
// has a stream along its north side.
func overlayMap(columns, rows int) *hmz2map.Map {
	m := testMap(columns, rows)
	h := m.At(2, 2)
	h.Landform, h.Biome, h.Depth = hmz2map.LandformHills, hmz2map.BiomeSavanna, ""
	h.Rivers = []hmz2map.River{{Side: hmz2map.SideN, Flow: hmz2map.CornerNE, DrainageKm2: 60, Size: hmz2map.SizeStream}}
	n := m.At(2, 1)
	n.Landform, n.Biome, n.Depth = hmz2map.LandformPlains, hmz2map.BiomeSavanna, ""
	n.Rivers = []hmz2map.River{{Side: hmz2map.SideS, Flow: hmz2map.CornerSE, DrainageKm2: 60, Size: hmz2map.SizeStream}}
	return m
}

func decorate(t *testing.T, m *hmz2map.Map, apothem int, ov Overlays) (*image.RGBA, *image.RGBA, Placement) {
	t.Helper()
	opt := Options{Apothem: apothem, Outlines: true}
	mapImg, rep, err := Render(m, opt)
	if err != nil {
		t.Fatal(err)
	}
	img, pl, err := Decorate(mapImg, m, rep, opt, ov)
	if err != nil {
		t.Fatal(err)
	}
	return mapImg, img, pl
}

func pos(s string) *Position {
	p, err := ParsePosition(s)
	if err != nil {
		panic(err)
	}
	return &p
}

func TestDecorateWithoutOverlays(t *testing.T) {
	mapImg, img, pl := decorate(t, overlayMap(5, 40), 8, Overlays{})
	if img != mapImg || pl.Map != mapImg.Bounds() || !pl.LeftPanel.Empty() || !pl.RightPanel.Empty() {
		t.Errorf("no overlays: got a new image or panels: %+v", pl)
	}
}

// The map's pixels are the same with panels, shifted right by a left panel,
// and the panels are background outside the overlays.
func TestDecorateKeepsMap(t *testing.T) {
	m := overlayMap(5, 60)
	for _, ov := range []Overlays{
		{Legend: pos("top-left")},
		{Compass: pos("middle-right")},
		{Legend: pos("bottom-right"), Compass: pos("top-right")},
		{Legend: pos("top-left"), Compass: pos("bottom-right")},
	} {
		mapImg, img, pl := decorate(t, m, 8, ov)
		if pl.Map.Dx() != mapImg.Bounds().Dx() || pl.Map.Dy() != mapImg.Bounds().Dy() || pl.Width != img.Bounds().Dx() {
			t.Fatalf("%+v: placement %+v for map %v", ov, pl, mapImg.Bounds())
		}
		if (ov.Legend != nil && ov.Legend.Side == Left || ov.Compass != nil && ov.Compass.Side == Left) != (pl.Map.Min.X > 0) {
			t.Errorf("%+v: map at %v", ov, pl.Map)
		}
		for y := range pl.Height {
			for x := range pl.Width {
				p := image.Pt(x, y)
				switch {
				case p.In(pl.Map):
					if img.RGBAAt(x, y) != mapImg.RGBAAt(x-pl.Map.Min.X, y) {
						t.Fatalf("%+v: map pixel (%d, %d) changed", ov, x-pl.Map.Min.X, y)
					}
				case p.In(pl.Legend), p.In(pl.Compass):
				default:
					if img.RGBAAt(x, y) != BackgroundColor {
						t.Fatalf("%+v: panel pixel (%d, %d) is %v", ov, x, y, img.RGBAAt(x, y))
					}
				}
			}
		}
		for _, r := range []image.Rectangle{pl.Legend, pl.Compass} {
			if !r.Empty() && !r.In(pl.LeftPanel) && !r.In(pl.RightPanel) {
				t.Errorf("%+v: overlay %v outside the panels", ov, r)
			}
		}
	}
}

func TestDecoratePlacement(t *testing.T) {
	_, _, pl := decorate(t, overlayMap(5, 60), 8, Overlays{Legend: pos("bottom-right"), Compass: pos("top-right")})
	met, err := newMetrics(8, true)
	if err != nil {
		t.Fatal(err)
	}
	if pl.Compass.Min.Y != met.marginY || pl.Legend.Max.Y != pl.Height-met.marginY {
		t.Errorf("compass at %v, legend at %v; want margins of %d px", pl.Compass, pl.Legend, met.marginY)
	}
	if pl.Compass.Min.X != pl.Map.Max.X+met.marginX || pl.Legend.Min.X != pl.Compass.Min.X {
		t.Errorf("compass at %v, legend at %v; want %d px right of the map", pl.Compass, pl.Legend, met.marginX)
	}
	if want := max(pl.Legend.Dx(), pl.Compass.Dx()) + 2*met.marginX; pl.RightPanel.Dx() != want {
		t.Errorf("right panel %d px wide, want %d", pl.RightPanel.Dx(), want)
	}
	_, _, pl = decorate(t, overlayMap(5, 60), 8, Overlays{Compass: pos("middle-left")})
	if c := (pl.Compass.Min.Y + pl.Compass.Max.Y) / 2; c < pl.Height/2-1 || c > pl.Height/2+1 {
		t.Errorf("middle compass centered at y %d, image %d tall", c, pl.Height)
	}
}

func TestDecorateErrors(t *testing.T) {
	opt := Options{Apothem: 8, Outlines: true}
	for _, tc := range []struct {
		rows int
		ov   Overlays
		want string
	}{
		{60, Overlays{Legend: pos("top-left"), Compass: pos("top-left")}, "both at top-left"},
		{8, Overlays{Legend: pos("top-right")}, "tall"},
		{18, Overlays{Legend: pos("top-left"), Compass: pos("bottom-left")}, "overlap"},
	} {
		m := overlayMap(5, tc.rows)
		mapImg, rep, err := Render(m, opt)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Decorate(mapImg, m, rep, opt, tc.ov); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%d rows, %+v: error %v, want %q", tc.rows, tc.ov, err, tc.want)
		}
	}
}

func TestLegendListsOnlyColorsOnTheMap(t *testing.T) {
	m := overlayMap(5, 40)
	m.At(0, 5).Landform, m.At(0, 5).Depth = hmz2map.LandformCliffs, ""
	v := m.At(3, 5)
	v.Landform, v.Biome, v.Depth, v.Flags = hmz2map.LandformMountains, hmz2map.BiomeSavanna, "", []hmz2map.Flag{hmz2map.FlagVolcano}
	m.At(2, 1).Flags = []hmz2map.Flag{hmz2map.FlagImpassable}
	_, rep, err := Render(m, Options{Apothem: 8})
	if err != nil {
		t.Fatal(err)
	}
	met, err := newMetrics(8, true)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, col := range newLegend(m, rep, met).columns {
		for _, sec := range col {
			for _, e := range sec.entries {
				got = append(got, sec.title+":"+e.label)
			}
		}
	}
	// The volcano's mountains are drawn magenta, so they add no relief
	// entry; there are no badlands.
	want := []string{"Water:deep water", "Impassable:cliffs", "Impassable:impassable land", "Features:volcano",
		"Biome:savanna", "Relief:plains", "Relief:hills", "Rivers:stream"}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("legend %v, want %v", got, want)
	}
}

// A flat-top compass's arrows point at the six neighbors.
func TestCompassArrowsPointAtNeighbors(t *testing.T) {
	g := NewGeometry(5, 5, 24)
	arrows := compassArrows(FlatTop)
	if len(arrows) != 6 {
		t.Fatalf("%d flat-top arrows, want 6", len(arrows))
	}
	for _, col := range []int{2, 3} {
		x0, y0 := g.Center(col, 2)
		for _, s := range hmz2map.Sides {
			name := strings.ToUpper(string(s))
			b, ok := arrows[name]
			if !ok {
				t.Fatalf("no arrow for %s", name)
			}
			x1, y1 := g.Center(hmz2map.Neighbor(col, 2, s))
			want := math.Mod(math.Atan2(x1-x0, y0-y1)*180/math.Pi+360, 360)
			if math.Abs(b-want) > 1e-9 {
				t.Errorf("column %d: arrow %s at %v°, neighbor at %v°", col, name, b, want)
			}
		}
	}
	pointy := compassArrows(PointyTop)
	for _, name := range []string{"NE", "E", "SE", "SW", "W", "NW"} {
		if _, ok := pointy[name]; !ok {
			t.Errorf("pointy-top compass lacks an arrow for %s", name)
		}
	}
	if len(pointy) != 6 {
		t.Errorf("%d pointy-top arrows, want 6", len(pointy))
	}
}

func TestOrientationOf(t *testing.T) {
	if o, err := OrientationOf(hmz2map.Layout); err != nil || o != FlatTop {
		t.Errorf("OrientationOf(%q) = %v, %v", hmz2map.Layout, o, err)
	}
	if o, err := OrientationOf("pointy-top, 0-based, odd rows right"); err != nil || o != PointyTop {
		t.Errorf("pointy-top layout: %v, %v", o, err)
	}
	if _, err := OrientationOf("square"); err == nil {
		t.Error("OrientationOf(square) succeeded")
	}
}

// The pointy-top compass draws, though no map has that layout yet.
func TestPointyCompassDraws(t *testing.T) {
	met, err := newMetrics(8, true)
	if err != nil {
		t.Fatal(err)
	}
	size := compassSize(met, PointyTop)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	drawCompass(img, met, PointyTop, 0, 0, size)
	if c := img.RGBAAt(size/2, size/2); c != CompassHexColor {
		t.Errorf("compass center is %v, want the hex color", c)
	}
}

func TestDecorateIsDeterministic(t *testing.T) {
	m := overlayMap(5, 60)
	ov := Overlays{Legend: pos("bottom-right"), Compass: pos("top-left")}
	var out [2][]byte
	for i := range out {
		_, img, _ := decorate(t, m, 8, ov)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		out[i] = buf.Bytes()
	}
	if !bytes.Equal(out[0], out[1]) {
		t.Error("two renders differ")
	}
}
