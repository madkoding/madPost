//go:build windows

package main

import (
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const noteClass = "MadPostNote"

const (
	timerSave   = 1
	saveDelayMs = 800 // espera tras la última tecla antes de guardar

	cmdColorBase = 100
	cmdTopmost   = 200
	cmdNew       = 201
	cmdDelete    = 202
	cmdDuplicate = 203
	cmdTodo      = 204
)

// noteColor es un color de papel; los tonos derivados se calculan una vez.
type noteColor struct {
	name    string
	r, g, b byte

	body       uintptr // papel
	adhesive   uintptr // franja adhesiva (arriba)
	lifted     uintptr // borde inferior, que se despega un poco
	foldShadow uintptr // sombra que la solapa proyecta sobre el papel
	foldEdge   uintptr // solapa junto al pliegue
	foldTip    uintptr // punta de la solapa
	muted      uintptr // iconos discretos
	bodyBrush  uintptr
	creasePen  uintptr // línea del pliegue
}

// Colores inspirados en los blocs clásicos.
var palette = []noteColor{
	{name: "Amarillo", r: 255, g: 241, b: 133},
	{name: "Rosa", r: 255, g: 199, b: 217},
	{name: "Verde", r: 206, g: 240, b: 172},
	{name: "Azul", r: 184, g: 226, b: 250},
	{name: "Naranja", r: 255, g: 211, b: 156},
	{name: "Lila", r: 223, g: 207, b: 252},
}

var inkColor = rgb(38, 40, 62) // azul-negro, como un rotulador

type barButton int

const (
	btnNone barButton = iota
	btnNew
	btnMenu
	btnClose
)

// noteWin es una nota viva: sus datos + sus ventanas.
type noteWin struct {
	Note
	hwnd   uintptr // ventana de la nota
	edit   uintptr // control EDIT con el texto
	shadow uintptr // ventana translúcida con la sombra (dueña de hwnd)
	active bool    // tiene el foco: controles visibles y nota "levantada"

	shadowW, shadowH int32 // tamaño con el que se calculó la sombra
	shadowLifted     bool
}

var (
	notes    = map[uintptr]*noteWin{}
	creating *noteWin // la nota cuya ventana se está creando ahora mismo
	fontText uintptr
	fontBar  uintptr
)

// Medidas en píxeles reales (escaladas por DPI).
func barHeight() int32 { return px(28) }
func margin() int32    { return px(12) }
func gripSize() int32  { return px(16) }
func edgeSize() int32  { return px(6) }
func curlSize() int32  { return px(24) } // lado de la esquina doblada

func initNoteResources() {
	fontText = createFont(px(17), 400, "Segoe Print")
	fontBar = createFont(px(17), 400, "Segoe UI")
	for i := range palette {
		c := &palette[i]
		c.body = rgb(c.r, c.g, c.b)
		c.adhesive = shade(c.r, c.g, c.b, 0.94)
		c.lifted = shade(c.r, c.g, c.b, 0.95)
		c.foldShadow = shade(c.r, c.g, c.b, 0.74)
		c.foldEdge = shade(c.r, c.g, c.b, 0.86)
		c.foldTip = shade(c.r, c.g, c.b, 1.04)
		c.muted = shade(c.r, c.g, c.b, 0.50)
		c.bodyBrush = createBrush(c.body)
		c.creasePen, _, _ = procCreatePen.Call(0 /*PS_SOLID*/, 1, shade(c.r, c.g, c.b, 0.70))
	}
	registerClass(noteClass, syscall.NewCallback(noteProc), csHRedraw|csVRedraw)
	registerClass(shadowClass, syscall.NewCallback(shadowProc), 0)
	initTodoResources()
}

func (nw *noteWin) color() *noteColor {
	if nw.Color < 0 || nw.Color >= len(palette) {
		return &palette[0]
	}
	return &palette[nw.Color]
}

// createNoteWindow materializa una Note en pantalla.
func createNoteWindow(n Note) *noteWin {
	nw := &noteWin{Note: n}
	ex := uint32(wsExToolWindow) // fuera de la barra de tareas
	if n.Topmost {
		ex |= wsExTopmost
	}
	// La sombra es la "dueña" de la nota: Windows mantiene siempre a una
	// ventana por encima de su dueña, así la sombra queda justo debajo.
	nw.shadow = createWindow(ex|wsExLayered|wsExTransparent|wsExNoActivate, shadowClass, "", wsPopup, 0, 0, 0, 0, 0)

	creating = nw
	createWindow(ex|wsExLayered, noteClass, "madPost", wsPopup|wsClipChildren, n.X, n.Y, n.W, n.H, nw.shadow)
	creating = nil
	applyColorKey(nw.hwnd) // esquina doblada transparente

	nw.edit = createWindow(0, "EDIT", "",
		wsChild|wsVisible|esMultiline|esAutoVScroll|esWantReturn|esNoHideSel,
		0, 0, 0, 0, nw.hwnd)
	procSendMessageW.Call(nw.edit, wmSetFont, fontText, 0)
	subclassEdit(nw) // casillas de tareas clicables
	if n.Todo {
		nw.Text = normalizeTasks(n.Text) // ☑ antiguas → ☒
	}
	setWindowText(nw.edit, nw.Text)
	procKillTimer.Call(nw.hwnd, timerSave) // SetWindowText dispara EN_CHANGE; no es un cambio real
	nw.layout()
	nw.syncShadow()
	procShowWindow.Call(nw.shadow, swShowNoActivate)
	procShowWindow.Call(nw.hwnd, swShowNoActivate)
	return nw
}

func (nw *noteWin) layout() {
	if nw.edit == 0 {
		return
	}
	rc := getClientRect(nw.hwnd)
	m, top := margin(), barHeight()+px(2)
	moveWindow(nw.edit, m, top, rc.Right-2*m, rc.Bottom-top-curlSize()-px(2))
}

func (nw *noteWin) buttonRect(btn barButton, width int32) RECT {
	b := barHeight()
	switch btn {
	case btnNew:
		return RECT{0, 0, b, b}
	case btnMenu:
		return RECT{width - 2*b, 0, width - b, b}
	case btnClose:
		return RECT{width - b, 0, width, b}
	}
	return RECT{}
}

// pinRect es el cuadro del 📌: centrado en el espacio libre de la franja.
func pinRect(width int32) RECT {
	b := barHeight()
	cx := (b + width - 2*b) / 2
	return RECT{cx - b/2, 0, cx + b/2, b}
}

func (nw *noteWin) buttonAt(x, y int32) barButton {
	width := getClientRect(nw.hwnd).Right
	for _, b := range []barButton{btnNew, btnMenu, btnClose} {
		if r := nw.buttonRect(b, width); r.contains(x, y) {
			return b
		}
	}
	return btnNone
}

// hitTest le dice a Windows qué parte de la ventana está bajo el mouse:
// bordes = redimensionar, franja superior = arrastrar, botones/resto = cliente.
func (nw *noteWin) hitTest(lp uintptr) uintptr {
	sx, sy := lparamPoint(lp)
	wr := getWindowRect(nw.hwnd)
	x, y := sx-wr.Left, sy-wr.Top
	w, h := wr.Right-wr.Left, wr.Bottom-wr.Top
	e := edgeSize()

	left, right, top, bottom := x < e, x >= w-e, y < e, y >= h-e
	switch {
	case (w-x)+(h-y) <= curlSize()+px(10): // la solapa doblada es el agarre
		return htBottomRight
	case top && left:
		return htTopLeft
	case top && right:
		return htTopRight
	case bottom && left:
		return htBottomLeft
	case bottom && right:
		return htBottomRight
	case left:
		return htLeft
	case right:
		return htRight
	case top:
		return htTop
	case bottom:
		return htBottom
	}
	if y < barHeight() {
		if nw.active && nw.buttonAt(x, y) != btnNone {
			return htClient
		}
		return htCaption
	}
	return htClient
}

// paint dibuja primero en un bitmap en memoria y lo copia de una vez a la
// pantalla (doble búfer): así nunca se ve un fotograma a medio pintar.
func (nw *noteWin) paint() {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(nw.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(nw.hwnd, uintptr(unsafe.Pointer(&ps)))

	rc := getClientRect(nw.hwnd)
	w, h := rc.Right, rc.Bottom
	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	old, _, _ := procSelectObject.Call(mem, bmp)

	nw.draw(mem, w, h)
	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, srcCopy)

	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
}

func (nw *noteWin) draw(hdc uintptr, w, h int32) {
	c := nw.color()
	rc := RECT{0, 0, w, h}

	// Papel liso.
	fillRect(hdc, &rc, c.bodyBrush)
	// Franja adhesiva: algo más oscura arriba, se funde con el papel.
	gradientV(hdc, RECT{0, 0, w, barHeight()}, c.adhesive, c.body)
	// El borde inferior se despega ligeramente de la pared.
	gradientV(hdc, RECT{0, h - gripSize(), w, h}, c.body, c.lifted)
	// Esquina doblada: hueco transparente + solapa.
	paintCutCorner(hdc, w, h)
	paintFold(hdc, c, w, h)

	procSetBkMode.Call(hdc, bkTransparent)
	procSetTextColor.Call(hdc, c.muted)
	old, _, _ := procSelectObject.Call(hdc, fontBar)
	if nw.active { // los controles solo aparecen en la nota con foco
		labels := map[barButton]string{btnNew: "+", btnMenu: "⋯", btnClose: "✕"}
		for btn, label := range labels {
			r := nw.buttonRect(btn, w)
			drawText(hdc, label, &r, dtCenter|dtVCenter|dtSingleLine)
		}
	}
	if nw.Topmost {
		r := pinRect(w)
		drawText(hdc, "📌", &r, dtCenter|dtVCenter|dtSingleLine)
	}
	if nw.Todo { // progreso de la lista, p. ej. "2/5"
		if done, total := taskProgress(getWindowText(nw.edit)); total > 0 {
			r := RECT{barHeight(), 0, w - 2*barHeight(), barHeight()}
			drawText(hdc, strconv.Itoa(done)+"/"+strconv.Itoa(total), &r, dtVCenter|dtSingleLine)
		}
	}
	procSelectObject.Call(hdc, old)
}

func (nw *noteWin) applyTopmost() {
	after := hwndNoTopmost
	if nw.Topmost {
		after = hwndTopmost
	}
	for _, h := range []uintptr{nw.shadow, nw.hwnd} { // dueña primero, luego la nota
		procSetWindowPos.Call(h, after, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
	invalidate(nw.hwnd)
}

func (nw *noteWin) showMenu() {
	m, _, _ := procCreatePopupMenu.Call()
	for i, c := range palette {
		flags := uint32(mfString)
		if i == nw.Color {
			flags |= mfChecked
		}
		appendMenu(m, flags, cmdColorBase+i, c.name)
	}
	checked := func(on bool) uint32 {
		if on {
			return mfString | mfChecked
		}
		return mfString
	}
	appendMenu(m, mfSeparator, 0, "")
	appendMenu(m, checked(nw.Todo), cmdTodo, "Lista de tareas")
	appendMenu(m, checked(nw.Topmost), cmdTopmost, "Siempre visible")
	appendMenu(m, mfSeparator, 0, "")
	appendMenu(m, mfString, cmdNew, "Nueva nota")
	appendMenu(m, mfString, cmdDuplicate, "Duplicar nota")
	appendMenu(m, mfString, cmdDelete, "Eliminar nota")

	switch cmd := popupMenu(nw.hwnd, m); {
	case cmd >= cmdColorBase && cmd < cmdColorBase+len(palette):
		nw.Color = cmd - cmdColorBase
		invalidate(nw.hwnd)
		invalidate(nw.edit)
		saveAll()
	case cmd == cmdTopmost:
		nw.Topmost = !nw.Topmost
		nw.applyTopmost()
		saveAll()
	case cmd == cmdTodo:
		nw.setTodo(!nw.Todo)
		saveAll()
	case cmd == cmdNew:
		newNote()
	case cmd == cmdDuplicate:
		nw.duplicate()
	case cmd == cmdDelete:
		nw.remove()
	}
}

func (nw *noteWin) remove() {
	if strings.TrimSpace(getWindowText(nw.edit)) != "" {
		if messageBox(nw.hwnd, "¿Eliminar esta nota?", "madPost", mbYesNo|mbIconQuestion) != idYes {
			return
		}
	}
	// Destruir la dueña (sombra) destruye también la nota; WM_DESTROY la quita del mapa.
	procDestroyWindow.Call(nw.shadow)
	saveAll()
}

// sync copia el estado real de la ventana (posición, tamaño, texto) a Note.
func (nw *noteWin) sync() {
	r := getWindowRect(nw.hwnd)
	nw.X, nw.Y, nw.W, nw.H = r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top
	nw.Text = getWindowText(nw.edit)
}

func collectNotes() []Note {
	list := make([]Note, 0, len(notes))
	for _, nw := range notes {
		nw.sync()
		list = append(list, nw.Note)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

var saveErrShown bool

func saveAll() {
	if err := saveNotes(collectNotes()); err != nil && !saveErrShown {
		saveErrShown = true
		messageBox(0, "No se pudieron guardar las notas:\n"+err.Error(), "madPost", mbIconWarning)
	}
}

func newNoteID() string { return strconv.FormatInt(time.Now().UnixNano(), 36) }

func newNote() {
	w, h := px(230), px(230) // los post-its son cuadrados
	x, y := nextNotePosition(collectNotes(), w, h, workArea())
	nw := createNoteWindow(Note{ID: newNoteID(), X: x, Y: y, W: w, H: h})
	nw.focus()
	saveAll()
}

// duplicate crea una copia exacta (texto, color, tamaño, modo) un poco
// desplazada, para que se vea que hay dos.
func (nw *noteWin) duplicate() {
	nw.sync()
	n := nw.Note
	n.ID = newNoteID()
	off := px(28)
	n.X, n.Y = nw.X+off, nw.Y+off
	if !isVisibleOnScreen(n) { // sin espacio abajo a la derecha: hacia arriba a la izquierda
		n.X, n.Y = nw.X-off, nw.Y-off
	}
	createNoteWindow(n).focus()
	saveAll()
}

func (nw *noteWin) focus() {
	procSetForegroundWindow.Call(nw.hwnd)
	procSetFocus.Call(nw.edit)
}

func lookupNote(hwnd uintptr) *noteWin {
	if nw := notes[hwnd]; nw != nil {
		return nw
	}
	// Los primeros mensajes (WM_CREATE, WM_SIZE...) llegan dentro de
	// CreateWindowEx, antes de conocer el hwnd: los asociamos aquí.
	if creating != nil {
		creating.hwnd = hwnd
		notes[hwnd] = creating
		return creating
	}
	return nil
}

func noteProc(hwnd, msg, wp, lp uintptr) uintptr {
	nw := lookupNote(hwnd)
	if nw == nil {
		return defWindowProc(hwnd, msg, wp, lp)
	}
	switch msg {
	case wmActivate:
		nw.active = wp&0xFFFF != 0
		invalidate(hwnd)
		nw.syncShadow() // la nota activa se "levanta": sombra más amplia
	case wmWindowPosChanged:
		// Primero DefWindowProc (envía WM_SIZE → la nota se repinta), luego
		// la sombra, para que ambas lleguen al mismo fotograma de DWM.
		r := defWindowProc(hwnd, msg, wp, lp)
		nw.syncShadow()
		return r
	case wmEraseBkgnd:
		return 1 // todo se pinta en WM_PAINT (evita parpadeo)
	case wmPaint:
		nw.paint()
		return 0
	case wmSize:
		nw.layout()
		// Pintar YA, dentro del redimensionado: si esperamos al WM_PAINT de la
		// cola, Windows muestra un instante el borde nuevo sin pintar.
		procRedrawWindow.Call(hwnd, 0, 0, rdwInvalidate|rdwUpdateNow|rdwAllChildren)
		return 0
	case wmSetFocus:
		procSetFocus.Call(nw.edit)
		return 0
	case wmNcHitTest:
		return nw.hitTest(lp)
	case wmNcLButtonDblClk:
		// Doble clic en el 📌 = quitar "Siempre visible". Las coordenadas
		// llegan en pantalla; se pasan a la ventana.
		sx, sy := lparamPoint(lp)
		wr := getWindowRect(hwnd)
		if nw.Topmost && pinRect(wr.Right-wr.Left).contains(sx-wr.Left, sy-wr.Top) {
			nw.Topmost = false
			nw.applyTopmost()
			invalidate(hwnd)
			saveAll()
		}
		return 0 // y nunca "maximizar" al hacer doble clic en la franja
	case wmNcRButtonUp, wmRButtonUp:
		nw.showMenu()
		return 0
	case wmLButtonDown:
		procSetFocus.Call(nw.edit)
		return 0
	case wmLButtonUp:
		x, y := lparamPoint(lp)
		switch nw.buttonAt(x, y) {
		case btnNew:
			newNote()
		case btnMenu:
			nw.showMenu()
		case btnClose:
			nw.remove()
		}
		return 0
	case wmExitSizeMove:
		saveAll()
		return 0
	case wmGetMinMaxInfo:
		mmi := (*MINMAXINFO)(unsafe.Pointer(lp))
		mmi.PtMinTrackSize = POINT{px(150), px(150)}
		return 0
	case wmCtlColorEdit:
		c := nw.color()
		procSetBkColor.Call(wp, c.body)
		procSetTextColor.Call(wp, inkColor)
		return c.bodyBrush
	case wmCommand:
		if hiword(wp) == enChange {
			procSetTimer.Call(hwnd, timerSave, saveDelayMs, 0) // reinicia el temporizador
			if nw.Todo {
				invalidate(hwnd) // actualizar el contador "hechas/total"
			}
		}
		return 0
	case wmTimer:
		if wp == timerSave {
			procKillTimer.Call(hwnd, timerSave)
			saveAll()
		}
		return 0
	case wmDestroy:
		delete(notes, hwnd)
		return 0
	}
	return defWindowProc(hwnd, msg, wp, lp)
}
