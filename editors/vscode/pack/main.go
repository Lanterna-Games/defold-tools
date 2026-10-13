package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	contentTypesFile   = "[Content_Types].xml"
	manifestFile       = "extension.vsixmanifest"
	iconFile           = "icon.png"
	readmeInThePackage = "extension/readme.md"
	contentTypes       = `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension=".css" ContentType="text/css"/><Default Extension=".json" ContentType="application/json"/><Default Extension=".js" ContentType="application/javascript"/><Default Extension=".md" ContentType="text/markdown"/><Default Extension=".png" ContentType="image/png"/><Default Extension=".txt" ContentType="text/plain"/><Default Extension=".vsixmanifest" ContentType="text/xml"/></Types>
`
	manifest = `<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011">
  <Metadata>
    <Identity Language="en-US" Id="%s" Version="%s" Publisher="%s"/>
    <DisplayName>%s</DisplayName>
    <Description xml:space="preserve">%s</Description>
    <Categories>%s</Categories>
    <Icon>extension/icon.png</Icon>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value="%s"/>
      <Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="%s"/>
    </Properties>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code"/>
  </Installation>
  <Dependencies/>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true"/>
    <Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/readme.md" Addressable="true"/>
    <Asset Type="Microsoft.VisualStudio.Services.Content.License" Path="extension/LICENSE.txt" Addressable="true"/>
    <Asset Type="Microsoft.VisualStudio.Services.Icons.Default" Path="extension/icon.png" Addressable="true"/>
  </Assets>
</PackageManifest>
`
)

type description struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"displayName"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Publisher     string   `json:"publisher"`
	Categories    []string `json:"categories"`
	ExtensionKind []string `json:"extensionKind"`
	Engines       struct {
		VSCode string `json:"vscode"`
	} `json:"engines"`
}

func main() {
	source := flag.String("source", "editors/vscode", "the directory of the extension")
	license := flag.String("license", "LICENSE", "the licence to include")
	output := flag.String("output", "", "the package to write")
	flag.Parse()

	if err := pack(*source, *license, *output); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		os.Exit(1)
	}
}

func readmeWithItsIcon(source string) (string, error) {
	readme, err := os.ReadFile(filepath.Join(source, "README.md")) //nolint:gosec // the directory is the one named on the command line of this build tool
	if err != nil {
		return "", err
	}
	icon, err := os.ReadFile(filepath.Join(source, iconFile)) //nolint:gosec // as above
	if err != nil {
		return "", err
	}
	inline := `src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(icon) + `"`
	return strings.ReplaceAll(string(readme), `src="`+iconFile+`"`, inline), nil
}

func escaped(text string) string {
	var builder strings.Builder
	_ = xml.EscapeText(&builder, []byte(text))
	return builder.String()
}

func pack(source, license, output string) error {
	if output == "" {
		return errors.New("-output is required")
	}
	encoded, err := os.ReadFile(filepath.Join(source, "package.json")) //nolint:gosec // the directory is the one named on the command line of this build tool
	if err != nil {
		return err
	}
	var described description
	if err := json.Unmarshal(encoded, &described); err != nil {
		return fmt.Errorf("reading package.json: %w", err)
	}

	files := map[string]string{
		"extension/package.json":   filepath.Join(source, "package.json"),
		"extension/extension.js":   filepath.Join(source, "extension.js"),
		"extension/fileView.js":    filepath.Join(source, "fileView.js"),
		"extension/media/view.js":  filepath.Join(source, "media", "view.js"),
		"extension/media/view.css": filepath.Join(source, "media", "view.css"),
		"extension/icon.png":       filepath.Join(source, iconFile),
		"extension/LICENSE.txt":    license,
	}
	readme, err := readmeWithItsIcon(source)
	if err != nil {
		return err
	}
	generated := map[string]string{
		readmeInThePackage: readme,
		contentTypesFile:   contentTypes,
		manifestFile: fmt.Sprintf(manifest,
			escaped(described.Name), escaped(described.Version), escaped(described.Publisher),
			escaped(described.DisplayName), escaped(described.Description), escaped(strings.Join(described.Categories, ",")),
			escaped(described.Engines.VSCode), escaped(strings.Join(described.ExtensionKind, ","))),
	}

	var packed bytes.Buffer
	archive := zip.NewWriter(&packed)
	if err := writeAll(archive, files, generated); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return err
	}
	return os.WriteFile(output, packed.Bytes(), 0o600)
}

func writeAll(archive *zip.Writer, files, generated map[string]string) error {
	for _, name := range []string{manifestFile, contentTypesFile, readmeInThePackage} {
		entry, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(entry, generated[name]); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content, err := os.ReadFile(files[name])
		if err != nil {
			return err
		}
		entry, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
			return err
		}
	}
	return nil
}
