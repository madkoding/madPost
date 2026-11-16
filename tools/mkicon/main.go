// mkicon dibuja el ícono de madPost y genera:
//
//   - icon.ico                 (para usarlo donde quieras)
//   - rsrc_windows_amd64.syso  (recurso que `go build` incrusta en el .exe)
//
// Uso (desde la raíz del proyecto):  go run ./tools/mkicon
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

var sizes = []int{16, 20, 24, 32, 40, 48, 64, 256}

func main() {
	var images [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, draw(s)); err != nil {
			log.Fatal(err)
		}
		images = append(images, buf.Bytes())
	}
	must(os.WriteFile("icon.ico", buildICO(images), 0o644))
	must(os.WriteFile("rsrc_windows_amd64.syso", buildSyso(images), 0o644))
	log.Println("icon.ico y rsrc_windows_amd64.syso generados")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// ---------- dibujo ----------

type rgba struct{ r, g, b, a float64 } // premultiplicado, 0..1

func over(dst, src rgba) rgba {
	k := 1 - src.a
	return rgba{src.r + dst.r*k, src.g + dst.g*k, src.b + dst.b*k, src.a + dst.a*k}
}

func solid(r, g, b uint8, a float64) rgba {
	return rgba{float64(r) / 255 * a, float64(g) / 255 * a, float64(b) / 255 * a, a}
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func mix(c1, c2 [3]uint8, t float64) (uint8, uint8, uint8) {
	return uint8(lerp(float64(c1[0]), float64(c2[0]), t)),
		uint8(lerp(float64(c1[1]), float64(c2[1]), t)),
		uint8(lerp(float64(c1[2]), float64(c2[2]), t))
}

func smoothstep(e0, e1, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-e0)/(e1-e0)))
	return t * t * (3 - 2*t)
}

func segDist(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	t := math.Max(0, math.Min(1, ((px-ax)*vx+(py-ay)*vy)/(vx*vx+vy*vy)))
	return math.Hypot(px-(ax+t*vx), py-(ay+t*vy))
}

// Geometría del post-it en coordenadas 0..1 (antes de girarlo).
const (
	x0, y0, x1, y1 = 0.13, 0.11, 0.87, 0.85
	fold           = 0.22 // lado de la esquina doblada
	tilt           = -7.0 * math.Pi / 180
)

// sample devuelve el color en el punto (x, y) del ícono de tamaño size.
func sample(x, y float64, size int) rgba {
	// Giramos el punto al revés para dibujar el papel ya inclinado.
	cx, cy := 0.5, 0.5
	sin, cos := math.Sincos(-tilt)
	ux := cos*(x-cx) - sin*(y-cy) + cx
	uy := sin*(x-cx) + cos*(y-cy) + cy

	out := rgba{}

	// Sombra: desplazada hacia abajo, difusa y algo más ancha abajo.
	{
		sx, sy := ux-0.01, uy-0.035
		spread := 0.02 * math.Max(0, (sy-y0)/(y1-y0))
		dx := math.Max(math.Max(x0-spread-sx, sx-(x1+spread)), 0)
		dy := math.Max(math.Max(y0+0.03-sy, sy-y1), 0)
		a := 0.42 * (1 - smoothstep(0, 0.07, math.Hypot(dx, dy)))
		out = over(out, rgba{0, 0, 0, a})
	}

	inRect := ux >= x0 && ux <= x1 && uy >= y0 && uy <= y1
	diag := (x1 - ux) + (y1 - uy)
	if !inRect || diag < fold {
		return out // fuera del papel (o en la esquina recortada)
	}

	// Papel: degradado vertical + franja adhesiva arriba.
	t := (uy - y0) / (y1 - y0)
	r, g, b := mix([3]uint8{255, 238, 125}, [3]uint8{255, 222, 86}, t)
	if uy < y0+0.16 {
		k := 1 - (uy-y0)/0.16
		r, g, b = mix([3]uint8{r, g, b}, [3]uint8{240, 205, 70}, 0.45*k)
	}
	paper := solid(r, g, b, 1)

	// Sombra que la solapa proyecta sobre el papel (junto a sus bordes).
	tipX, tipY := x1-fold, y1-fold
	if ux > tipX-0.05 && uy > tipY-0.05 {
		d := math.Min(math.Abs(ux-tipX), math.Abs(uy-tipY))
		if ux < tipX || uy < tipY {
			paper = over(paper, rgba{0, 0, 0, 0.22 * (1 - smoothstep(0, 0.05, d))})
		}
	}

	// Líneas escritas a mano.
	ink := solid(38, 48, 92, 1)
	thick := math.Max(0.028, 0.9/float64(size))
	lines := [][4]float64{
		{0.25, 0.37, 0.73, 0.36},
		{0.25, 0.50, 0.70, 0.50},
		{0.25, 0.63, 0.53, 0.635},
	}
	if size <= 24 { // en tamaños mini, dos líneas se leen mejor que tres
		lines = lines[:2]
		thick = 1.1 / float64(size)
	}
	for _, l := range lines {
		d := segDist(ux, uy, l[0], l[1], l[2], l[3])
		aa := 1.0 / float64(size)
		a := 1 - smoothstep(thick-aa, thick+aa, d)
		if a > 0 {
			paper = over(paper, rgba{ink.r * a, ink.g * a, ink.b * a, a})
		}
	}
	out = over(out, paper)

	// Solapa doblada: de la punta (clara) al pliegue (más oscuro).
	if ux >= tipX && uy >= tipY && (ux-tipX)+(uy-tipY) <= fold {
		f := ((ux - tipX) + (uy - tipY)) / fold // 0 punta, 1 pliegue
		r, g, b := mix([3]uint8{255, 246, 175}, [3]uint8{232, 196, 64}, f)
		flap := solid(r, g, b, 1)
		if f > 0.93 {
			flap = over(flap, rgba{0, 0, 0, 0.25})
		}
		out = over(out, flap)
	}
	return out
}

