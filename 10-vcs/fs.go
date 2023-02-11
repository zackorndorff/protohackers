package main

import (
	"fmt"
	"bytes"
	"errors"
	"strings"
)

var ErrEmptyComponent = errors.New("empty component in path")
var ErrTrailingSlash = errors.New("trailing slash in file name")
var ErrNoTrailingSlash = errors.New("no trailing slash in directory name")
var ErrNotFound = errors.New("file not found")
var ErrNoRevision = errors.New("revision does not exist")
var ErrEmptyPath = errors.New("path was empty")
var ErrRelativePath = errors.New("relative path found")
var ErrIllegalFilename = errors.New("illegal filename")
var ErrInvalidText = errors.New("found non-text character")

type Revision int

type Repository struct {
	root *Directory
}

type File struct {
	revisions [][]byte
}

func (f *File) currentRevision() Revision {
	return Revision(len(f.revisions))
}

type Directory struct {
	subdirs map[string]*Directory
	files   map[string]*File
}

func NewRepository() *Repository {
	return &Repository{
		root: newDirectory(),
	}
}

func (r *Repository) lookupDirectory(components []string, create bool) (*Directory, error) {
	walk := r.root
	for _, comp := range components {
		next, ok := walk.subdirs[comp]
		if !ok && !create {
			return nil, ErrNotFound
		} else if !ok {
			next = newDirectory()
			walk.subdirs[comp] = next
		}
		walk = next
	}
	return walk, nil
}

func (r *Repository) lookupFile(components []string, create bool) (*File, error) {
	if len(components) < 1 {
		return nil, ErrEmptyPath
	}

	dir, err := r.lookupDirectory(components[:len(components)-1], create)
	if err != nil {
		return nil, err
	}

	last := components[len(components)-1]
	file, ok := dir.files[last]
	if !ok && !create {
		return nil, ErrNotFound
	} else if !ok {
		file = &File{}
		dir.files[last] = file
	}
	return file, nil
}

func newDirectory() *Directory {
	return &Directory{
		subdirs: make(map[string]*Directory),
		files:   make(map[string]*File),
	}
}

func parsePath(path string) ([]string, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}

	components := strings.Split(path, "/")
	if components[0] != "" {
		return nil, ErrRelativePath
	}
	if len(components) < 2 {
		return nil, ErrRelativePath
	}

	components = components[1:]
	// handles "/" case
	if components[0] == "" {
		return nil, nil
	}

	for _, comp := range components {
		if len(comp) == 0 {
			return nil, ErrEmptyComponent
		}
		if filenameIsIllegal(comp) {
			return nil, ErrIllegalFilename
		}
	}
	return components, nil
}

func parseDirectory(path string) ([]string, error) {
	//if !strings.HasSuffix(path, "/") {
	//	return nil, ErrNoTrailingSlash
	//}
	return parsePath(path)
}

func parseFilename(path string) ([]string, error) {
	if strings.HasSuffix(path, "/") {
		return nil, ErrTrailingSlash
	}
	return parsePath(path)
}

func filenameIsIllegal(filename string) bool {
	return strings.ContainsAny(filename, "[]!@#$%^&*()|{}[]`+?\"'=~")
}

func textIsValid(data []byte) bool {
	for _, b := range data {
		if b < 0x9 || b > 0xd && b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}

type ListEntry struct {
	Name      string
	IsOnlyDir bool
	Revision  Revision
}

func (r *Repository) List(path string) ([]*ListEntry, error) {
	parsed, err := parseDirectory(path)
	if err != nil {
		return nil, err
	}

	dir, err := r.lookupDirectory(parsed, false)
	if err != nil {
		return nil, err
	}

	combined := make(map[string]*ListEntry)
	for name, _ := range dir.subdirs {
		combined[name] = &ListEntry{
			Name:      name,
			IsOnlyDir: true,
			Revision:  0,
		}
	}

	for name, file := range dir.files {
		if entry, ok := combined[name]; ok {
			entry.IsOnlyDir = false
			entry.Revision = file.currentRevision()
		} else {
			combined[name] = &ListEntry{
				Name:      name,
				IsOnlyDir: false,
				Revision:  file.currentRevision(),
			}
		}
	}

	entries := make([]*ListEntry, len(combined))
	count := 0
	for _, entry := range combined {
		entries[count] = entry
		count += 1
	}
	return entries, nil
}

func (r *Repository) Get(path string, hasRevision bool, revision Revision) ([]byte, error) {
	parsed, err := parseFilename(path)
	if err != nil {
		return nil, err
	}

	file, err := r.lookupFile(parsed, false)
	if err != nil {
		return nil, err
	}

	if !hasRevision {
		return file.revisions[len(file.revisions)-1], nil
	}

	if file.currentRevision() < revision || revision < 1 {
		return nil, ErrNoRevision
	}

	return file.revisions[revision-1], nil
}

func (r *Repository) Put(path string, data []byte) (Revision, error) {
	fmt.Println("data", string(data))
	parsed, err := parseFilename(path)
	if err != nil {
		return 0, err
	}

	file, err := r.lookupFile(parsed, true)
	if err != nil {
		return 0, err
	}

	if !textIsValid(data) {
		return 0, ErrInvalidText
	}

	if len(file.revisions) == 0 || !bytes.Equal(data, file.revisions[len(file.revisions)-1]) {
		file.revisions = append(file.revisions, data)
	}
	return file.currentRevision(), nil
}
