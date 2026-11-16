//go:build windows

package main

import (
	"strings"
	"testing"
	"unsafe"
)

// Usa una clave propia, nunca la clave Run real del usuario.
const testKeyPath = `Software\madPost-test`

func TestRegistryRoundTrip(t *testing.T) {
	t.Cleanup(func() {
		p := advapi32.NewProc("RegDeleteKeyW")
		p.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(wstr(testKeyPath))))
	})

	want := `"C:\Ruta con espacios\madPost.exe"`
	if err := regWriteString(testKeyPath, runValue, want); err != nil {
		t.Fatal(err)
	}
	if got, ok := regReadString(testKeyPath, runValue); !ok || got != want {
		t.Fatalf("leído %q (existe=%v), quiero %q", got, ok, want)
	}
	if err := regDeleteValue(testKeyPath, runValue); err != nil {
		t.Fatal(err)
	}
	if _, ok := regReadString(testKeyPath, runValue); ok {
		t.Fatal("el valor sigue existiendo tras borrarlo")
	}
	if err := regDeleteValue(testKeyPath, runValue); err != nil {
		t.Fatalf("borrar un valor inexistente debería ser inocuo: %v", err)
	}
}

func TestAutostartCommandQuoted(t *testing.T) {
	cmd, err := autostartCommand()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cmd, `"`) || !strings.HasSuffix(cmd, `.exe"`) {
		t.Fatalf("comando sin comillas o sin .exe: %s", cmd)
	}
}
