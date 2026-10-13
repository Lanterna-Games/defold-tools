package docs

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	cacheDirectoryOfOurs = "defold-vet"
	archivesDirectory    = "archives"
	suffixOfAnArchive    = "-ref-doc.zip"
)

type Store struct {
	Archives string
	Carried  func() (version string, documentation func() (*Reference, error), hasOne bool)
	Fetch    func(sha1 string) ([]byte, error)
	Announce func(message string)
}

func ArchivesOfThisMachine() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, cacheDirectoryOfOurs, archivesDirectory)
}

func (s Store) IsAtHand(version, sha1 string) bool {
	if s.Carried != nil {
		if carried, _, hasOne := s.Carried(); hasOne && carried == version {
			return true
		}
	}
	_, err := os.Stat(s.archiveOf(sha1))
	return s.Archives != "" && err == nil
}

func (s Store) archiveOf(sha1 string) string {
	return filepath.Join(s.Archives, sha1+suffixOfAnArchive)
}

func (s Store) For(version, sha1 string) (*Reference, error) {
	if s.Carried != nil {
		if carried, documentation, hasOne := s.Carried(); hasOne && carried == version {
			return documentation()
		}
	}
	if s.Archives != "" {
		if archive, err := os.ReadFile(s.archiveOf(sha1)); err == nil {
			return FromArchive(version, archive)
		}
	}
	if s.Fetch == nil {
		return nil, fmt.Errorf("the documentation of Defold %s is not on this machine", version)
	}
	if s.Announce != nil {
		s.Announce("fetching the documentation of Defold " + version)
	}
	archive, err := s.Fetch(sha1)
	if err != nil {
		return nil, fmt.Errorf("fetching the documentation of Defold %s: %w", version, err)
	}
	reference, err := FromArchive(version, archive)
	if err != nil {
		return nil, err
	}
	s.keep(sha1, archive)
	return reference, nil
}

func (s Store) keep(sha1 string, archive []byte) {
	if s.Archives == "" {
		return
	}
	if err := os.MkdirAll(s.Archives, 0o750); err != nil {
		return
	}
	_ = os.WriteFile(s.archiveOf(sha1), archive, 0o600)
}
