package pls

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/Lanterna-Games/defold-tools/internal/annotations"
	"github.com/Lanterna-Games/defold-tools/internal/config"
	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/docs/bundled"
	"github.com/Lanterna-Games/defold-tools/internal/docserver"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	cacheDirectoryOfOurs = "defold-vet"
	annotationsDirectory = "annotations"
	portFile             = "reference-port"
)

type workshop struct {
	cache    string
	versions func(announce func(message string)) engine.Versions
	store    func(announce func(message string)) docs.Store

	starting  sync.Mutex
	reference string
}

func workshopOfThisMachine() *workshop {
	cache := ""
	if userCache, err := os.UserCacheDir(); err == nil {
		cache = filepath.Join(userCache, cacheDirectoryOfOurs)
	}
	return &workshop{
		cache:    cache,
		versions: engine.VersionsOfThisMachine,
		store: func(announce func(message string)) docs.Store {
			return docs.Store{
				Archives: docs.ArchivesOfThisMachine(),
				Carried:  carriedDocumentation,
				Fetch: func(sha1 string) ([]byte, error) {
					return engine.Published().ReferenceArchive(http.DefaultClient, sha1)
				},
				Announce: announce,
			}
		},
	}
}

func carriedDocumentation() (version string, documentation func() (*docs.Reference, error), hasOne bool) {
	version, hasOne = bundled.Version()
	return version, bundled.Documentation, hasOne
}

func (w *workshop) versionOf(directory string) (string, error) {
	settings, err := config.Load(os.DirFS(directory))
	if err != nil {
		return "", err
	}
	return settings.Defold, nil
}

func (w *workshop) documentationOf(declared string, announce func(message string)) (*docs.Reference, error) {
	reference, err := w.versions(announce).For(declared)
	if err != nil {
		return nil, err
	}
	return w.store(announce).For(reference.Defold, reference.SHA1)
}

func (w *workshop) versionsNewestFirst() []string {
	carried := engine.BundledVersions()
	slices.Reverse(carried)
	versions := make([]string, 0, len(carried))
	for _, version := range carried {
		versions = append(versions, version.String())
	}
	return versions
}

func (w *workshop) startTheReference(ctx context.Context, announce func(message string)) (address string, err error) {
	w.starting.Lock()
	defer w.starting.Unlock()
	if w.reference != "" {
		return w.reference, nil
	}
	server := &docserver.Server{
		Versions: w.versionsNewestFirst,
		Documentation: func(version string) (*docs.Reference, error) {
			return w.documentationOf(version, announce)
		},
	}
	if w.cache != "" {
		server.PortFile = filepath.Join(w.cache, portFile)
	}
	w.reference, err = server.Start(ctx)
	return w.reference, err
}

func (w *workshop) annotationsDirectoryOf(version string) (string, error) {
	if w.cache == "" {
		return "", errors.New("this machine has no cache directory to keep the annotations in")
	}
	return filepath.Join(w.cache, annotationsDirectory, version), nil
}

func writeAnnotations(reference *docs.Reference, links docs.Links, directory string) error {
	files := annotations.Generate(reference, links)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	stale, err := filepath.Glob(filepath.Join(directory, "*.lua"))
	if err != nil {
		return err
	}
	for _, file := range stale {
		if _, isStillWritten := files[filepath.Base(file)]; !isStillWritten {
			if err := os.Remove(file); err != nil {
				return err
			}
		}
	}
	for name, content := range files {
		target := filepath.Join(directory, name)
		if current, err := os.ReadFile(target); err == nil && string(current) == content { //nolint:gosec // a file this program writes itself, in its own cache directory
			continue
		}
		halfWritten := target + ".part"
		if err := os.WriteFile(halfWritten, []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}
		if err := os.Rename(halfWritten, target); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}
	}
	return nil
}
