//go:build windows

// madPost: post-its para el escritorio de Windows.
//
// Compilar:  go build -ldflags "-H windowsgui -s -w" -o madPost.exe .
//
// El ícono vive en rsrc_windows_amd64.syso; para regenerarlo: go generate
//
//go:generate go run ./tools/mkicon
package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var instanceMutex uintptr // se mantiene abierto mientras viva el proceso

func main() {
	// Win32 exige que los mensajes de una ventana se procesen en el hilo que la creó.
	runtime.LockOSThread()

	if !acquireSingleInstance() {
		return
	}
	initDPI()
	hInstance, _, _ = procGetModuleHandleW.Call(0)
	initNoteResources()
	createTray()
	refreshAutostart() // si el .exe se movió, actualiza la ruta del autoinicio

	loaded, err := loadNotes()
	if err != nil {
		messageBox(0, "No se pudieron leer las notas guardadas:\n"+err.Error(), "madPost", mbIconWarning)
	}
	if len(loaded) == 0 {
		newNote()
	}
	var placed []Note
	for _, n := range loaded {
		if !isVisibleOnScreen(n) { // p. ej. estaba en un monitor ya desconectado
			n.W, n.H = max(n.W, px(160)), max(n.H, px(120))
			n.X, n.Y = nextNotePosition(placed, n.W, n.H, workArea())
		}
		createNoteWindow(n)
		placed = append(placed, n)
	}

	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// acquireSingleInstance evita dos copias con las mismas notas. Si ya hay una
// corriendo, le pide que cree una nota nueva y termina.
func acquireSingleInstance() bool {
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(wstr("madPost.SingleInstance"))))
	if h != 0 && err == syscall.ERROR_ALREADY_EXISTS {
		if other, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(wstr(trayClass))), 0); other != 0 {
			procPostMessageW.Call(other, wmNewNote, 0, 0)
		}
		return false
	}
	instanceMutex = h
	return true
}

// isVisibleOnScreen comprueba que la barra de la nota caiga dentro del escritorio virtual.
func isVisibleOnScreen(n Note) bool {
	vx, vy := systemMetric(smXVirtualScreen), systemMetric(smYVirtualScreen)
	vw, vh := systemMetric(smCXVirtualScreen), systemMetric(smCYVirtualScreen)
	return n.W > 0 && n.H > 0 &&
		n.X+n.W > vx+px(40) && n.X < vx+vw-px(40) &&
		n.Y >= vy && n.Y < vy+vh-barHeight()
}
