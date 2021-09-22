package vdir

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/fatih/color"
)

const (
	StatusCompleted   = "COMPLETED"
	StatusNeedsAction = "NEEDS-ACTION"
	PriorityHigh      = 1
	PriorityMedium    = 5
	PriorityLow       = 6
)

// Item represents an iCalendar item with a unique id
type Item struct {
	Id   int
	Path string
	Ical *ical.Calendar
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

// Format returns a string representation of an item
func (i *Item) Format() (string, error) {
	sb := strings.Builder{}

	colorStatusDone := color.New(color.FgGreen).SprintFunc()
	colorStatusUndone := color.New(color.FgBlue).SprintFunc()
	colorPrioHigh := color.New(color.FgHiRed, color.Bold).SprintFunc()
	colorPrioMedium := color.New(color.FgHiYellow, color.Bold).SprintFunc()
	colorDesc := color.New(color.Faint).SprintFunc()
	// colorDate := color.New(color.FgYellow).SprintFunc()

	var vtodo *ical.Component

	for _, comp := range i.Ical.Children {
		if comp.Name == ical.CompToDo {
			vtodo = comp
		}
	}

	if vtodo.Name != ical.CompToDo {
		return "", fmt.Errorf("Not VTODO component: %v", vtodo)
	}

	var (
		status      string
		summary     string
		description string
		prio        string
		// due         string
	)

	for name, prop := range vtodo.Props {
		p := prop[0]

		switch name {

		case ical.PropStatus:
			if p.Value == StatusCompleted {
				status = colorStatusDone("[x]")
			} else {
				status = colorStatusUndone("[ ]")
			}

		case ical.PropSummary:
			summary = p.Value

		case ical.PropDescription:
			description = colorDesc(fmt.Sprintf("(%s)", p.Value))

		case ical.PropPriority:
			v, err := strconv.Atoi(p.Value)
			if err != nil {
				return "", err
			}
			switch {
			case v == PriorityHigh:
				prio = colorPrioHigh("!!!")
			case v > PriorityHigh && v <= PriorityMedium:
				prio = colorPrioMedium("!!")
			case v > PriorityMedium:
				prio = colorPrioMedium("!")
			}

		}
	}

	sb.WriteString(fmt.Sprintf("%2d", i.Id))
	sb.WriteString(fmt.Sprintf(" %s", status))
	if prio != "" {
		sb.WriteString(fmt.Sprintf(" %s", prio))
	}
	sb.WriteString(fmt.Sprintf(" %s", summary))
	if description != "" {
		sb.WriteString(fmt.Sprintf(" %s", description))
	}

	return sb.String(), nil
}

// WriteFile encodes ical data and writes to file
func (i *Item) WriteFile() error {
	var buf bytes.Buffer
	err := ical.NewEncoder(&buf).Encode(i.Ical)
	if err != nil {
		return err
	}

	// TODO: update modified date prop

	f, err := os.Create(i.Path)
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

// GenerateUID returns a random string containing timestamp and hostname
func GenerateUID() string {
	sb := strings.Builder{}

	time := time.Now().UnixNano()

	randStr := func(n int) string {
		rs := rand.NewSource(time)
		r := rand.New(rs)
		var letters = []rune("1234567890abcdefghijklmnopqrstuvwxyz")

		s := make([]rune, n)
		for i := range s {
			s[i] = letters[r.Intn(len(letters))]
		}
		return string(s)
	}

	sb.WriteString(fmt.Sprint(time))
	sb.WriteString(fmt.Sprintf("-%s", randStr(8)))
	if hostname, _ := os.Hostname(); hostname != "" {
		sb.WriteString(fmt.Sprintf("@%s", hostname))
	}

	return sb.String()
}
