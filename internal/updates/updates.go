package updates

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
)

type moduleLine struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Main     bool   `json:"Main"`
	Indirect bool   `json:"Indirect"`
	Update   *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
	} `json:"Update"`
}

type DepUpdate struct {
	Path     string `json:"path"`
	Current  string `json:"currentVersion"`
	Latest   string `json:"latestVersion"`
	Indirect bool   `json:"indirect"`
}

func Parse(r io.Reader) ([]DepUpdate, error) {
	dec := json.NewDecoder(r)
	var out []DepUpdate
	for {
		var m moduleLine
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if m.Main || m.Update == nil {
			continue
		}

		latest := m.Update.Version
		if latest == "" {
			continue
		}

		out = append(out, DepUpdate{
			Path:     m.Path,
			Current:  m.Version,
			Latest:   latest,
			Indirect: m.Indirect,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}
