# map2png

`map2png` is a map converter: it reads a hex map written by [`hmz2map`](https://github.com/maloquacious/hmz2map) and renders it as a PNG for players and the referee.
It isn't part of the campaign-map pipeline; it only consumes the map.

The map is decoded with `hmz2map`'s own Go types, so the schema is `hmz2map`'s.
`map2png` reads nothing else: not the heightmap, the climate file, or the rivers file.
Hex geometry is computed from the map's coordinates alone.

## Usage

```text
go run ./cmd/map2png [flags] <map.json>
```

Flags:

- `-output <file>` is the PNG file to write. Required.
- `-apothem <px>` is a hex's apothem, the distance from its center to the middle of a side, in whole pixels. The default is `24`; the smallest is `4`.
- `-outlines` draws hex outlines (see [Outlines](#outlines)). The default is on; `-outlines=false` turns them off.
- `-wetlands-as-land` treats hexes with a `marshes`, `swamps`, or `mangroves` surface as land for rivers (see [Rivers](#rivers)). The default is off: they count as water, so no river is drawn along their sides.
- `-legend <position>` adds a legend of the colors on the map, and `-compass <position>` a compass (see [Legend and compass](#legend-and-compass)). A position is `top-left`, `middle-left`, `bottom-left`, `top-right`, `middle-right`, or `bottom-right`. Both are off by default. Giving both the same position is an error.
- `-version` prints the version.

The command prints the time taken by each phase and the counts it drew.

`map2png` has no border option: it draws whatever map it is given.
For a map with a deep-ocean border, give it the map from `hmz2map -border`.

### Input

The input must have `schema_version` 1 (`hmz2map.SchemaVersion`) and `layout` `"flat-top, 0-based, odd columns down"` (`hmz2map.Layout`); any other value is an error.
It must list `columns × rows` hexes, ordered by row, then column, with `col` and `row` matching each position.
A landform, surface, biome, or depth outside the terrain model is an error, since it has no color.
`map2png` doesn't otherwise validate the map; `hmz2map` did.

## Geometry

Hexes are flat-top, with **odd columns pushed down** half a hex, as in `hmz2map`.
Image coordinates are in pixels, with x to the right and y down; pixel (`px`, `py`) covers the square from (`px`, `py`) to (`px` + 1, `py` + 1), so its center is (`px` + 0.5, `py` + 0.5).

With apothem `a` and side `s = 2a/√3`, the center of hex (`c`, `r`) is:

```text
x = s + 1.5·s·c
y = a + 2a·r + (a if c is odd)
```

Its corners are at these offsets from the center, named as in `hmz2map` (clockwise from east, y down):

| Corner | Offset          |
| ------ | --------------- |
| `e`    | (s, 0)          |
| `se`   | (s/2, a)        |
| `sw`   | (−s/2, a)       |
| `w`    | (−s, 0)         |
| `nw`   | (−s/2, −a)      |
| `ne`   | (s/2, −a)       |

Side `n` runs from corner `nw` to `ne`, `ne` from `ne` to `e`, and so on (`hmz2map`'s `Side.Corners`).

A map of `C` columns and `R` rows makes an image of:

```text
width  = ⌈2s + 1.5·s·(C − 1)⌉
height = 2a·R + a
```

The last column's east corners are at x = 2s + 1.5·s·(C − 1), and the odd columns reach half a hex lower than the even ones.

| Apothem | 106 × 222 map | 114 × 230 map (`-border`) |
| ------: | ------------: | ------------------------: |
|   12 px | 2,211 × 5,340 |             2,377 × 5,532 |
|   16 px | 2,947 × 7,120 |             3,169 × 7,376 |
|   20 px | 3,684 × 8,900 |             3,961 × 9,220 |
|   24 px | 4,421 × 10,680 |           4,753 × 11,064 |

### Pixel ownership

A point's **hex distance** from the center (`x₀`, `y₀`) of a hex is:

```text
dx = |x − x₀|,  dy = |y − y₀|
d  = max(dy, (√3·dx + dy) / 2) / a
```

The hex, sides included, is the set of points with `d ≤ 1`.
To keep floating-point noise out of the result, `d` is **snapped to 6 decimals** before it is compared: `⌊d·10⁶ + 0.5⌋`, an integer count of millionths.

A pixel belongs to a hex when the snapped hex distance of its center from that hex's center is at most 10⁶.
Neighboring hexes share their sides, so a pixel center on (or, after snapping, within half a millionth of) a shared side or corner is in two or three hexes.
The **tie rule**: it belongs to the one with the lowest index, `row × columns + col`, that is, the one in the upper row, or the left column in the same row.
A pixel in no hex is **background**.

Because `a` is a whole number and pixel centers are at half-pixels, no pixel center lies on a horizontal side; ties can only happen on the slanted sides, where the snapping makes a pixel center within half a millionth of a side count as on it. The Panama maps have none at the default apothem, but they do happen: at apothem 37 there are 443 (459 with the border), each in two hexes.

## Colors

Every pixel of a hex has the hex's **fill color**, except its outline pixels (see [Outlines](#outlines)) and the pixels of river lines drawn over it.
There's no anti-aliasing: every pixel of the map is exactly one of these colors.
This applies to the map area only; the legend and compass (see [Legend and compass](#legend-and-compass)) have anti-aliased text.

### Base colors

| Hex | Color |
| --- | ----- |
| Background (pixel in no hex) | `#ffffff` |
| Salt water with the `inland-sea` flag | `#2aa098` |
| Other salt water with the `coast` flag | `#8ec9e8` |
| Other salt water, `shallow` | `#4f93c4` |
| Other salt water, `open` | `#2b578c` |
| Other salt water, `deep` (the border too) | `#142c55` |
| Fresh water (lakes) | `#1f78d1` |
| Cliffs | `#c01010` |
| Badlands | `#702040` |
| Land with the `volcano` flag | `#ff00c8` |
| Other land | its surface's color below, or its biome's if the surface is `clear`, then tinted (below) |

The salt-water rows are in precedence order: an inland sea is teal even where it has the `coast` flag.

| Surface | Color |
| ------- | ----- |
| `glacial-ice` | `#e8f4ff` |
| `marshes` | `#7ad0c0` |
| `swamps` | `#3c8c7c` |
| `bogs` | `#8a7a9a` |
| `mangroves` | `#00a080` |
| `salt-flats` | `#fff0f5` |

| Biome | Color |
| ----- | ----- |
| `tundra` | `#9fb3a6` |
| `alpine` | `#bdb6ad` |
| `desert` | `#f2e2a8` |
| `scrubland` | `#c9a86a` |
| `grassland` | `#a9cf6e` |
| `steppe` | `#d6cf98` |
| `savanna` | `#d9b44a` |
| `boreal-forest` | `#3f6a5a` |
| `temperate-forest` | `#4f8f3a` |
| `temperate-rainforest` | `#2f6f4f` |
| `tropical-dry-forest` | `#8faa3c` |
| `tropical-rainforest` | `#1f5f1f` |
| `cloud-forest` | `#5a8f9a` |

These are `hmz2bio`'s preview colors.

### Land tint

Land (the seven relief landforms, `flats` through `volcanic-highlands`) without the `volcano` flag shows its landform as a tint of its surface or biome color: lighter for low relief, darker for high.

| Landform | Tint |
| -------- | ---: |
| `flats` | lighten 15 |
| `plains` | lighten 8 |
| `rolling-plains` | none |
| `hills` | darken 12 |
| `mountains` | darken 25 |
| `plateaus` | darken 8 |
| `volcanic-highlands` | darken 18 |

Each channel `v` (0–255) of the color is changed with integer arithmetic, dividing with truncation:

```text
lighten t:  v + (255 − v)·t / 100
darken t:   v·(100 − t) / 100
```

Land with the `impassable` flag (the land along a border cut) is then blended 40% toward the cliff color, channel by channel: `(v·60 + cliff·40) / 100`, where `cliff` is the cliff color's channel.

The `volcano` color replaces all of this: a volcano is magenta whatever its surface, biome, landform, or flags.

## Outlines

With `-outlines` (the default), a hex's **outline pixels** are the pixels it owns whose right neighbor (`px` + 1, `py`) or lower neighbor (`px`, `py` + 1) it doesn't own, where that neighbor is in the image: another hex's pixel, or background.
An outline pixel's color is the fill color with each channel multiplied by 3/4, truncated: `v·3 / 4`.

So each shared side is drawn once, as a 1-pixel line on the hex above or to the left of it, and the map's own right and bottom edges are outlined but its top and left edges aren't.

## Rivers

Rivers are drawn from the hexes' `rivers` lists, in `#103ac8`, over the fills and outlines.

**Each edge is considered once.** A hex draws its rivers on sides `n`, `ne`, and `se`; it draws those on sides `s`, `sw`, and `nw` only when the neighbor across that side (`hmz2map.Neighbor`) is off the map.
The hex across an `n`, `ne`, or `se` side lists the same edge as its `s`, `sw`, or `nw` side, so every edge is drawn (or skipped) by exactly one hex.

**Wet hexes.** For rivers, a hex is **wet** if its landform is `salt-water` or `fresh-water`, or its surface is `marshes`, `swamps`, or `mangroves`; every other hex is **land**.
With `-wetlands-as-land`, only `salt-water` and `fresh-water` hexes are wet.
Cliffs, badlands, and hexes with a `bogs`, `glacial-ice`, or `salt-flats` surface are always land.
A neighbor off the map is neither.

**Only edges between land are drawn.** Rivers flow into and out of lakes and the sea, not along their shores, so an edge is drawn only if neither of its hexes is wet.
An edge between a land hex and a neighbor off the map is drawn.
An edge with a wet hex on one side and land on the other is a **shore edge**; the rest (two wet hexes, or a wet hex and one off the map) are **water edges**. Neither is drawn.
A river whose edges are all shore or water edges isn't drawn at all.

**Mouths.** Where water flows into a wet hex, a mouth is drawn: the part of a disc centered on the vertex that lies in pixels owned by the wet hex.
A drawn edge's downstream vertex is its hex's `flow` corner, which must be one of the side's two corners (any other value is an error).
That vertex is shared by the edge's two hexes and a third, the neighbor across the hex's other side at that corner: for side `n`, the third hex at corner `ne` is the one across side `ne`, and at corner `nw` the one across side `nw`; in general, the side one step clockwise at the side's clockwise corner, and one step counterclockwise at the other.
If the third hex is on the map and wet, the edge has a mouth.
Since both of the edge's hexes are land, the third hex is the only wet one that can be at the vertex, so the choice doesn't depend on the drawing order, and a vertex has at most one mouth.

A mouth's pixels are those **owned by the wet hex** (see [Pixel ownership](#pixel-ownership)) whose center is less than `r` from the vertex, where `r = 3w` and `w` is the width of the edge that reaches it; the distance is snapped to 6 decimals, and a pixel exactly at `r` counts only if its center is above the vertex, or level with it and to its left, as for edges.
Because the vertex is a corner of the wet hex, the mouth is a 120° sector of the disc.
It has the river color, like the edge.

Nothing extra is drawn where a river flows **out** of a wet hex: the edge simply starts at the shore.

A river edge is the segment between the two corners of its side.
A pixel is part of the river line when the distance from its center to the segment's nearest point, snapped to 6 decimals as for [pixel ownership](#pixel-ownership), is less than `w/2`, where `w` is the line's width.
A pixel exactly at `w/2` (after snapping) is part of the line only if its center is **above** that nearest point (smaller y), or level with it and to its **left**.
This half-open rule matters on the `n` and `s` sides, which are horizontal at a whole-pixel y, halfway between two rows of pixel centers: without it, a 1-pixel stream would cover both rows.
With it, a line of width `w` along a horizontal side covers exactly `w` rows, the extra row going above the side when `w` is odd.


| Size | Width at apothem 24 | Width at apothem `a` |
| ---- | ------------------: | -------------------- |
| `stream` | 1 px | max(1, ⌊1·a/24 + 0.5⌋) |
| `river` | 2 px | max(1, ⌊2·a/24 + 0.5⌋) |
| `great-river` | 3 px | max(1, ⌊3·a/24 + 0.5⌋) |

| Size | Mouth radius at apothem 24 |
| ---- | -------------------------: |
| `stream` | 3 px |
| `river` | 6 px |
| `great-river` | 9 px |

The distance to a segment is the distance to its nearest point, so line ends are rounded and consecutive edges join without gaps.
Every river pixel has the same color, so the order in which lines are drawn doesn't matter.
A river line can also cover pixels outside every hex (background) at the map's edge.

## Legend and compass

`-legend` and `-compass` draw in **panels** added beside the map; nothing is drawn over it.
A position's side picks the panel: `left` adds a panel west of the map and shifts the map right by the panel's width, and `right` adds one east of it.
A legend and a compass on the same side share one panel.
Panels are as tall as the map and filled with the background color.
Without either flag, the image is the map alone, byte for byte as before.

All sizes come from the map's apothem `a` (`-apothem`):

| Size | Value |
| ---- | ----- |
| Swatch and compass hex apothem `l` | `2a` |
| Swatch hex side `sₗ` | `2l / √3` |
| Swatch width | `⌈2·sₗ⌉` |
| Pad `p` | `l / 2`, truncated |
| Horizontal margin `mx` | `⌈8a / √3⌉`: two map hexes wide |
| Vertical margin `my` | `4a`: two map hexes tall |
| Text | Go's `goregular` font at `l` px (72 DPI, no hinting), color `#202020` |

A text's width is the sum of its glyphs' advance widths, each rounded to the nearest 1/64 px, halves up (Go's 26.6 fixed point), with the sum rounded up to a whole pixel; goregular has no kerning. A text's **cap-height center** is the point where the middle of a capital letter sits: the baseline is that `y` plus half the font's cap height (rounded, then halved with truncation).

### Panels

A panel is `mx + w + mx` wide, where `w` is the widest overlay on that side.
An overlay starts `mx` right of the image's left edge in a left panel, or `mx` right of the map's right edge in a right panel; two overlays on one side are left-aligned.
Vertically, for an overlay `h` tall in a map `H` tall:
- `top` puts its top at `my`;
- `middle` at `⌊(H − h) / 2⌋`;
- `bottom` at `H − my − h`.

It's an error if an overlay is taller than `H − 2·my`, or if two overlays on one side come within `my` of each other.

### Legend

The legend lists only the colors that appear on the map, in this README's [Colors](#colors) order, in sections that are left out when empty:

| Column | Section | Entries |
| ------ | ------- | ------- |
| 1 | Water | inland sea, coast water, shallow water, open water, deep water, lake |
| 1 | Impassable | cliffs, badlands, then `impassable land` if any non-volcano land hex has the `impassable` flag |
| 1 | Features | volcano |
| 1 | Surface | each surface used by a non-volcano land hex |
| 1 | Biome | each biome used by a non-volcano land hex with a `clear` surface |
| 2 | Relief | each landform of a non-volcano land hex |
| 2 | Rivers | each river size with at least one edge drawn |

Section titles are the Section names above.
A salt-water hex counts toward the one color it's drawn in, so a coast hex adds "coast water" and not its depth band.
Labels are the names with `-` replaced by a space.

The two kinds of entry:
- **Base colors** have a hex swatch in their color.
- **Relief swatches** are the reference gray `#a8a8a8` with the landform's tint (see [Land tint](#land-tint)); `impassable land` is that gray blended 40% toward the cliff color, so it sits beside cliffs.

Layout, from the legend's top-left corner:
- **Section title:** starts at the column's left edge, with its cap-height center `l/2` (truncated) below the title row's top. The title row is `l + p` tall.
- **Entry row:** `2l + p` tall.
  - Hex swatch: centered `sₗ` right of the column's left edge and `l` below the row's top, with apothem `l`, in the map's orientation. A swatch pixel is a pixel whose center is in that hex under the [pixel ownership](#pixel-ownership) distance rule. With `-outlines`, a swatch pixel whose right or lower neighbor isn't a swatch pixel gets the outline color.
  - Rivers: instead of a hex, a line from the swatch's west to east corner (through its center), in the river color, `RiverWidth(size, l)` pixels wide (twice the map's width), under the [river line rule](#rivers).
  - Label: starts `swatch width + p` right of the column's left edge, with its cap-height center `l` below the row's top.
- **Between sections:** `l`.
- **Columns:** a column is as wide as its widest title or `swatch width + p + label width`. Column 2 starts `2l` right of column 1's right edge.
- **Size:** the legend is as wide as its columns, plus the `2l` gap only when column 2 has a section, and as tall as its taller column.

### Compass

The compass shows map north and the eight directions N, NE, E, SE, S, SW, W, NW around a hex in the map's orientation, filled `#e8e8e8` and always outlined.
Arrows point to the six directions that lead to a neighbor:

| Orientation | Arrows (bearing, degrees clockwise from north) | Labels only |
| ----------- | --------------------------------------------- | ----------- |
| flat-top (every map today) | N 0, NE 60, SE 120, S 180, SW 240, NW 300 | E, W |
| pointy-top | NE 30, E 90, SE 150, SW 210, W 270, NW 330 | N, S |

The pointy-top diagonals aren't 45° bearings, but they are the names players use for those neighbors.
The orientation comes from the map's `layout`.

From the compass's center, the center of its square:
- **Arrow shaft:** runs along its bearing from `1.25·l` to `1.75·l`, `max(2, l/8)` pixels wide (truncated), under the river line rule.
- **Arrow head:** a triangle with its tip at `2.25·l` and a base `l/2` wide at `1.75·l`; a pixel is in it if its center is inside or on an edge.
- **Labels:** centered at `3.25·l` along the arrow's bearing, or along the label's own bearing (a multiple of 45°) if it has no arrow. With the point rounded to whole pixels, halves away from zero, `(X, Y)`, a label `w` wide starts at `X − ⌊w/2⌋` with its cap-height center at `Y`.
- **Colors:** arrows and labels are `#303030`, except N, which is `#c01010`.

The compass is a square of side `2·(⌈h⌉ + p)`, where `h` is the largest of `|sin b|·3.25·l + w/2` and `|cos b|·3.25·l + c/2` over the labels. Here `b` is the bearing the label is placed at, `w` its width, and `c` the cap height rounded to a whole pixel.

When any panel is added, the command prints the map's rectangle in the image and the legend's and compass's rectangles.

## Output

The PNG is 8-bit RGB, written with Go's `image/png` at its default compression (it drops the alpha channel of an opaque image).
The same input and flags give the same bytes: hexes and their rivers are drawn in the map's order, nothing is taken from a Go map's iteration order, no time or path is embedded, and the legend's font is built into the program.

## Results

On `hmz2map` v0.2.0's Panama maps (rivers from `hmz2riv` v0.3.0) at the default apothem (24), with outlines:

| Measurement | Without border | With `-border` |
| ----------- | -------------: | -------------: |
| Map | 106 × 222 | 114 × 230 |
| Image | 4,421 × 10,680 px | 4,753 × 11,064 px |
| Pixels in hexes | 46,954,332 | 52,317,180 |
| Background pixels | 261,948 | 270,012 |
| Ties | 0 | 0 |
| Outline pixels | 2,082,911 | 2,320,754 |
| River pixels | 159,685 | 159,681 |
| PNG | 2.7 MB | 2.9 MB |
| Time | 1.7 s | 1.4 s |
| Peak memory | 456 MB | 505 MB |

- River edges drawn: 2,724 `stream` (1 px), 1,225 `river` (2 px), and 144 `great-river` (3 px); 358 shore edges and 153 water edges are skipped. Mouths drawn: 99 `stream`, 66 `river`, and 14 `great-river`. The border changes none of these.
- With `-wetlands-as-land`: 2,842 `stream`, 1,307 `river`, and 169 `great-river` edges drawn; 220 shore edges (137 on the coast, 83 on lake shores) and 66 water edges skipped; 87, 54, and 11 mouths; 168,683 river pixels without the border and 168,681 with it. The river-pixel counts differ slightly because the border shifts the map 4 columns, 6·s pixels, which isn't a whole number, so slanted sides cross the pixel grid differently.
- No pixel center is in two hexes at this apothem, so the tie rule decides nothing here.
- With `-compass top-right -legend bottom-right`:
  - The image is 5,783 × 10,680 px: a right panel 1,362 px wide.
  - The compass is a 406 px square at (4532, 96).
  - The legend is 1,140 × 3,312 px at (4532, 7272) and lists 33 entries: 6 water, 2 impassable (cliffs and impassable land; Panama has no badlands), 1 feature, 4 surfaces, 10 biomes, 7 relief, and 3 river sizes.

Every pixel of both images, of the `-wetlands-as-land` renders, and of renders at apothems 4, 12, 13, and 37, with and without outlines, was cross-checked against an independent Python calculation written from this README, with no mismatches.
- About a third of the time is rendering and two-thirds PNG encoding. Memory is the image (4 bytes a pixel), the pixel-to-hex table (4 bytes a pixel), and a river mask (1 byte a pixel): about 9 bytes a pixel, so it grows with the square of the apothem.

## License

MIT. See `LICENSE`.
