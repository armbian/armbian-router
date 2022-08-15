package redirector

import (
	"encoding/csv"
	"io"
	"os"
	"strings"
)

// loadMapFile loads a file as a map
func loadMapFile(file string) (map[string]string, error) {
	f, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	return loadMap(f)
}

// loadMap loads a pipe separated file of mappings
func loadMap(f io.Reader) (map[string]string, error) {
	m := make(map[string]string)

	r := csv.NewReader(f)

	r.Comma = '|'

	for {
		row, err := r.Read()

		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		m[strings.TrimLeft(row[0], "/")] = strings.TrimLeft(row[1], "/")
	}

	return m, nil
}
