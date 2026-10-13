package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/docs/bundled"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	repositoryOfDefold  = "https://github.com/defold/defold.git"
	prefixOfATag        = "refs/tags/"
	suffixOfAPeeledTag  = "^{}"
	timeToListTheTags   = time.Minute
	archivesDirectory   = "archives"
	cacheDirectoryOfOur = "defold-vet"
)

type release struct {
	version engine.Version
	sha1    string
}

func main() {
	output := flag.String("output", "internal/engine/reference", "the directory to write the references into")
	documentation := flag.String("documentation", "internal/docs/bundled", "the directory to write the documentation of the newest version into")
	archives := flag.String("archives", "", "where downloaded archives are kept between runs (default: the user's cache directory)")
	flag.Parse()

	if err := run(*output, *documentation, *archives); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

func run(output, documentation, archives string) error {
	if archives == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		archives = filepath.Join(cache, cacheDirectoryOfOur, archivesDirectory)
	}
	releases, err := supportedReleases()
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		return errors.New("the Defold repository lists no version in the supported window")
	}

	references := make([]*engine.Reference, 0, len(releases))
	for _, published := range releases {
		reference, err := referenceOf(published, archives)
		if err != nil {
			return err
		}
		references = append(references, reference)
	}
	engine.FillHoles(references)
	engine.RememberSettingsOfOtherVersions(references)
	if err := write(references, output); err != nil {
		return err
	}
	return writeDocumentation(releases[len(releases)-1], archives, documentation)
}

func writeDocumentation(newest release, archives, output string) error {
	archive, err := os.ReadFile(filepath.Join(archives, newest.sha1+"-ref-doc.zip")) //nolint:gosec // a file this tool wrote itself, under the directory named on its command line
	if err != nil {
		return err
	}
	reference, err := docs.FromArchive(newest.version.String(), archive)
	if err != nil {
		return err
	}
	packed, err := docs.Pack(reference)
	if err != nil {
		return err
	}
	stale, err := filepath.Glob(filepath.Join(output, "*"+bundled.FileExtension))
	if err != nil {
		return err
	}
	for _, file := range stale {
		if err := os.Remove(file); err != nil {
			return err
		}
	}
	fmt.Printf("documentation of Defold %s: %d modules, %d bytes\n", reference.Version, len(reference.Modules), len(packed))
	return os.WriteFile(filepath.Join(output, reference.Version+bundled.FileExtension), packed, 0o600) //nolint:gosec // the directory is named on the command line of this tool and the version comes from a tag that was parsed as three numbers
}

func supportedReleases() ([]release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeToListTheTags)
	defer cancel()
	listed, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", repositoryOfDefold).Output()
	if err != nil {
		return nil, fmt.Errorf("listing the tags of %s: %w", repositoryOfDefold, err)
	}
	commitOf := map[engine.Version]string{}
	isPeeled := map[engine.Version]bool{}
	lines := bufio.NewScanner(bytes.NewReader(listed))
	for lines.Scan() {
		sha1, reference, isATag := strings.Cut(lines.Text(), "\t")
		name, isUnderTags := strings.CutPrefix(reference, prefixOfATag)
		if !isATag || !isUnderTags {
			continue
		}
		name, peeled := strings.CutSuffix(name, suffixOfAPeeledTag)
		version, isAVersion := engine.ParseVersion(name)
		if !isAVersion || version.IsBefore(engine.OldestSupported) || (isPeeled[version] && !peeled) {
			continue
		}
		commitOf[version], isPeeled[version] = sha1, peeled
	}
	releases := make([]release, 0, len(commitOf))
	for version, sha1 := range commitOf {
		releases = append(releases, release{version: version, sha1: sha1})
	}
	slices.SortFunc(releases, func(a, b release) int { return a.version.Compare(b.version) })
	return releases, nil
}

func referenceOf(published release, archives string) (*engine.Reference, error) {
	referencePath := filepath.Join(archives, published.sha1+"-ref-doc.zip")
	builtinsPath := filepath.Join(archives, published.sha1+"-builtins.zip")
	referenceArchive, errOfReference := os.ReadFile(referencePath) //nolint:gosec // a file this tool wrote itself, under the directory named on its command line
	builtinsArchive, errOfBuiltins := os.ReadFile(builtinsPath)    //nolint:gosec // as above
	if errOfReference != nil || errOfBuiltins != nil {
		fmt.Printf("fetching Defold %s\n", published.version)
		var err error
		referenceArchive, builtinsArchive, err = engine.Published().Archives(http.DefaultClient, published.sha1)
		if err != nil {
			return nil, fmt.Errorf("fetching Defold %s: %w", published.version, err)
		}
		if err := keep(archives, referencePath, referenceArchive, builtinsPath, builtinsArchive); err != nil {
			return nil, err
		}
	}
	reference, err := engine.FromArchives(published.version.String(), published.sha1, referenceArchive, builtinsArchive)
	if err != nil {
		return nil, err
	}
	schemaPath := filepath.Join(archives, published.sha1+"-meta.properties")
	schema, err := os.ReadFile(schemaPath) //nolint:gosec // as above
	if err != nil {
		fmt.Printf("fetching the settings of Defold %s\n", published.version)
		schema, err = engine.Published().SettingsSchema(fetched, published.version.String())
		if err != nil {
			return nil, fmt.Errorf("fetching the settings of Defold %s: %w", published.version, err)
		}
		if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
			return nil, err
		}
	}
	reference.Settings = engine.SettingsFrom(schema)
	return reference, nil
}

func fetched(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeToListTheTags)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func keep(directory, referencePath string, referenceArchive []byte, builtinsPath string, builtinsArchive []byte) error {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(referencePath, referenceArchive, 0o600); err != nil {
		return err
	}
	return os.WriteFile(builtinsPath, builtinsArchive, 0o600)
}

func write(references []*engine.Reference, output string) error {
	stale, err := filepath.Glob(filepath.Join(output, "*"+engine.BundledFileExtension))
	if err != nil {
		return err
	}
	for _, file := range stale {
		if err := os.Remove(file); err != nil {
			return err
		}
	}
	for _, reference := range references {
		packed, err := engine.Pack(reference)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, reference.Defold+engine.BundledFileExtension), packed, 0o600); err != nil {
			return fmt.Errorf("writing the reference of Defold %s: %w", reference.Defold, err)
		}
		fmt.Printf("Defold %-8s %4d functions %2d messages %3d holes filled %6d bytes\n",
			reference.Defold, len(reference.Functions), len(reference.Messages), len(reference.Filled), len(packed))
	}
	return nil
}
