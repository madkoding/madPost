//go:build windows

// Efectos visuales: sombra realista (ventana translúcida por píxel),
// esquina doblada y degradados GDI.
package main

import (
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	msimg32 = syscall.NewLazyDLL("msimg32.dll")

	procGradientFill        = msimg32.NewProc("GradientFill")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procCreatePen           = gdi32.NewProc("CreatePen")
	procPolygon             = gdi32.NewProc("Polygon")
	procGetStockObject      = gdi32.NewProc("GetStockObject")

	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procMoveToEx                   = gdi32.NewProc("MoveToEx")
	procLineTo                     = gdi32.NewProc("LineTo")

	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procRedrawWindow           = user32.NewProc("RedrawWindow")
)

const (
	srcCopy = 0x00CC0020

	rdwInvalidate  = 0x0001
	rdwAllChildren = 0x0080
	rdwUpdateNow   = 0x0100
)

const (
	shadowClass = "MadPostShadow"

	wmActivate         = 0x0006
	wmMouseActivate    = 0x0021
	wmWindowPosChanged = 0x0047
	maNoActivate       = 3
	htTransparent      = ^uintptr(0) // -1

	wsExTransparent = 0x00000020
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000

	swpNoZOrder  = 0x0004
	ulwAlpha     = 0x02
	lwaColorKey  = 0x01
	nullPenStock = 8 // NULL_PEN

	gradientFillRectH    = 0
	gradientFillRectV    = 1
	gradientFillTriangle = 2
)

type SIZE struct{ CX, CY int32 }

type bitmapInfo struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	Colors        [1]uint32
}

// --- degradados ---

type triVertex struct {
	X, Y       int32
	R, G, B, A uint16
}

func vertex(x, y int32, c uintptr) triVertex {
	return triVertex{X: x, Y: y,
		R: uint16(c&0xFF) << 8,
		G: uint16(c>>8&0xFF) << 8,
		B: uint16(c>>16&0xFF) << 8}
}

func gradientRect(hdc uintptr, r RECT, from, to uintptr, mode uintptr) {
	v := [2]triVertex{vertex(r.Left, r.Top, from), vertex(r.Right, r.Bottom, to)}
	mesh := [2]uint32{0, 1}
	procGradientFill.Call(hdc, uintptr(unsafe.Pointer(&v[0])), 2, uintptr(unsafe.Pointer(&mesh[0])), 1, mode)
}

// gradientV: de arriba (top) a abajo (bottom).
func gradientV(hdc uintptr, r RECT, top, bottom uintptr) {
	gradientRect(hdc, r, top, bottom, gradientFillRectV)
}

// gradientTriangle interpola el color entre los tres vértices.
func gradientTriangle(hdc uintptr, a, b, c triVertex) {
	v := [3]triVertex{a, b, c}
	mesh := [3]uint32{0, 1, 2}
	procGradientFill.Call(hdc, uintptr(unsafe.Pointer(&v[0])), 3, uintptr(unsafe.Pointer(&mesh[0])), 1, gradientFillTriangle)
}

// shade oscurece (f<1) o aclara (f>1) un color RGB.
func shade(r, g, b byte, f float64) uintptr {
	scale := func(v byte) byte { return byte(math.Min(float64(v)*f, 255)) }
	return rgb(scale(r), scale(g), scale(b))
}

func drawLine(hdc uintptr, pen uintptr, x1, y1, x2, y2 int32) {
	old, _, _ := procSelectObject.Call(hdc, pen)
	procMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	procLineTo.Call(hdc, uintptr(x2), uintptr(y2))
	procSelectObject.Call(hdc, old)
}

// --- esquina doblada ---

// La esquina recortada se hace con un "color clave": Windows vuelve
// transparentes (y atravesables por el mouse) los píxeles de ese color exacto.
// Antes usábamos SetWindowRgn, pero cambiar la forma de la ventana en cada
// paso del redimensionado obliga a DWM a recrear su superficie, y la nota
// se veía vacía durante un instante (el parpadeo del borde).
var (
	colorKey      = rgb(255, 0, 255)
	colorKeyBrush uintptr
)

func applyColorKey(hwnd uintptr) {
	if colorKeyBrush == 0 {
		colorKeyBrush = createBrush(colorKey)
	}
	procSetLayeredWindowAttributes.Call(hwnd, colorKey, 0, lwaColorKey)
}

