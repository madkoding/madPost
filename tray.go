//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	trayClass = "MadPostTray"
	wmTray    = wmApp + 1 // eventos del ícono de bandeja
	wmNewNote = wmApp + 2 // otra instancia pide una nota nueva

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 1
	nifIcon    = 2
	nifTip     = 4

	trayCmdNew       = 1
	trayCmdShow      = 2
	trayCmdExit      = 3
	trayCmdAutostart = 4
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

var (
	trayHwnd          uintptr
	trayData          NOTIFYICONDATAW
	msgTaskbarCreated uintptr
)

func createTray() {
	registerClass(trayClass, syscall.NewCallback(trayProc), 0)
	trayHwnd = createWindow(0, trayClass, "madPost", wsPopup, 0, 0, 0, 0, 0) // nunca se muestra
	// Explorer emite este mensaje si se reinicia; hay que volver a añadir el ícono.
	msgTaskbarCreated, _, _ = procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(wstr("TaskbarCreated"))))

	icon := appIcon(systemMetric(smCXSmIcon), systemMetric(smCYSmIcon))
	trayData = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(trayData)),
		HWnd:             trayHwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            icon,
	}
	tip, _ := syscall.UTF16FromString("madPost — clic: mostrar notas · clic derecho: menú")
	copy(trayData.SzTip[:len(trayData.SzTip)-1], tip)
	addTrayIcon()
}

const (
	smCXSmIcon = 49
	smCYSmIcon = 50
	imageIcon  = 1
)

var procLoadImageW = user32.NewProc("LoadImageW")

// appIcon carga el ícono incrustado en el .exe (grupo 1, ver tools/mkicon)
// eligiendo la imagen más cercana al tamaño pedido. Si el .exe se compiló
// sin el .syso, usa el ícono genérico de Windows.
func appIcon(w, h int32) uintptr {
	if icon, _, _ := procLoadImageW.Call(hInstance, 1, imageIcon, uintptr(w), uintptr(h), 0); icon != 0 {
		return icon
	}
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	return icon
}

func addTrayIcon() {
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
}

func showAllNotes() {
	for _, nw := range notes {
		procSetWindowPos.Call(nw.hwnd, hwndTop, 0, 0, 0, 0, swpNoMove|swpNoSize)
		procSetForegroundWindow.Call(nw.hwnd)
	}
}

func quit() {
	saveAll()
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayData)))
	procPostQuitMessage.Call(0)
}

func trayMenu() {
	m, _, _ := procCreatePopupMenu.Call()
	appendMenu(m, mfString, trayCmdNew, "Nueva nota")
	appendMenu(m, mfString, trayCmdShow, "Mostrar notas")
	appendMenu(m, mfSeparator, 0, "")
	autostart := autostartEnabled() // se lee del registro cada vez: refleja cambios hechos fuera
	autoFlags := uint32(mfString)
	if autostart {
		autoFlags |= mfChecked
	}
	appendMenu(m, autoFlags, trayCmdAutostart, "Iniciar con Windows")
	appendMenu(m, mfSeparator, 0, "")
	appendMenu(m, mfString, trayCmdExit, "Salir")
	switch popupMenu(trayHwnd, m) {
	case trayCmdNew:
		newNote()
	case trayCmdShow:
		showAllNotes()
	case trayCmdAutostart:
		if err := setAutostart(!autostart); err != nil {
			messageBox(0, "No se pudo cambiar el inicio con Windows:\n"+err.Error(), "madPost", mbIconWarning)
		}
	case trayCmdExit:
		quit()
	}
}

func trayProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch {
	case msg == wmTray:
		switch lp & 0xFFFF {
		case wmLButtonUp:
			showAllNotes()
		case wmLButtonDblClk:
			newNote()
		case wmRButtonUp:
			trayMenu()
		}
		return 0
	case msg == wmNewNote:
		newNote()
		return 0
	case msg == wmQueryEndSession:
		saveAll() // Windows se está apagando
		return 1
	case msg == wmEndSession:
		return 0
	case msgTaskbarCreated != 0 && msg == msgTaskbarCreated:
		addTrayIcon()
		return 0
	}
	return defWindowProc(hwnd, msg, wp, lp)
}
