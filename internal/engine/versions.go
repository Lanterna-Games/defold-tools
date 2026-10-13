package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	tagsOfDefold          = "https://api.github.com/repos/defold/defold/git"
	archiveOfDefold       = "https://d.defold.com/archive/stable"
	sourcesOfDefold       = "https://raw.githubusercontent.com/defold/defold"
	referenceArchivePath  = "%s/%s/engine/share/ref-doc.zip"
	builtinsArchivePath   = "%s/%s/engine/share/builtins.zip"
	cacheDirectoryOfOurs  = "defold-vet"
	cacheDirectoryOfThese = "reference"
	timeToFetch           = 2 * time.Minute
)

type Publisher struct {
	Tags    string
	Archive string
	Sources string
}

func Published() Publisher {
	return Publisher{Tags: tagsOfDefold, Archive: archiveOfDefold, Sources: sourcesOfDefold}
}

type Versions struct {
	Publisher      Publisher
	Client         *http.Client
	CacheDirectory string
	Announce       func(message string)
}

func VersionsOfThisMachine(announce func(message string)) Versions {
	versions := Versions{Publisher: Published(), Client: http.DefaultClient, Announce: announce}
	if cache, err := os.UserCacheDir(); err == nil {
		versions.CacheDirectory = filepath.Join(cache, cacheDirectoryOfOurs, cacheDirectoryOfThese)
	}
	return versions
}

func (v Versions) For(declared string) (*Reference, error) {
	newest, hasAny := NewestBundled()
	if !hasAny {
		return nil, errors.New("this build carries no Defold reference")
	}
	if declared == "" {
		return bundledFor(newest)
	}
	version, isAVersion := ParseVersion(declared)
	switch {
	case !isAVersion:
		return nil, fmt.Errorf("%q is not a Defold version: write it as in %q", declared, newest.String())
	case version.IsBefore(OldestSupported):
		return nil, fmt.Errorf("the project targets Defold %s, and the oldest version defold-vet supports is %s", version, OldestSupported)
	case slices.Contains(BundledVersions(), version):
		return bundledFor(version)
	}
	return v.fetched(version.String())
}

func (v Versions) fetched(version string) (*Reference, error) {
	if cached, isCached := v.cached(version); isCached {
		return cached, nil
	}
	if v.Announce != nil {
		v.Announce("fetching the reference of Defold " + version + ", which this build does not carry")
	}
	sha1, err := v.Publisher.sha1Of(v.Client, version)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("version %s of Defold was never published", version)
	}
	if err != nil {
		return nil, fmt.Errorf("looking for Defold %s: %w", version, err)
	}
	reference, err := v.Publisher.Download(v.Client, version, sha1)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("version %s of Defold was published without the reference archives this build reads (%s/%s)", version, v.Publisher.Archive, sha1)
	}
	if err != nil {
		return nil, err
	}
	v.keep(reference)
	return reference, nil
}

func (v Versions) cacheFileOf(version string) string {
	return filepath.Join(v.CacheDirectory, version+".json")
}

func (v Versions) cached(version string) (reference *Reference, isCached bool) {
	if v.CacheDirectory == "" {
		return nil, false
	}
	content, err := os.ReadFile(v.cacheFileOf(version))
	if err != nil {
		return nil, false
	}
	var read Reference
	if err := json.Unmarshal(content, &read); err != nil || read.Defold != version {
		return nil, false
	}
	return &read, true
}

func (v Versions) keep(reference *Reference) {
	if v.CacheDirectory == "" {
		return
	}
	encoded, err := json.Marshal(reference)
	if err != nil {
		return
	}
	if err := os.MkdirAll(v.CacheDirectory, 0o750); err != nil {
		return
	}
	_ = os.WriteFile(v.cacheFileOf(reference.Defold), encoded, 0o600)
}

type gitObject struct {
	Object struct {
		Type string `json:"type"`
		SHA  string `json:"sha"`
	} `json:"object"`
}

func (p Publisher) sha1Of(client *http.Client, version string) (string, error) {
	var tag gitObject
	if err := fetchJSON(client, p.Tags+"/ref/tags/"+version, &tag); err != nil {
		return "", err
	}
	if tag.Object.Type != "tag" {
		return tag.Object.SHA, nil
	}
	var annotated gitObject
	if err := fetchJSON(client, p.Tags+"/tags/"+tag.Object.SHA, &annotated); err != nil {
		return "", err
	}
	return annotated.Object.SHA, nil
}

func (p Publisher) Download(client *http.Client, version, sha1 string) (*Reference, error) {
	referenceArchive, builtinsArchive, err := p.Archives(client, sha1)
	if err != nil {
		return nil, err
	}
	reference, err := FromArchives(version, sha1, referenceArchive, builtinsArchive)
	if err != nil || p.Sources == "" {
		return reference, err
	}
	schema, err := p.SettingsSchema(func(url string) ([]byte, error) { return fetch(client, url) }, version)
	if err != nil {
		return nil, err
	}
	reference.Settings = SettingsFrom(schema)
	return reference, nil
}

func (p Publisher) ReferenceArchive(client *http.Client, sha1 string) ([]byte, error) {
	return fetch(client, fmt.Sprintf(referenceArchivePath, p.Archive, sha1))
}

func (p Publisher) Archives(client *http.Client, sha1 string) (referenceArchive, builtinsArchive []byte, err error) {
	referenceArchive, err = p.ReferenceArchive(client, sha1)
	if err != nil {
		return nil, nil, err
	}
	builtinsArchive, err = fetch(client, fmt.Sprintf(builtinsArchivePath, p.Archive, sha1))
	if err != nil {
		return nil, nil, err
	}
	return referenceArchive, builtinsArchive, nil
}

func fetchJSON(client *http.Client, url string, into any) error {
	content, err := fetch(client, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeToFetch)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("preparing to fetch %s: %w", url, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("fetching %s: %w", url, fs.ErrNotExist)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", url, response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	return content, nil
}
