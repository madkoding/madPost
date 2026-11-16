//go:build windows

// Envoltorios mínimos sobre la API Win32 usando solo la librería estándar.
package main

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procDrawTextW              = user32.NewProc("DrawTextW")
	procEndPaint               = user32.NewProc("EndPaint")
	procFillRect               = user32.NewProc("FillRect")
	procFindWindowW            = user32.NewProc("FindWindowW")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procGetDpiForSystem        = user32.NewProc("GetDpiForSystem")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procGetWindowRect          = user32.NewProc("GetWindowRect")
	procGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procKillTimer              = user32.NewProc("KillTimer")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procLoadIconW              = user32.NewProc("LoadIconW")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procMoveWindow             = user32.NewProc("MoveWindow")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procSendMessageW           = user32.NewProc("SendMessageW")
	procSetFocus               = user32.NewProc("SetFocus")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procSetProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	procSetTimer               = user32.NewProc("SetTimer")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procSetWindowTextW         = user32.NewProc("SetWindowTextW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procSystemParametersInfoW  = user32.NewProc("SystemParametersInfoW")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procTranslateMessage       = user32.NewProc("TranslateMessage")

	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procSetBkColor       = gdi32.NewProc("SetBkColor")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")

	procCreateMutexW     = kernel32.NewProc("CreateMutexW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wsPopup        = 0x80000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsClipChildren = 0x02000000

	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080

	esMultiline   = 0x0004
	esAutoVScroll = 0x0040
	esNoHideSel   = 0x0100
	esWantReturn  = 0x1000
	enChange      = 0x0300

	csVRedraw    = 0x0001
	csHRedraw    = 0x0002
	csDropShadow = 0x00020000

	wmNull            = 0x0000
	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmSetFocus        = 0x0007
	wmPaint           = 0x000F
	wmQueryEndSession = 0x0011
	wmEraseBkgnd      = 0x0014
	wmEndSession      = 0x0016
	wmGetMinMaxInfo   = 0x0024
	wmSetFont         = 0x0030
	wmNcHitTest       = 0x0084
	wmNcLButtonDblClk = 0x00A3
	wmNcRButtonUp     = 0x00A5
	wmCommand         = 0x0111
	wmTimer           = 0x0113
	wmCtlColorEdit    = 0x0133
	wmLButtonDown     = 0x0201
	wmLButtonUp       = 0x0202
	wmLButtonDblClk   = 0x0203
	wmRButtonUp       = 0x0205
	wmExitSizeMove    = 0x0232
	wmApp             = 0x8000

	htClient      = 1
	htCaption     = 2
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	swShowNoActivate = 4

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010

	hwndTop       = uintptr(0)
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	hwndNoTopmost = ^uintptr(1) // (HWND)-2

	mfString    = 0x0000
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	mbYesNo        = 0x0004
	mbIconWarning  = 0x0030
	mbIconQuestion = 0x0020
	idYes          = 6

	dtCenter     = 0x0001
	dtVCenter    = 0x0004
	dtSingleLine = 0x0020

	bkTransparent = 1

	idcArrow       = 32512
	idiApplication = 32512

	spiGetWorkArea = 0x0030

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

type POINT struct{ X, Y int32 }

type RECT struct{ Left, Top, Right, Bottom int32 }

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

var hInstance uintptr

// --- utilidades ---

func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", ""))
	return p
}

func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

func hiword(v uintptr) uint16 { return uint16(v >> 16) }

// lparamPoint extrae coordenadas con signo (pueden ser negativas en multi-monitor).
func lparamPoint(lp uintptr) (int32, int32) { return int32(int16(lp)), int32(int16(lp >> 16)) }

func (r RECT) contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// --- envoltorios ---

func registerClass(name string, proc uintptr, style uint32) {
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         style,
		LpfnWndProc:   proc,
		HInstance:     hInstance,
		HCursor:       cursor,
		LpszClassName: wstr(name),
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

func createWindow(exStyle uint32, class, title string, style uint32, x, y, w, h int32, parent uintptr) uintptr {
	r, _, _ := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(wstr(class))),
		uintptr(unsafe.Pointer(wstr(title))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, 0, hInstance, 0)
	return r
}

func defWindowProc(h, msg, wp, lp uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(h, msg, wp, lp)
	return r
}

func getClientRect(h uintptr) RECT {
	var r RECT
	procGetClientRect.Call(h, uintptr(unsafe.Pointer(&r)))
	return r
}

func getWindowRect(h uintptr) RECT {
	var r RECT
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r)))
	return r
}

func getCursorPos() POINT {
	var p POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func workArea() RECT {
	var r RECT
	procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	return r
}

func systemMetric(i int) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int32(r)
}

func invalidate(h uintptr) { procInvalidateRect.Call(h, 0, 1) }

func moveWindow(h uintptr, x, y, w, hgt int32) {
	procMoveWindow.Call(h, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), 1)
}

func getWindowText(h uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func setWindowText(h uintptr, s string) {
	procSetWindowTextW.Call(h, uintptr(unsafe.Pointer(wstr(s))))
}

func messageBox(owner uintptr, text, caption string, flags uint32) int {
	r, _, _ := procMessageBoxW.Call(owner,
		uintptr(unsafe.Pointer(wstr(text))),
		uintptr(unsafe.Pointer(wstr(caption))),
		uintptr(flags))
	return int(r)
}

func fillRect(hdc uintptr, r *RECT, brush uintptr) {
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(r)), brush)
}

func drawText(hdc uintptr, s string, r *RECT, flags uint32) {
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(wstr(s))), ^uintptr(0), uintptr(unsafe.Pointer(r)), uintptr(flags))
}

func createFont(height int32, weight int, face string) uintptr {
	h := -height // altura negativa = tamaño de carácter, no de celda
	r, _, _ := procCreateFontW.Call(uintptr(h), 0, 0, 0, uintptr(weight),
		0, 0, 0, 1 /*DEFAULT_CHARSET*/, 0, 0, 5 /*CLEARTYPE_QUALITY*/, 0,
		uintptr(unsafe.Pointer(wstr(face))))
	return r
}

func createBrush(color uintptr) uintptr {
	r, _, _ := procCreateSolidBrush.Call(color)
	return r
}

func appendMenu(menu uintptr, flags uint32, id int, label string) {
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(wstr(label))))
}

// popupMenu muestra un menú contextual en el cursor y devuelve el id elegido (0 = nada).
func popupMenu(owner uintptr, menu uintptr) int {
	p := getCursorPos()
	procSetForegroundWindow.Call(owner)
	r, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton, uintptr(p.X), uintptr(p.Y), 0, owner, 0)
	procPostMessageW.Call(owner, wmNull, 0, 0)
	procDestroyMenu.Call(menu)
	return int(r)
}

// --- DPI ---

var dpiScale = 1.0

func initDPI() {
	procSetProcessDPIAware.Call()
	if procGetDpiForSystem.Find() == nil {
		if dpi, _, _ := procGetDpiForSystem.Call(); dpi > 0 {
			dpiScale = float64(dpi) / 96
		}
	}
}

// px convierte píxeles "lógicos" (a 96 DPI) a píxeles reales.
func px(v int) int32 { return int32(float64(v)*dpiScale + 0.5) }
