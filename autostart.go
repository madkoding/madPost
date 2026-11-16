//go:build windows

// Iniciar con Windows: un valor en HKCU\...\Run con la ruta del .exe.
// Es por usuario, no necesita permisos de administrador y Windows lo
// ejecuta al iniciar sesión.
package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")
	procRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW  = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue   = "madPost"

	hkeyCurrentUser = 0x80000001
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	regSZ           = 1
)

func regOpen(path string) (uintptr, error) {
	var key uintptr
	r, _, _ := procRegCreateKeyExW.Call(hkeyCurrentUser,
		uintptr(unsafe.Pointer(wstr(path))), 0, 0, 0,
		keyQueryValue|keySetValue, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return key, nil
}

// regReadString devuelve el valor (REG_SZ) y si existe.
func regReadString(path, name string) (string, bool) {
	key, err := regOpen(path)
	if err != nil {
		return "", false
	}
	defer procRegCloseKey.Call(key)

	var typ, size uint32
	namePtr := uintptr(unsafe.Pointer(wstr(name)))
	if r, _, _ := procRegQueryValueEx.Call(key, namePtr, 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size))); r != 0 || typ != regSZ {
		return "", false
	}
	buf := make([]uint16, size/2+1)
	if r, _, _ := procRegQueryValueEx.Call(key, namePtr, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func regWriteString(path, name, value string) error {
	key, err := regOpen(path)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(key)

	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	r, _, _ := procRegSetValueExW.Call(key, uintptr(unsafe.Pointer(wstr(name))), 0, regSZ,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2)) // bytes, con el NUL final
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func regDeleteValue(path, name string) error {
	key, err := regOpen(path)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(key)
	if r, _, _ := procRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(wstr(name)))); r != 0 && r != uintptr(syscall.ERROR_FILE_NOT_FOUND) {
		return syscall.Errno(r)
	}
	return nil
}

// autostartCommand es lo que Windows ejecutará: la ruta del .exe entre comillas
// (las rutas con espacios, como "OneDrive\Documentos", lo necesitan).
func autostartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return `"` + exe + `"`, nil
}

func autostartEnabled() bool {
	_, ok := regReadString(runKeyPath, runValue)
	return ok
}

func setAutostart(on bool) error {
	if !on {
		return regDeleteValue(runKeyPath, runValue)
	}
	cmd, err := autostartCommand()
	if err != nil {
		return err
	}
	return regWriteString(runKeyPath, runValue, cmd)
}

// refreshAutostart corrige la ruta si el autoinicio está activo pero el
// .exe se movió de carpeta desde que se activó.
func refreshAutostart() {
	current, ok := regReadString(runKeyPath, runValue)
	if !ok {
		return
	}
	if cmd, err := autostartCommand(); err == nil && cmd != current {
		regWriteString(runKeyPath, runValue, cmd)
	}
}
