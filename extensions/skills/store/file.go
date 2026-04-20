package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vitaliiPsl/crappy-adk/extensions/skills"
	"github.com/vitaliiPsl/crappy-adk/utils/frontmatter"
)

const (
	skillFile     = "skill.md"
	referencesDir = "references"
)

// Frontmatter holds the YAML metadata parsed from the top of a skill.md file.
type Frontmatter struct {
	// Name of the skill, used as the key in the catalog.
	Name string `yaml:"name"`
	// Short description of the skill shown in the catalog.
	Description string `yaml:"description"`
}

// FileStore is a [Store] backed by a directory of skill folders on disk.
// Each subdirectory must contain a skill.md file with YAML frontmatter.
type FileStore struct {
	dir string

	once     sync.Once
	indexErr error
	index    map[string]string
}

// NewFileStore creates a [FileStore] rooted at the given directory.
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (fs *FileStore) List(_ context.Context) ([]skills.Skill, error) {
	if err := fs.prepareIndex(); err != nil {
		return nil, err
	}

	skills := make([]skills.Skill, 0, len(fs.index))
	for _, dir := range fs.index {
		s, err := fs.loadSkill(dir)
		if err != nil {
			continue
		}

		skills = append(skills, *s)
	}

	return skills, nil
}

func (fs *FileStore) Get(_ context.Context, name string) (*skills.Skill, error) {
	if err := fs.prepareIndex(); err != nil {
		return nil, err
	}

	dir, ok := fs.index[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	return fs.loadSkill(dir)
}

func (fs *FileStore) GetReference(_ context.Context, skill string, reference string) (string, error) {
	if err := fs.prepareIndex(); err != nil {
		return "", err
	}

	dir, ok := fs.index[skill]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skill)
	}

	data, err := os.ReadFile(filepath.Join(dir, referencesDir, reference))
	if err != nil {
		return "", fmt.Errorf("reference not found: %s/%s", skill, reference)
	}

	return string(data), nil
}

func (fs *FileStore) prepareIndex() error {
	fs.once.Do(func() {
		entries, err := os.ReadDir(fs.dir)
		if err != nil {
			fs.indexErr = fmt.Errorf("read skills dir: %w", err)

			return
		}

		idx := make(map[string]string, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(fs.dir, entry.Name(), skillFile)

			data, err := os.ReadFile(skillPath)
			if err != nil {
				continue
			}

			fm, _, err := frontmatter.Unmarshal[Frontmatter](string(data))
			if err != nil || fm.Name == "" {
				continue
			}

			idx[fm.Name] = filepath.Join(fs.dir, entry.Name())
		}

		fs.index = idx
	})

	return fs.indexErr
}

func (fs *FileStore) loadSkill(dir string) (*skills.Skill, error) {
	data, err := os.ReadFile(filepath.Join(dir, skillFile))
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	fm, content, err := frontmatter.Unmarshal[Frontmatter](string(data))
	if err != nil {
		return nil, err
	}

	var refs []string
	if refEntries, err := os.ReadDir(filepath.Join(dir, referencesDir)); err == nil {
		for _, e := range refEntries {
			if !e.IsDir() {
				refs = append(refs, e.Name())
			}
		}
	}

	return &skills.Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Content:     content,
		References:  refs,
	}, nil
}
