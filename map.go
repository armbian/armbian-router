package redirector

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"golang.org/x/exp/maps"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ErrUnsupportedFormat is returned when an unsupported map format is used.
var ErrUnsupportedFormat = errors.New("unsupported map format")

// loadMapFile loads a file as a map
func loadMapFile(file string, specialExtensions map[string]string) (map[string]string, error) {
	f, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	ext := path.Ext(file)

	switch ext {
	case ".json":
		return loadMapJSON(f, specialExtensions)
	}

	return nil, ErrUnsupportedFormat
}

// Map represents a JSON format of an asset list
type Map struct {
	Assets []ReleaseFile `json:"assets"`
}

// ReleaseFile represents a file to be mapped
type ReleaseFile struct {
	BoardSlug     string `json:"board_slug"`
	FileURL       string `json:"file_url"`
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
func loadMapJSON(f io.Reader, specialExtensions map[string]string) (map[string]string, error) {
	// Avoid panics
	if specialExtensions == nil {
		specialExtensions = make(map[string]string)
	}

	m := make(map[string]string)

	var data Map

	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}

	for _, file := range data.Assets {
		// Because download mapping a full URL, redirecting, and finding a server again is redundant,
		// we parse the URL and only return the path here. Previously, it would use https://dl.armbian.com/PATH
		// which is not supported, as the redirector will always prepend a server
		u, err := url.Parse(file.FileURL)

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"uri":   file.FileURL,
			}).Warning("Error parsing redirect url or path")
			continue
		}

		var sb strings.Builder

		if file.Repository == "os" {
			sb.WriteString("nightly/")
		}

		sb.WriteString(file.BoardSlug)
		sb.WriteString("/")
		sb.WriteString(distroCaser.String(file.DistroRelease))
		sb.WriteString("_")
		sb.WriteString(file.KernelBranch)
		sb.WriteString("_")
		sb.WriteString(file.ImageVariant)

		if file.Preinstalled != "" {
			sb.WriteString("-")
			sb.WriteString(file.Preinstalled)
		}

		// Check special case for some extensions
		for k, v := range specialExtensions {
			if strings.Contains(file.Extension, k) {
				sb.WriteString(v)
			}
		}

		imageExtensions := maps.Keys(specialExtensions)
		imageExtensions = append(imageExtensions, "img.xz") // extra allocation, but it's fine

		// Add board into the map without an extension
		for _, ext := range imageExtensions {
			if strings.HasSuffix(file.Extension, ext) {
				m[sb.String()] = u.Path
				break
			}
		}

		sb.WriteString(".")

		if strings.HasSuffix(file.Extension, ".sha") {
			sb.WriteString("sha")
		} else if strings.HasSuffix(file.Extension, ".asc") {
			sb.WriteString("asc")
		} else if strings.HasSuffix(file.Extension, ".torrent") {
			sb.WriteString("torrent")
		} else {
			sb.WriteString(file.Extension)
		}

		m[sb.String()] = u.Path // Add board into the map with an extension
	}

	return m, nil
}
