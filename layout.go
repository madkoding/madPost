package main

// nextNotePosition decide en qué coordenadas de pantalla (esquina superior
// izquierda) aparece una nota nueva de tamaño w×h.
//
//   - existing: las notas que ya están en pantalla (X, Y, W, H en píxeles reales).
//   - area: el área de trabajo del monitor principal (sin la barra de tareas).
//     Ojo: Left/Top no siempre son 0 (barra de tareas arriba o a la izquierda).
//
// También se usa al arrancar para recolocar notas que quedaron fuera de pantalla.
func nextNotePosition(existing []Note, w, h int32, area RECT) (x, y int32) {
	// TODO(human): elegir la posición de la nota nueva.
	// Provisional: todas aparecen en el mismo lugar (y se tapan entre sí).
	return area.Left + px(80), area.Top + px(80)
}
