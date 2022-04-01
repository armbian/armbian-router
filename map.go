package main

import (
	"encoding/csv"
	"io"
	"os"
)

func loadMap(file string) (map[string]string, error) {
	f, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer f.Close()

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

		m[row[0]] = row[1]
	}

	return m, nil
}
