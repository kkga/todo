package vdir

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-ical"
)

// VdirRoot represents the topmost vdir root folder
type VdirRoot struct {
	Path string
}

// Collection represents a Vdir collection
type Collection struct {
	Name  string
	Color string
	Path  string
}

// Item represents an iCalendar item with a unique id
type Item struct {
	Id   int
	Path string
	Ical *ical.Calendar
}

const (
	MetaDisplayName = "displayname" // MetaDisplayName is a filename vdir uses for collection name
	MetaColor       = "color"       // MetaColor is a filename vdir uses for collection color
)

// NewVdirRoot initializes a VdirRoot checking that the directory exists
func NewVdirRoot(path string) (*VdirRoot, error) {
	f, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("Specified vdir path does not exist.")
	}
	if !f.IsDir() {
		return nil, errors.New("Specified vdir path is not a directory.")
	}
	return &VdirRoot{path}, nil
}

// NewItem initializes an Item with a decoded ical from path
func NewItem(path string) (*Item, error) {
	i := &Item{}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	i.Path = path

	dec := ical.NewDecoder(file)

	for {
		item, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// filter only items that contain vtodo
		for _, comp := range item.Children {
			if comp.Name == ical.CompToDo {
				i.Ical = item
				return i, nil
			}
		}
	}
	return nil, err
}

// NewCollection initializes a Collection with a path, name and color parsed from path
func NewCollection(path string, name string) (*Collection, error) {
	c := &Collection{
		Path: path,
		Name: name,
	}

	// parse dir for metadata
	err := filepath.WalkDir(
		path,
		func(pp string, dd fs.DirEntry, err error) error {
			if dd.Name() == MetaDisplayName {
				name, err := os.ReadFile(pp)
				if err != nil {
					return err
				}
				c.Name = string(name)
			}
			if dd.Name() == MetaColor {
				color, err := os.ReadFile(pp)
				if err != nil {
					return err
				}
				c.Color = string(color)
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c Collection) String() string {
	return c.Name
}

// Collections returns a map of all collections and items in vdir, items have unique id values
func (v VdirRoot) Collections() (items map[*Collection][]*Item, err error) {
	items = make(map[*Collection][]*Item)
	id := 0

	isIcal := func(path string, de fs.DirEntry) bool {
		return !de.IsDir() && strings.TrimPrefix(filepath.Ext(path), ".") == ical.Extension
	}

	// parse dir for vdir collections (folders)
	err = filepath.WalkDir(
		v.Path,
		func(p string, d fs.DirEntry, err error) error {
			if d.IsDir() && p != v.Path {
				var c, err = NewCollection(p, d.Name())
				if err != nil {
					return err
				}
				// parse collection folder for ical files
				err = filepath.WalkDir(c.Path, func(pp string, dd fs.DirEntry, err error) error {
					if isIcal(pp, dd) {
						item, err := NewItem(pp)
						if err != nil {
							return err
						}
						if item != nil {
							id++
							item.Id = id
							items[c] = append(items[c], item)
						}
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return
	}
	return items, nil
}

// WriteItem encodes given ical.Calendar data and writes to file in collection dir
func (v VdirRoot) WriteItem(collection Collection, item *Item) error {
	var buf bytes.Buffer
	err := ical.NewEncoder(&buf).Encode(item.Ical)
	if err != nil {
		return err
	}

	// TODO: update modified date prop

	f, err := os.Create(item.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.Write(buf.Bytes())
	if err != nil {
		return err
	}

	w.Flush()
	if err != nil {
		return err
	}

	return nil
}
