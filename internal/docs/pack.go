package docs

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
)

func Pack(reference *Reference) ([]byte, error) {
	encoded, err := json.Marshal(reference)
	if err != nil {
		return nil, fmt.Errorf("encoding the documentation of Defold %s: %w", reference.Version, err)
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

func Unpack(packed []byte) (*Reference, error) {
	unpacker, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("unpacking the documentation: %w", err)
	}
	defer unpacker.Close()
	encoded, err := io.ReadAll(unpacker)
	if err != nil {
		return nil, fmt.Errorf("unpacking the documentation: %w", err)
	}
	var reference Reference
	if err := json.Unmarshal(encoded, &reference); err != nil {
		return nil, fmt.Errorf("reading the documentation: %w", err)
	}
	return &reference, nil
}
