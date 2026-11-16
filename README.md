# madPost

Post-its para el escritorio de Windows, escrito en Go puro (sin CGO ni dependencias externas: llama a Win32 con `syscall`).

## Compilar

Requiere Go 1.22 o superior.

```powershell
go build -ldflags "-H windowsgui -s -w" -o madPost.exe .
```

`-H windowsgui` hace que el `.exe` no abra una consola al ejecutarse.

El ícono (del `.exe` y de la bandeja) está en `rsrc_windows_amd64.syso`, que `go build` incrusta solo.
Lo dibuja `tools/mkicon` por código; si lo cambias, regenéralo con:

```powershell
go generate
```

## Uso

- **Arrastrar**: desde la barra superior de la nota.
- **Redimensionar**: desde los bordes o el triángulo de puntos de abajo a la derecha.
- **`+`**: nueva nota · **`⋯`** (o clic derecho en la franja): color, *Lista de tareas*, *Siempre visible*, duplicar, eliminar · **`✕`**: eliminar.
- **Lista de tareas**: cada línea es una tarea `☐`; clic en la casilla para marcarla `☑`, *Enter* crea otra. La franja muestra el progreso (`2/5`). Al desactivarla se quitan las casillas y el texto queda igual.
- **Duplicar nota**: copia texto, color, tamaño y modo, un poco desplazada.
- **Ícono de bandeja**: clic = traer notas al frente · doble clic = nueva nota · clic derecho = menú / Salir.
- Si vuelves a ejecutar `madPost.exe` mientras ya está abierto, se crea una nota nueva en la instancia existente.

Las notas se guardan solas (texto ~0.8 s después de dejar de escribir; posición y tamaño al soltar) en
`%APPDATA%\madPost\notes.json`.

## Iniciar con Windows

Clic derecho en el ícono de la bandeja → **Iniciar con Windows**. Se guarda en
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (solo para tu usuario, sin permisos de administrador).
Si mueves `madPost.exe` a otra carpeta, al abrirlo de nuevo actualiza la ruta solo.

## Estructura

| Archivo     | Qué hace                                                    |
|-------------|-------------------------------------------------------------|
| `main.go`   | Arranque, instancia única y bucle de mensajes               |
| `note.go`   | Ventana de cada nota: pintado, arrastre, menú, guardado     |
| `tray.go`   | Ícono en la bandeja del sistema                             |
| `store.go`  | Lectura y escritura del JSON                                |
| `layout.go` | Dónde aparece cada nota nueva                               |
| `style.go`  | Sombra translúcida, esquina doblada y degradados            |
| `win32.go`  | Envoltorios mínimos de la API Win32                         |
| `tools/mkicon` | Genera `icon.ico` y el recurso `.syso` del ícono         |
