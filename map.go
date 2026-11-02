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
var extensionFormats = []string{".asc", ".sha", ".torrent"}

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
	BoardSlug      string `json:"board_slug"`
	FileURL        string `json:"file_url"`
	FileURLASC     string `json:"file_url_asc"`
	FileURLSHA     string `json:"file_url_sha"`
	FileURLTorrent string `json:"file_url_torrent"`
	FileUpdated    string `json:"file_updated"`
	FileSize       string `json:"file_size"`
	DistroRelease  string `json:"distro"`
	KernelBranch   string `json:"branch"`
	ImageVariant   string `json:"variant"`
	Preinstalled   string `json:"file_application"`
	Promoted       string `json:"promoted"`
	Repository     string `json:"download_repository"`
	Extension      string `json:"file_extension"`
	RediURL        string `json:"redi_url"`
}

var distroCaser = cases.Title(language.Und)

// rediHost is the only redi_url host whose path is used as a map key
const rediHost = "dl.armbian.com"

// loadMapJSON loads a map file from JSON, based on the format specified in the github issue.
// See: https://github.com/armbian/os/pull/129
func loadMapJSON(f io.Reader, specialExtensions map[string]string) (map[string]string, error) {
	// Avoid panics
	if specialExtensions == nil {
		specialExtensions = make(map[string]string)
	}

	m := make(map[string]string)
	legacy := make(map[string]string)

	var data Map

	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}

	imageExtensions := maps.Keys(specialExtensions)
	imageExtensions = append(imageExtensions, "img.xz", "tar.xz") // extra allocation, but it's fine

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

		isImage := false

		for _, ext := range imageExtensions {
			if strings.HasSuffix(file.Extension, ext) {
				isImage = true
				break
			}
		}

		legacyKey := legacyMapKey(file, specialExtensions)

		rediKey, ok := rediMapKey(file.RediURL)

		if !ok || rediKey == legacyKey {
			addMapEntries(m, legacyKey, file, u.Path, isImage)
			continue
		}

		// A redi_url of its own marks a dedicated download, so it always gets the bare key
		addMapEntries(m, rediKey, file, u.Path, true)
		addMapEntries(legacy, legacyKey, file, u.Path, isImage)
	}

	// Keep the old keys working where no redi_url claims them
	for k, v := range legacy {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}

	return m, nil
}

// legacyMapKey builds the key from the asset fields and the special extension suffixes
func legacyMapKey(file ReleaseFile, specialExtensions map[string]string) string {
	var sb strings.Builder

	if file.Repository == "ci" {
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
			break
		}
	}

	return sb.String()
}

// rediMapKey returns the path of a dl.armbian.com redi_url without the leading slash
func rediMapKey(rediURL string) (string, bool) {
	if rediURL == "" {
		return "", false
	}

	u, err := url.Parse(rediURL)

	if err != nil || u.Host != rediHost {
		return "", false
	}

	key := strings.TrimPrefix(u.Path, "/")

	return key, key != ""
}

// addMapEntries maps the key with and without the extension, plus the .asc .sha and .torrent keys
func addMapEntries(m map[string]string, key string, file ReleaseFile, filePath string, bare bool) {
	// Add board into the map without an extension
	if bare {
		m[key] = filePath
	}

	// Also add entries for .asc .sha and .torrent
	for _, ext := range extensionFormats {
		if strings.HasSuffix(file.Extension, ext) {
			continue
		}

		var sigPath string
		switch ext {
		case ".asc":
			sigPath = file.FileURLASC
		case ".sha":
			sigPath = file.FileURLSHA
		case ".torrent":
			sigPath = file.FileURLTorrent
		}

		m[key+ext] = sigPath
	}

	m[key+"."+file.Extension] = filePath // Add board into the map with an extension
}
