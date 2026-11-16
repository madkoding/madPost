package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Note es lo que se persiste en disco por cada post-it.
type Note struct {
	ID      string `json:"id"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
	W       int32  `json:"w"`
	H       int32  `json:"h"`
	Text    string `json:"text"`
	Color   int    `json:"color"`
	Topmost bool   `json:"topmost"`
	Todo    bool   `json:"todo,omitempty"` // modo lista de tareas
}

// storePath devuelve %APPDATA%\madPost\notes.json
func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "madPost", "notes.json"), nil
}

func loadNotes() ([]Note, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Editores como el Bloc de notas añaden un BOM UTF-8 que json rechaza.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// saveNotes escribe a un archivo temporal y lo renombra, para que un corte
// a mitad de escritura nunca deje el JSON corrupto.
func saveNotes(notes []Note) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
