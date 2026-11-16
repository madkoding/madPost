//go:build windows

// Lista de tareas dentro de una nota: casillas clicables y Enter que crea
// una tarea nueva. Se logra "subclasificando" el control EDIT: sustituimos
// su procedimiento de ventana por editProc, que intercepta clics y teclas y
// delega todo lo demás al procedimiento original.
//
// Las casillas que se ven las dibujamos nosotros encima de los caracteres
// ☐/☒ del texto: así todas miden lo mismo y quedan centradas en su línea,
// sin depender de qué fuente elija Windows para esos símbolos.
package main

import (
	"syscall"
	"unsafe"
)

const (
	wmChar       = 0x0102
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmMouseMove  = 0x0200
	wmVScroll    = 0x0115
	wmMouseWheel = 0x020A
	wmSetText    = 0x000C

	emGetSel              = 0x00B0
	emSetSel              = 0x00B1
	emGetLineCount        = 0x00BA
	emLineIndex           = 0x00BB
	emReplaceSel          = 0x00C2
	emGetFirstVisibleLine = 0x00CE
	emPosFromChar         = 0x00D6
	emCharFromPos         = 0x00D7
	gwlpWndProc           = ^uintptr(3) // -4
)

var (
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procGetTextMetricsW   = gdi32.NewProc("GetTextMetricsW")
	procRoundRect         = gdi32.NewProc("RoundRect")
	procPolyline          = gdi32.NewProc("Polyline")

	editProcCallback = syscall.NewCallback(editProc)
	origEditProc     uintptr                  // el de la clase EDIT (es el mismo para todos)
	editNotes        = map[uintptr]*noteWin{} // EDIT → su nota

	textLineHeight int32 // alto de línea de fontText
	inkBrush       uintptr
)

func sendMsg(h, msg, wp, lp uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(h, msg, wp, lp)
	return r
}

func initTodoResources() {
	dc, _, _ := procGetDC.Call(0)
	old, _, _ := procSelectObject.Call(dc, fontText)
	var tm [64]byte // TEXTMETRICW; solo usamos tmHeight (primer campo)
	procGetTextMetricsW.Call(dc, uintptr(unsafe.Pointer(&tm[0])))
	textLineHeight = *(*int32)(unsafe.Pointer(&tm[0]))
	procSelectObject.Call(dc, old)
	procReleaseDC.Call(0, dc)
	inkBrush = createBrush(inkColor)
}

func subclassEdit(nw *noteWin) {
	prev, _, _ := procSetWindowLongPtrW.Call(nw.edit, gwlpWndProc, editProcCallback)
	if origEditProc == 0 {
		origEditProc = prev
	}
	editNotes[nw.edit] = nw
}

func callOrigEdit(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procCallWindowProcW.Call(origEditProc, hwnd, msg, wp, lp)
	return r
}

func editProc(hwnd, msg, wp, lp uintptr) uintptr {
	nw := editNotes[hwnd]
	if msg == wmDestroy {
		delete(editNotes, hwnd)
	}
	if nw == nil || !nw.Todo {
		return callOrigEdit(hwnd, msg, wp, lp)
	}
	switch msg {
	case wmLButtonDown:
		if nw.toggleTaskAt(lp) {
			nw.drawTaskBoxes()
			return 0 // el clic fue en una casilla: no mover el cursor
		}
	case wmChar:
		if wp == '\r' {
			nw.replaceSel(lineSep + taskPrefixOpen) // Enter = nueva tarea
			nw.drawTaskBoxes()
			return 0
		}
	}
	r := callOrigEdit(hwnd, msg, wp, lp)
	switch msg {
	// Tras pintar, o tras cualquier cosa con la que el EDIT redibuja líneas
	// por su cuenta (sin WM_PAINT), volvemos a poner nuestras casillas encima.
	case wmPaint, wmChar, wmKeyDown, wmKeyUp, wmLButtonDown, wmLButtonUp,
		wmLButtonDblClk, wmMouseMove, wmVScroll, wmMouseWheel, wmSetText, emReplaceSel:
		nw.drawTaskBoxes()
	}
	return r
}

func (nw *noteWin) replaceSel(s string) {
	sendMsg(nw.edit, emReplaceSel, 1 /* se puede deshacer */, uintptr(unsafe.Pointer(wstr(s))))
}

// editText devuelve el texto en UTF-16, que es como el EDIT cuenta posiciones.
func (nw *noteWin) editText() []uint16 {
	return syscall.StringToUTF16(getWindowText(nw.edit)) // termina en 0
}

// boxCell devuelve la celda (x, y, ancho) del carácter casilla en start.
func (nw *noteWin) boxCell(start uintptr) (x, y, adv int32) {
	x, y = lparamPoint(sendMsg(nw.edit, emPosFromChar, start, 0))
	adv = px(16)
	if next := sendMsg(nw.edit, emPosFromChar, start+1, 0); int32(next) != -1 {
		if nx, ny := lparamPoint(next); ny == y && nx > x {
			adv = nx - x
		}
	}
	return x, y, adv
}

