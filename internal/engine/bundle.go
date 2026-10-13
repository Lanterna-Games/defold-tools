package engine

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

const (
	bundleDirectory      = "reference"
	BundledFileExtension = ".json.gz"
)

//go:embed reference/*.json.gz
var bundle embed.FS

func BundledVersions() []Version {
	entries, err := bundle.ReadDir(bundleDirectory)
	if err != nil {
		return nil
	}
	var versions []Version
	for _, entry := range entries {
		name, isAReference := strings.CutSuffix(entry.Name(), BundledFileExtension)
		if version, isAVersion := ParseVersion(name); isAReference && isAVersion {
			versions = append(versions, version)
		}
	}
	slices.SortFunc(versions, Version.Compare)
	return versions
}

func NewestBundled() (version Version, hasAny bool) {
	versions := BundledVersions()
	if len(versions) == 0 {
		return Version{}, false
	}
	return versions[len(versions)-1], true
}

func Bundled() (*Reference, error) {
	newest, hasAny := NewestBundled()
	if !hasAny {
		return nil, errors.New("this build carries no Defold reference")
	}
	return bundledFor(newest)
}

func bundledFor(version Version) (*Reference, error) {
	packed, err := bundle.ReadFile(bundleDirectory + "/" + version.String() + BundledFileExtension)
	if err != nil {
		return nil, fmt.Errorf("reading the bundled reference of Defold %s: %w", version, err)
	}
	return Unpack(packed)
}

func Unpack(packed []byte) (*Reference, error) {
	unpacker, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("unpacking a Defold reference: %w", err)
	}
	defer unpacker.Close()
	encoded, err := io.ReadAll(unpacker)
	if err != nil {
		return nil, fmt.Errorf("unpacking a Defold reference: %w", err)
	}
	var reference Reference
	if err := json.Unmarshal(encoded, &reference); err != nil {
		return nil, fmt.Errorf("reading a Defold reference: %w", err)
	}
	return &reference, nil
}

func Pack(reference *Reference) ([]byte, error) {
	encoded, err := json.Marshal(reference)
	if err != nil {
		return nil, fmt.Errorf("encoding the reference of Defold %s: %w", reference.Defold, err)
	}
	var packed bytes.Buffer
	packer, err := gzip.NewWriterLevel(&packed, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := packer.Write(encoded); err != nil {
		return nil, err
	}
	if err := packer.Close(); err != nil {
		return nil, err
	}
	return packed.Bytes(), nil
}
