package main

import "testing"

func TestToTasks(t *testing.T) {
	cases := map[string]string{
		"":                 "☐ ",
		"   ":              "☐ ",
		"pan":              "☐ pan",
		"pan\r\n\r\nleche": "☐ pan\r\n\r\n☐ leche",
		"☒ hecho\r\nnuevo": "☒ hecho\r\n☐ nuevo",
		"☑ antigua":        "☒ antigua",
		"☐ ya era tarea":   "☐ ya era tarea",
	}
	for in, want := range cases {
		if got := toTasks(in); got != want {
			t.Errorf("toTasks(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestFromTasks(t *testing.T) {
	cases := map[string]string{
		"☐ pan\r\n☒ leche": "pan\r\nleche",
		"☑ antigua":        "antigua",
		"☐pegado":          "pegado",
		"sin casilla":      "sin casilla",
		"☐ ":               "",
	}
	for in, want := range cases {
		if got := fromTasks(in); got != want {
			t.Errorf("fromTasks(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestNormalizeTasks(t *testing.T) {
	in := "☑ a\r\n☐ b\r\ntexto con ☑ en medio"
	want := "☒ a\r\n☐ b\r\ntexto con ☑ en medio"
	if got := normalizeTasks(in); got != want {
		t.Errorf("normalizeTasks = %q, quiero %q", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	text := "Comprar café\r\nLlamar a Juan\r\n\r\nRevisar PR"
	if got := fromTasks(toTasks(text)); got != text {
		t.Errorf("ida y vuelta: %q, quiero %q", got, text)
	}
}

func TestTaskProgress(t *testing.T) {
	done, total := taskProgress("☒ a\r\n☐ b\r\nnota suelta\r\n☑ c")
	if done != 2 || total != 3 {
		t.Errorf("taskProgress = %d/%d, quiero 2/3", done, total)
	}
}