// drawTaskBoxes tapa cada ☐/☒ visible y dibuja encima una casilla uniforme.
func (nw *noteWin) drawTaskBoxes() {
	if !nw.Todo || nw.edit == 0 {
		return
	}
	text := nw.editText()
	rc := getClientRect(nw.edit)
	hdc, _, _ := procGetDC.Call(nw.edit)
	defer procReleaseDC.Call(nw.edit, hdc)

	c := nw.color()
	pen, _, _ := procCreatePen.Call(0, uintptr(max(px(1), 1)), inkColor)
	defer procDeleteObject.Call(pen)
	checkPen, _, _ := procCreatePen.Call(0, uintptr(max(px(2), 2)), c.body)
	defer procDeleteObject.Call(checkPen)

	lines := sendMsg(nw.edit, emGetLineCount, 0, 0)
	for i := sendMsg(nw.edit, emGetFirstVisibleLine, 0, 0); i < lines; i++ {
		start := sendMsg(nw.edit, emLineIndex, i, 0)
		if int(start) >= len(text)-1 {
			break
		}
		ch := rune(text[start])
		if !isBox(ch) {
			continue
		}
		x, y, adv := nw.boxCell(start)
		if y >= rc.Bottom {
			break
		}
		drawCheckbox(hdc, c, pen, checkPen, x, y, adv, isDone(ch))
	}
}

func drawCheckbox(hdc uintptr, c *noteColor, pen, checkPen uintptr, x, y, adv int32, done bool) {
	lh := textLineHeight
	// Tapamos el glifo original con el color del papel.
	cover := RECT{x, y, x + adv, y + lh}
	fillRect(hdc, &cover, c.bodyBrush)

	size := min(adv-px(2), lh*48/100)
	bx := x + (adv-size)/2
	by := y + (lh-size)/2 + px(1) // Segoe Print tiene mucho espacio arriba
	radius := px(4)

	fill := c.bodyBrush
	if done {
		fill = inkBrush
	}
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	oldBrush, _, _ := procSelectObject.Call(hdc, fill)
	procRoundRect.Call(hdc, uintptr(bx), uintptr(by), uintptr(bx+size), uintptr(by+size), uintptr(radius), uintptr(radius))
	procSelectObject.Call(hdc, oldBrush)

	if done { // marca ✓ en el color del papel
		s := float64(size)
		pts := [3]POINT{
			{bx + int32(s*0.24), by + int32(s*0.52)},
			{bx + int32(s*0.43), by + int32(s*0.72)},
			{bx + int32(s*0.78), by + int32(s*0.30)},
		}
		procSelectObject.Call(hdc, checkPen)
		procPolyline.Call(hdc, uintptr(unsafe.Pointer(&pts[0])), 3)
	}
	procSelectObject.Call(hdc, oldPen)
}

// toggleTaskAt marca o desmarca la casilla bajo el punto (coordenadas del
// EDIT). Devuelve false si el clic no cayó sobre una casilla.
func (nw *noteWin) toggleTaskAt(lp uintptr) bool {
	r := sendMsg(nw.edit, emCharFromPos, 0, lp)
	line := r >> 16 & 0xFFFF
	start := sendMsg(nw.edit, emLineIndex, line, 0)

	text := nw.editText()
	if int(start) >= len(text)-1 || !isBox(rune(text[start])) {
		return false
	}
	// ¿El clic fue sobre la casilla y no sobre el texto de la tarea?
	bx, _, adv := nw.boxCell(start)
	if x, _ := lparamPoint(lp); x < bx || x > bx+adv {
		return false
	}

	var selStart, selEnd uint32
	sendMsg(nw.edit, emGetSel, uintptr(unsafe.Pointer(&selStart)), uintptr(unsafe.Pointer(&selEnd)))
	sendMsg(nw.edit, emSetSel, start, start+1)
	if isDone(rune(text[start])) {
		nw.replaceSel(string(boxOpen))
	} else {
		nw.replaceSel(string(boxDone))
	}
	sendMsg(nw.edit, emSetSel, uintptr(selStart), uintptr(selEnd))
	return true
}

// setTodo activa o desactiva el modo lista convirtiendo el texto.
func (nw *noteWin) setTodo(on bool) {
	nw.Todo = on
	text := getWindowText(nw.edit)
	if on {
		text = toTasks(text)
	} else {
		text = fromTasks(text)
	}
	setWindowText(nw.edit, text)
	n := uintptr(len(syscall.StringToUTF16(text)) - 1)
	sendMsg(nw.edit, emSetSel, n, n) // cursor al final
	procSetFocus.Call(nw.edit)
	invalidate(nw.hwnd)
	invalidate(nw.edit)
}