// draw renderiza el ícono con supermuestreo 4×4 (bordes suaves).
func draw(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var acc rgba
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					c := sample(x, y, size)
					acc.r += c.r
					acc.g += c.g
					acc.b += c.b
					acc.a += c.a
				}
			}
			n := float64(ss * ss)
			a := acc.a / n
			if a <= 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(acc.r / n / a * 255)),
				G: uint8(math.Round(acc.g / n / a * 255)),
				B: uint8(math.Round(acc.b / n / a * 255)),
				A: uint8(math.Round(a * 255)),
			})
		}
	}
	return img
}

// ---------- formato .ico ----------

func dim(s int) uint8 {
	if s >= 256 {
		return 0 // 0 significa 256 en los formatos de ícono
	}
	return uint8(s)
}

func buildICO(images [][]byte) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&b, le, [3]uint16{0, 1, uint16(len(images))})
	offset := 6 + 16*len(images)
	for i, img := range images {
		b.Write([]byte{dim(sizes[i]), dim(sizes[i]), 0, 0})
		binary.Write(&b, le, [2]uint16{1, 32})
		binary.Write(&b, le, [2]uint32{uint32(len(img)), uint32(offset)})
		offset += len(img)
	}
	for _, img := range images {
		b.Write(img)
	}
	return b.Bytes()
}

// ---------- recurso COFF (.syso) ----------
//
// Un .syso es un objeto COFF que el enlazador de Go añade al ejecutable.
// Contiene una sección .rsrc con el árbol de recursos de Windows:
//   tipo (RT_ICON / RT_GROUP_ICON) → id → idioma → datos.

const (
	rtIcon      = 3
	rtGroupIcon = 14
	groupIconID = 1 // tray.go carga el ícono con este id
	langEnUS    = 0x0409
)

type resLeaf struct {
	typ, id uint32
	data    []byte
}

