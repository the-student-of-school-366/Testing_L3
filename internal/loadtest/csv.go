package loadtest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Row is one parameterized virtual-user iteration.
// Columns match loadtest/data/posts.csv.
type Row struct {
	Username     string
	Password     string
	Title        string
	Body         string
	UserID       int
	PatchedTitle string
}

// LoadRows reads the CSV data set used to parameterize requests.
func LoadRows(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()
	return ParseRows(f)
}

// ParseRows parses CSV with a header row.
func ParseRows(r io.Reader) ([]Row, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, errors.New("csv: need a header and at least one data row")
	}

	header := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		header[strings.TrimSpace(name)] = i
	}
	for _, name := range []string{"username", "password", "title", "body", "user_id", "patched_title"} {
		if _, ok := header[name]; !ok {
			return nil, fmt.Errorf("csv: missing column %q", name)
		}
	}

	rows := make([]Row, 0, len(records)-1)
	for i, rec := range records[1:] {
		if strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		line := i + 2
		userID, err := strconv.Atoi(strings.TrimSpace(field(rec, header, "user_id")))
		if err != nil {
			return nil, fmt.Errorf("csv: row %d user_id: %w", line, err)
		}
		row := Row{
			Username:     strings.TrimSpace(field(rec, header, "username")),
			Password:     field(rec, header, "password"),
			Title:        strings.TrimSpace(field(rec, header, "title")),
			Body:         field(rec, header, "body"),
			UserID:       userID,
			PatchedTitle: strings.TrimSpace(field(rec, header, "patched_title")),
		}
		if row.Username == "" || row.Password == "" || row.Title == "" || row.PatchedTitle == "" {
			return nil, fmt.Errorf("csv: row %d has an empty required field", line)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, errors.New("csv: no data rows")
	}
	return rows, nil
}

func field(rec []string, header map[string]int, name string) string {
	idx := header[name]
	if idx >= len(rec) {
		return ""
	}
	return rec[idx]
}
