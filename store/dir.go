package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// DirStore implements Store using one file per key in a directory.
// Keys are mapped to filenames by escaping path separators.
// Writes are atomic (temp file + rename). No long-running locks are held.
type DirStore struct {
	dir string
}

// NewDirStore creates a DirStore rooted at dir. The directory must already exist.
func NewDirStore(dir string) *DirStore {
	return &DirStore{dir: dir}
}

func (s *DirStore) Get(key string) (string, error) {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("key not found")
		}
		return "", err
	}
	return string(data), nil
}

func (s *DirStore) Set(key, value string) error {
	p := s.path(key)
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

func (s *DirStore) Delete(key string) error {
	err := os.Remove(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *DirStore) List(prefix string, limit int) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".tmp-") {
			continue
		}
		key := unescape(e.Name())
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
			if limit > 0 && len(keys) >= limit {
				break
			}
		}
	}
	return keys, nil
}

func (s *DirStore) Close() error {
	return nil
}

func (s *DirStore) path(key string) string {
	return filepath.Join(s.dir, escape(key))
}

// escape replaces characters unsafe for filenames.
func escape(key string) string {
	r := strings.NewReplacer("/", "__", "\\", "__", ":", "_c_")
	return r.Replace(key)
}

func unescape(name string) string {
	r := strings.NewReplacer("_c_", ":", "__", "/")
	return r.Replace(name)
}