// paintCutCorner pinta con el color clave el triángulo que queda más allá
// del pliegue: por ahí se ve el escritorio (y la sombra).
func paintCutCorner(hdc uintptr, w, h int32) {
	k := curlSize()
	pts := [3]POINT{{w + 1, h - k}, {w + 1, h + 1}, {w - k, h + 1}}
	oldBrush, _, _ := procSelectObject.Call(hdc, colorKeyBrush)
	nullPen, _, _ := procGetStockObject.Call(nullPenStock)
	oldPen, _, _ := procSelectObject.Call(hdc, nullPen)
	procPolygon.Call(hdc, uintptr(unsafe.Pointer(&pts[0])), 3)
	procSelectObject.Call(hdc, oldPen)
	procSelectObject.Call(hdc, oldBrush)
}

// paintFold dibuja la solapa doblada y la sombra que proyecta sobre el papel.
func paintFold(hdc uintptr, c *noteColor, w, h int32) {
	k, sh := curlSize(), px(6)
	tx, ty := w-k, h-k // punta de la solapa

	// Sombra de la solapa: más oscura junto a la punta, se desvanece hacia el pliegue.
	gradientTriangle(hdc, vertex(tx, ty-sh, c.body), vertex(tx, ty, c.foldShadow), vertex(w, ty, c.body))
	gradientTriangle(hdc, vertex(tx-sh, ty, c.body), vertex(tx, ty, c.foldShadow), vertex(tx, h, c.lifted))
	gradientTriangle(hdc, vertex(tx-sh, ty-sh, c.body), vertex(tx, ty-sh, c.body), vertex(tx, ty, c.foldShadow))
	gradientTriangle(hdc, vertex(tx-sh, ty-sh, c.body), vertex(tx-sh, ty, c.body), vertex(tx, ty, c.foldShadow))

	// La solapa (reverso del papel): más clara en la punta, que recibe más luz.
	gradientTriangle(hdc, vertex(w, ty, c.foldEdge), vertex(tx, h, c.foldEdge), vertex(tx, ty, c.foldTip))

	// Línea del pliegue.
	drawLine(hdc, c.creasePen, w, ty, tx, h)
}

// --- sombra ---

// Forma de la sombra (en píxeles lógicos). shadowPad se deriva de estos
// mismos valores: si el bitmap es más chico que lo que la sombra se
// difumina, se corta en seco y aparece una "caja" que parpadea al redimensionar.
const (
	shadowLiftActive  = 1.4 // la nota con foco se "levanta"
	shadowSpreadMin   = -2.0
	shadowSpreadMax   = 3.0 // ensanche lateral abajo
	shadowBlurTop     = 5.0
	shadowBlurGrow    = 14.0 // difuminado extra hacia abajo
	shadowDropCenter  = 3.0  // desplazamiento bajo el centro del borde inferior
	shadowDropCorners = 8.0  // extra en las esquinas (se curvan más)
)

func shadowPad() (side, top, bottom int32) {
	l := shadowLiftActive
	maxBlur := (shadowBlurTop + shadowBlurGrow) * l
	side = px(int(math.Ceil(shadowSpreadMax*l+maxBlur)) + 2)
	bottom = px(int(math.Ceil((shadowDropCenter+shadowDropCorners)*l+maxBlur)) + 2)
	return side, px(10), bottom
}

func shadowProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmMouseActivate:
		return maNoActivate
	case wmNcHitTest:
		return htTransparent // los clics atraviesan la sombra
	}
	return defWindowProc(hwnd, msg, wp, lp)
}

