package main

import "strings"

// Modo "lista de tareas": cada línea empieza con una casilla.
//
// Se guardan como ☐ (pendiente) y ☒ (hecha): ambos vienen de la misma fuente
// (Segoe UI Symbol) y miden lo mismo, así el texto de todas las tareas queda
// alineado. ☑ no sirve: Windows lo dibuja con la fuente de emoji, más grande.
// Encima de estos caracteres la nota dibuja sus propias casillas (todo.go).
const (
	boxOpen    = '☐'
	boxDone    = '☒'
	boxDoneOld = '☑' // versiones anteriores; se convierte al abrir la nota
)

const (
	taskPrefixOpen = string(boxOpen) + " "
	taskPrefixDone = string(boxDone) + " "
)

// El control EDIT de Windows separa las líneas con \r\n.
const lineSep = "\r\n"

func isBox(r rune) bool  { return r == boxOpen || r == boxDone || r == boxDoneOld }
func isDone(r rune) bool { return r == boxDone || r == boxDoneOld }

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func isTaskLine(line string) bool { return isBox(firstRune(line)) }

// toTasks convierte cada línea con texto en una tarea pendiente. Las que ya
// tienen casilla se respetan. Una nota vacía queda con una tarea en blanco.
func toTasks(text string) string {
	if strings.TrimSpace(text) == "" {
		return taskPrefixOpen
	}
	lines := strings.Split(text, lineSep)
	for i, l := range lines {
		if strings.TrimSpace(l) != "" && !isTaskLine(l) {
			lines[i] = taskPrefixOpen + l
		}
	}
	return normalizeTasks(strings.Join(lines, lineSep))
}

// fromTasks quita las casillas y deja el texto tal cual.
func fromTasks(text string) string {
	lines := strings.Split(text, lineSep)
	for i, l := range lines {
		if r := firstRune(l); isBox(r) {
			lines[i] = strings.TrimPrefix(l[len(string(r)):], " ")
		}
	}
	return strings.Join(lines, lineSep)
}

// normalizeTasks cambia las casillas ☑ antiguas por ☒.
func normalizeTasks(text string) string {
	lines := strings.Split(text, lineSep)
	for i, l := range lines {
		if firstRune(l) == boxDoneOld {
			lines[i] = string(boxDone) + l[len(string(boxDoneOld)):]
		}
	}
	return strings.Join(lines, lineSep)
}

// taskProgress cuenta las tareas hechas y el total.
func taskProgress(text string) (done, total int) {
	for _, l := range strings.Split(text, lineSep) {
		if r := firstRune(l); isBox(r) {
			total++
			if isDone(r) {
				done++
			}
		}
	}
	return done, total
}
