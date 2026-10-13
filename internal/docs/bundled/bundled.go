package bundled

import (
	"embed"
	"errors"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

const FileExtension = ".docs.json.gz"

//go:embed *.docs.json.gz
var files embed.FS

func Version() (version string, hasOne bool) {
	entries, err := files.ReadDir(".")
	if err != nil || len(entries) == 0 {
		return "", false
	}
	return strings.TrimSuffix(entries[len(entries)-1].Name(), FileExtension), true
}

func Documentation() (*docs.Reference, error) {
	version, hasOne := Version()
	if !hasOne {
		return nil, errors.New("this build carries no documentation")
	}
	packed, err := files.ReadFile(version + FileExtension)
	if err != nil {
		return nil, err
	}
	return docs.Unpack(packed)
}
