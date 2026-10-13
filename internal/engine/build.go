package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/inputbinding"
	"github.com/Lanterna-Games/defold-tools/internal/material"
	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	languageOfScripts         = "Lua"
	languageBeforeItWasStated = ""
)

var nameOfAScriptFunction = regexp.MustCompile(`^[a-z_][a-z0-9_]*([.:][a-z_][a-z0-9_]*)*$`)

type document struct {
	Info struct {
		Language  string `json:"language"`
		Namespace string `json:"namespace"`
	} `json:"info"`
	Elements []struct {
		Type       string      `json:"type"`
		Name       string      `json:"name"`
		Parameters []Parameter `json:"parameters"`
	} `json:"elements"`
}

func FromArchives(version, sha1 string, referenceArchive, builtinsArchive []byte) (*Reference, error) {
	reference, err := referenceFrom(referenceArchive)
	if err != nil {
		return nil, err
	}
	if err := addBuiltins(reference, builtinsArchive); err != nil {
		return nil, err
	}
	if len(reference.Functions) == 0 {
		return nil, fmt.Errorf("the reference of Defold %s documents no function: it is not written the way this build reads", version)
	}
	reference.Defold, reference.SHA1 = version, sha1
	return reference, nil
}

func isAFunctionOfScripts(language, name string) bool {
	if language == languageBeforeItWasStated {
		return nameOfAScriptFunction.MatchString(name)
	}
	return language == languageOfScripts
}

func referenceFrom(archive []byte) (*Reference, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("opening the reference archive: %w", err)
	}
	reference := &Reference{
		Functions:     map[string][]Parameter{},
		Properties:    map[string][]string{},
		MessageFields: map[string][]string{},
	}
	isMessage := map[string]bool{}
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, "_doc.json") {
			continue
		}
		parsed, err := readDocument(file)
		if err != nil {
			return nil, err
		}
		for _, element := range parsed.Elements {
			switch {
			case element.Type == "MESSAGE":
				isMessage[element.Name] = true
				for _, field := range element.Parameters {
					if !slices.Contains(reference.MessageFields[element.Name], field.Name) {
						reference.MessageFields[element.Name] = append(reference.MessageFields[element.Name], field.Name)
					}
				}
			case element.Type == "PROPERTY":
				componentType := ComponentTypeDocumentedIn(parsed.Info.Namespace)
				reference.Properties[componentType] = append(reference.Properties[componentType], element.Name)
			case element.Type == "FUNCTION" && isAFunctionOfScripts(parsed.Info.Language, element.Name):
				reference.Functions[element.Name] = element.Parameters
			}
		}
	}
	for message := range isMessage {
		reference.Messages = append(reference.Messages, message)
	}
	sort.Strings(reference.Messages)
	return reference, nil
}

func addBuiltins(reference *Reference, archive []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("opening the builtins archive: %w", err)
	}
	reference.InputBindings = map[string][]string{}
	for _, file := range reader.File {
		isMaterial, isInputBinding := strings.HasSuffix(file.Name, material.Extension), strings.HasSuffix(file.Name, inputbinding.Extension)
		if !isMaterial && !isInputBinding {
			continue
		}
		content, err := contentOf(file)
		if err != nil {
			return err
		}
		parsed, syntaxErrors := textproto.Parse(content, source.NewLocator(content))
		if len(syntaxErrors) > 0 {
			return fmt.Errorf("reading %s: %s", file.Name, syntaxErrors[0].Message)
		}
		if isMaterial {
			reference.MaterialProperties = append(reference.MaterialProperties, material.PropertyNames(parsed)...)
		} else {
			reference.InputBindings["/"+strings.TrimPrefix(file.Name, "/")] = inputbinding.Actions(parsed)
		}
	}
	sort.Strings(reference.MaterialProperties)
	reference.MaterialProperties = slices.Compact(reference.MaterialProperties)
	return nil
}

func contentOf(file *zip.File) ([]byte, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", file.Name, err)
	}
	defer opened.Close()
	content, err := io.ReadAll(opened)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file.Name, err)
	}
	return content, nil
}

func readDocument(file *zip.File) (*document, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", file.Name, err)
	}
	defer opened.Close()
	var parsed document
	if err := json.NewDecoder(opened).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("reading %s: %w", file.Name, err)
	}
	return &parsed, nil
}