// syncShadow coloca la sombra bajo la nota; solo la recalcula si cambió el
// tamaño o el estado "levantada" (moverla es barato, recalcularla no tanto).
func (nw *noteWin) syncShadow() {
	if nw.shadow == 0 || nw.hwnd == 0 {
		return
	}
	r := getWindowRect(nw.hwnd)
	w, h := r.Right-r.Left, r.Bottom-r.Top
	side, top, _ := shadowPad()
	x, y := r.Left-side, r.Top-top
	if w != nw.shadowW || h != nw.shadowH || nw.active != nw.shadowLifted {
		renderShadow(nw.shadow, x, y, w, h, nw.active)
		nw.shadowW, nw.shadowH, nw.shadowLifted = w, h, nw.active
		return
	}
	procSetWindowPos.Call(nw.shadow, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}

func renderShadow(hwnd uintptr, x, y, w, h int32, lifted bool) {
	side, top, bottom := shadowPad()
	bw, bh := w+2*side, h+top+bottom
	if bw <= 0 || bh <= 0 {
		return
	}
	bi := bitmapInfo{Size: 40, Width: bw, Height: -bh /* de arriba a abajo */, Planes: 1, BitCount: 32}
	mem, _, _ := procCreateCompatibleDC.Call(0)
	defer procDeleteDC.Call(mem)
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 {
		return
	}
	defer procDeleteObject.Call(bmp)

	fillShadow(unsafe.Slice((*uint32)(bits), int(bw*bh)), bw, bh, side, top, w, h, lifted)

	old, _, _ := procSelectObject.Call(mem, bmp)
	defer procSelectObject.Call(mem, old)
	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)

	dst, src, size := POINT{x, y}, POINT{}, SIZE{bw, bh}
	blend := uint32(255<<16 | 1<<24) // AC_SRC_OVER, alpha global 255, AC_SRC_ALPHA
	procUpdateLayeredWindow.Call(hwnd, screen,
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		mem, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
}

func clamp01(v float64) float64 { return max(0, min(1, v)) }

// dist es la longitud de (dx, dy); evita math.Hypot, que es más lento
// porque protege contra desbordes que aquí no pueden ocurrir.
func dist(dx, dy float64) float64 {
	if dx == 0 {
		return dy
	}
	if dy == 0 {
		return dx
	}
	return math.Sqrt(dx*dx + dy*dy)
}

func smoothstep(e0, e1, x float64) float64 {
	t := clamp01((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

// fillShadow calcula la sombra píxel a píxel (alfa premultiplicado, negro).
// Combina dos capas:
//   - ambiente: amplia y difusa; casi nula arriba (el adhesivo pega el papel
//     a la pared) y cada vez más ancha y suave hacia abajo, sobre todo en las
//     esquinas, que es donde el papel se curva y se despega.
//   - contacto: fina y nítida pegada al borde.
func fillShadow(pix []uint32, bw, bh, side, top, w, h int32, lifted bool) {
	s := dpiScale
	lift := 1.0
	if lifted {
		lift = shadowLiftActive
	}
	L, T := float64(side), float64(top)
	W, H := float64(w), float64(h)
	R, B := L+W, T+H
	cx := (L + R) / 2
	k := float64(curlSize())

	// Cada fila es independiente: las repartimos entre los núcleos.
	workers := int32(runtime.NumCPU())
	chunk := (bh + workers - 1) / workers
	var wg sync.WaitGroup
	for start := int32(0); start < bh; start += chunk {
		end := min(start+chunk, bh)
		wg.Add(1)
		go func(start, end int32) {
			defer wg.Done()
			shadowRows(pix, bw, start, end, s, lift, L, T, W, H, R, B, cx, k)
		}(start, end)
	}
	wg.Wait()
}

func shadowRows(pix []uint32, bw, start, end int32, s, lift, L, T, W, H, R, B, cx, k float64) {
	for py := start; py < end; py++ {
		y := float64(py) + 0.5
		t := clamp01((y - T) / H) // 0 arriba, 1 abajo
		spread := (shadowSpreadMin + (shadowSpreadMax-shadowSpreadMin)*t*t) * s * lift
		blur := (shadowBlurTop + shadowBlurGrow*t) * s * lift
		st := T + 6*s
		row := pix[py*bw : (py+1)*bw]
		insideRow := y > T+1 && y < B-1
		for px := int32(0); px < bw; px++ {
			x := float64(px) + 0.5
			// Lo que queda tapado por la nota no se ve: no lo calculamos
			// (es ~2/3 del bitmap y abarata mucho cada paso del redimensionado).
			if insideRow && x > L+1 && x < R-1 && (R-x)+(B-y) > k+1 {
				row[px] = 0
				continue
			}
			u := clamp01(math.Abs(x-cx) / (W / 2)) // 0 centro, 1 bordes
			sb := B + (shadowDropCenter+shadowDropCorners*u*u)*s*lift

			dx := max(L-spread-x, x-(R+spread), 0)
			dy := max(st-y, y-sb, 0)
			ambient := 0.26 * (1 - smoothstep(0, blur, dist(dx, dy)))

			dx2 := max(L-x, x-R, 0)
			dy2 := max(T+s-y, y-(B+1.5*s), 0)
			contact := 0.22 * (1 - smoothstep(0, 3*s, dist(dx2, dy2)))

			// Bajo la esquina doblada no hay papel: la sombra se aclara.
			if diag := (R - x) + (B - y); diag < k {
				dd := (k - diag) / math.Sqrt2
				ambient *= 1 - 0.55*smoothstep(0, 10*s, dd)
				contact *= 1 - smoothstep(0, 2*s, dd)
			}

			a := 1 - (1-ambient)*(1-contact)
			row[px] = uint32(a*255+0.5) << 24
		}
	}
}
