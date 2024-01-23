package redirector

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"io"
	"os"
	"path"
	"strings"
)

var ErrUnsupportedFormat = errors.New("unsupported map format")

// loadMapFile loads a file as a map
func loadMapFile(file string) (map[string]string, error) {
	f, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	ext := path.Ext(file)

	switch ext {
	case ".csv":
		return loadMapCSV(f)
	case ".json":
		return loadMapJSON(f)
	}

	return nil, ErrUnsupportedFormat
}

// loadMapCSV loads a pipe separated file of mappings
func loadMapCSV(f io.Reader) (map[string]string, error) {
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

// ReleaseFile represents a file to be mapped
type ReleaseFile struct {
	BoardSlug     string `json:"board_slug"`
	FileURI       string `json:"file_uri"`
	FileUpdated   string `json:"file_updated"`
	FileSize      string `json:"file_size"`
	DistroRelease string `json:"distro_release"`
	KernelBranch  string `json:"kernel_branch"`
	ImageVariant  string `json:"image_variant"`
	Preinstalled  string `json:"preinstalled_application"`
	Promoted      string `json:"promoted"`
	Repository    string `json:"download_repository"`
	Extension     string `json:"file_extension"`
}

var distroCaser = cases.Title(language.Und)

// loadMapJSON loads a map file from JSON, based on the format specified in the github issue.
// See: https://github.com/armbian/os/pull/129
func loadMapJSON(f io.Reader) (map[string]string, error) {
	m := make(map[string]string)

	var files []ReleaseFile

	if err := json.NewDecoder(f).Decode(&files); err != nil {
		return nil, err
	}

	for _, file := range files {
		builtUri := fmt.Sprintf("%s/%s-%s.%s",
			file.BoardSlug,
			distroCaser.String(file.DistroRelease),
			file.KernelBranch,
			file.Extension)

		m[builtUri] = file.FileURI
	}

	return m, nil
}
