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

| Apothem | 107 × 222 map | 115 × 230 map (`-border`) |
| ------: | ------------: | ------------------------: |
|   12 px | 2,231 × 5,340 |             2,398 × 5,532 |
|   16 px | 2,975 × 7,120 |             3,197 × 7,376 |
|   20 px | 3,719 × 8,900 |             3,996 × 9,220 |
|   24 px | 4,462 × 10,680 |           4,795 × 11,064 |

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
There's no anti-aliasing: every pixel is exactly one of these colors.

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

## Output

The PNG is 8-bit RGB, written with Go's `image/png` at its default compression (it drops the alpha channel of an opaque image).
The same input and flags give the same bytes: hexes and their rivers are drawn in the map's order, nothing is taken from a Go map's iteration order, and no time or path is embedded.

## Results

On `hmz2map` v0.1.0's Panama maps at the default apothem (24), with outlines:

| Measurement | Without border | With `-border` |
| ----------- | -------------: | -------------: |
| Map | 107 × 222 | 115 × 230 |
| Image | 4,462 × 10,680 px | 4,795 × 11,064 px |
| Pixels in hexes | 47,397,000 | 52,776,260 |
| Background pixels | 257,160 | 275,620 |
| Ties | 0 | 0 |
| Outline pixels | 2,102,447 | 2,341,224 |
| River pixels | 161,650 | 161,547 |
| PNG | 2.7 MB | 2.9 MB |
| Time | 1.6 s | 1.5 s |
| Peak memory | 460 MB | 515 MB |

- River edges drawn: 2,759 `stream` (1 px), 1,246 `river` (2 px), and 138 `great-river` (3 px); 348 shore edges and 176 water edges are skipped. Mouths drawn: 102 `stream`, 68 `river`, and 16 `great-river`. The border changes none of these.
- With `-wetlands-as-land`: 2,866 `stream`, 1,333 `river`, and 164 `great-river` edges drawn; 231 shore edges (154 on the coast, 77 on lake shores) and 73 water edges skipped; 84, 57, and 12 mouths; 170,595 river pixels without the border. The river-pixel counts differ slightly because the border shifts the map 4 columns, 6·s pixels, which isn't a whole number, so slanted sides cross the pixel grid differently.
- No pixel center is in two hexes at this apothem, so the tie rule decides nothing here.

Every pixel of both images, and of renders at apothems 4, 12, 13, and 37, with and without outlines, was cross-checked against an independent Python calculation written from the v0.1.0 README, with no mismatches. The shore-edge and mouth rules of v0.2.0 haven't been cross-checked yet.
- About a third of the time is rendering and two-thirds PNG encoding. Memory is the image (4 bytes a pixel), the pixel-to-hex table (4 bytes a pixel), and a river mask (1 byte a pixel): about 9 bytes a pixel, so it grows with the square of the apothem.

## License

MIT. See `LICENSE`.
