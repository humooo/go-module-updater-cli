package modinfo

import (
	"errors"
	"fmt"

	"golang.org/x/mod/modfile"
)

type Info struct {
	Module    string
	GoVersion string
}

func Parse(data []byte) (Info, error) {
	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return Info{}, fmt.Errorf("parse go.mod: %w", err)
	}
	if modFile.Module == nil || modFile.Go == nil {
		return Info{}, errors.New("go.mod has no module or go directive")
	}
	if modFile.Module.Mod.Path == "" || modFile.Go.Version == "" {
		return Info{}, errors.New("go.mod has no module or go directive")
	}
	return Info{
		Module:    modFile.Module.Mod.Path,
		GoVersion: modFile.Go.Version,
	}, nil
}
