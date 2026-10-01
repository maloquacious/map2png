// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command map2png renders an hmz2map hex map as a PNG.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"strings"
	"time"

	"github.com/maloquacious/hmz2map"
	"github.com/maloquacious/map2png"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "map2png: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("map2png", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: map2png [flags] <map.json>\n\n")
		fs.PrintDefaults()
	}
	output := fs.String("output", "", "PNG file to write (required)")
	apothem := fs.Int("apothem", 24, fmt.Sprintf("hex apothem in whole pixels, at least %d", map2png.MinApothem))
	outlines := fs.Bool("outlines", true, "draw hex outlines")
	wetlandsAsLand := fs.Bool("wetlands-as-land", false, "treat marshes, swamps, and mangroves as land for rivers")
	legend := fs.String("legend", "", "add a legend at `position`: "+strings.Join(map2png.Positions, ", "))
	compass := fs.String("compass", "", "add a compass at `position`: "+strings.Join(map2png.Positions, ", "))
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, map2png.Version())
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one input file, got %d", fs.NArg())
	}
	if *output == "" {
		return fmt.Errorf("-output is required")
	}
	if *apothem < map2png.MinApothem {
		return fmt.Errorf("-apothem %d is below the minimum of %d", *apothem, map2png.MinApothem)
	}
	var overlays map2png.Overlays
	for _, o := range []struct {
		name, value string
		pos         **map2png.Position
	}{{"legend", *legend, &overlays.Legend}, {"compass", *compass, &overlays.Compass}} {
		if o.value == "" {
			continue
		}
		p, err := map2png.ParsePosition(o.value)
		if err != nil {
			return fmt.Errorf("-%s: %w", o.name, err)
		}
		*o.pos = &p
	}
	if overlays.Legend != nil && overlays.Compass != nil && *overlays.Legend == *overlays.Compass {
		return fmt.Errorf("-legend and -compass are both %s", overlays.Legend)
	}
	input := fs.Arg(0)
	start := time.Now()
	phase := func(name string) {
		fmt.Fprintf(stdout, "%-24s %6.1fs\n", name, time.Since(start).Seconds())
	}

	var m hmz2map.Map
	if err := readJSON(input, &m); err != nil {
		return err
	}
	if err := map2png.Check(&m); err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}
	phase("read input")
	opt := map2png.Options{Apothem: *apothem, Outlines: *outlines, WetlandsAsLand: *wetlandsAsLand}
	mapImg, rep, err := map2png.Render(&m, opt)
	if err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}
	img, pl, err := map2png.Decorate(mapImg, &m, rep, opt, overlays)
	if err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}
	phase("render")
	if err := writeFile(*output, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
		return err
	}
	phase("write output")

	fmt.Fprintf(stdout, "map:              %d × %d, border %d\n", m.Columns, m.Rows, m.Border)
	fmt.Fprintf(stdout, "image:            %d × %d px, apothem %d\n", pl.Width, pl.Height, *apothem)
	if pl.Width != rep.Width {
		fmt.Fprintf(stdout, "map area:         %v\n", pl.Map)
	}
	if overlays.Legend != nil {
		fmt.Fprintf(stdout, "legend:           %s, %v\n", overlays.Legend, pl.Legend)
	}
	if overlays.Compass != nil {
		fmt.Fprintf(stdout, "compass:          %s, %v\n", overlays.Compass, pl.Compass)
	}
	fmt.Fprintf(stdout, "pixels:           %d in hexes, %d background, %d ties\n", rep.HexPixels, rep.BackgroundPixels, rep.Ties)
	fmt.Fprintf(stdout, "outline pixels:   %d\n", rep.OutlinePixels)
	fmt.Fprintf(stdout, "river edges:      ")
	for _, s := range hmz2map.RiverSizes {
		w, _ := map2png.RiverWidth(s, *apothem)
		fmt.Fprintf(stdout, "%d %s (%d px), ", rep.EdgesDrawn[s], s, w)
	}
	fmt.Fprintf(stdout, "%d skipped along shores, %d skipped with no land\n", rep.ShoreEdgesSkipped, rep.WaterEdgesSkipped)
	fmt.Fprintf(stdout, "river mouths:     ")
	for i, s := range hmz2map.RiverSizes {
		if i > 0 {
			fmt.Fprintf(stdout, ", ")
		}
		fmt.Fprintf(stdout, "%d %s", rep.Mouths[s], s)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "river pixels:     %d\n", rep.RiverPixels)
	return nil
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := write(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	return f.Close()
}