func buildSyso(images [][]byte) []byte {
	le := binary.LittleEndian

	// Hojas: cada imagen como RT_ICON 1..N, y el directorio del grupo.
	var icons []resLeaf
	var grp bytes.Buffer
	binary.Write(&grp, le, [3]uint16{0, 1, uint16(len(images))})
	for i, img := range images {
		id := uint32(i + 1)
		icons = append(icons, resLeaf{rtIcon, id, img})
		grp.Write([]byte{dim(sizes[i]), dim(sizes[i]), 0, 0})
		binary.Write(&grp, le, [2]uint16{1, 32})
		binary.Write(&grp, le, uint32(len(img)))
		binary.Write(&grp, le, uint16(id))
	}
	types := []struct {
		typ    uint32
		leaves []resLeaf
	}{
		{rtIcon, icons},
		{rtGroupIcon, []resLeaf{{rtGroupIcon, groupIconID, grp.Bytes()}}},
	}

	// Calculamos el tamaño de cada nivel para conocer los offsets.
	const dirSize, entrySize, dataEntrySize = 16, 8, 16
	nLeaves := 0
	for _, t := range types {
		nLeaves += len(t.leaves)
	}
	level1 := dirSize + entrySize*len(types)
	level2 := 0
	for _, t := range types {
		level2 += dirSize + entrySize*len(t.leaves)
	}
	level3 := (dirSize + entrySize) * nLeaves
	dataEntries := dataEntrySize * nLeaves
	dataStart := level1 + level2 + level3 + dataEntries

	sec := make([]byte, dataStart)
	var relocs []uint32 // offsets de campos que el enlazador debe convertir en RVA

	putDir := func(off, n int) int {
		le.PutUint16(sec[off+14:], uint16(n)) // NumberOfIdEntries
		return off + dirSize
	}
	putEntry := func(off int, id, target uint32, isDir bool) {
		if isDir {
			target |= 0x80000000
		}
		le.PutUint32(sec[off:], id)
		le.PutUint32(sec[off+4:], target)
	}

	l2 := level1
	l3 := level1 + level2
	de := level1 + level2 + level3
	data := []byte{}

	e1 := putDir(0, len(types))
	for i, t := range types {
		putEntry(e1+i*entrySize, t.typ, uint32(l2), true)
		e2 := putDir(l2, len(t.leaves))
		l2 = e2 + entrySize*len(t.leaves)
		for j, leaf := range t.leaves {
			putEntry(e2+j*entrySize, leaf.id, uint32(l3), true)
			e3 := putDir(l3, 1)
			putEntry(e3, langEnUS, uint32(de), false)
			l3 = e3 + entrySize

			for len(data)%8 != 0 {
				data = append(data, 0)
			}
			le.PutUint32(sec[de:], uint32(dataStart+len(data))) // reubicado → RVA
			le.PutUint32(sec[de+4:], uint32(len(leaf.data)))
			relocs = append(relocs, uint32(de))
			de += dataEntrySize
			data = append(data, leaf.data...)
		}
	}
	sec = append(sec, data...)

	// Objeto COFF: cabecera + 1 sección + reubicaciones + 1 símbolo + tabla de cadenas.
	const fileHdr, secHdr, relocSize, symSize = 20, 40, 10, 18
	rawPtr := fileHdr + secHdr
	relocPtr := rawPtr + len(sec)
	symPtr := relocPtr + relocSize*len(relocs)

	var out bytes.Buffer
	binary.Write(&out, le, struct {
		Machine              uint16
		NumberOfSections     uint16
		TimeDateStamp        uint32
		PointerToSymbolTable uint32
		NumberOfSymbols      uint32
		SizeOfOptionalHeader uint16
		Characteristics      uint16
	}{0x8664, 1, 0, uint32(symPtr), 1, 0, 0})
	binary.Write(&out, le, struct {
		Name                 [8]byte
		VirtualSize          uint32
		VirtualAddress       uint32
		SizeOfRawData        uint32
		PointerToRawData     uint32
		PointerToRelocations uint32
		PointerToLinenumbers uint32
		NumberOfRelocations  uint16
		NumberOfLinenumbers  uint16
		Characteristics      uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(len(sec)), uint32(rawPtr), uint32(relocPtr), 0,
		uint16(len(relocs)), 0, 0x40000040 /* INITIALIZED_DATA | MEM_READ */})
	out.Write(sec)
	for _, r := range relocs {
		binary.Write(&out, le, struct {
			VirtualAddress   uint32
			SymbolTableIndex uint32
			Type             uint16
		}{r, 0, 3 /* IMAGE_REL_AMD64_ADDR32NB */})
	}
	binary.Write(&out, le, struct {
		Name               [8]byte
		Value              uint32
		SectionNumber      int16
		Type               uint16
		StorageClass       uint8
		NumberOfAuxSymbols uint8
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 1, 0, 3 /* STATIC */, 0})
	binary.Write(&out, le, uint32(4)) // tabla de cadenas vacía
	return out.Bytes()
}
