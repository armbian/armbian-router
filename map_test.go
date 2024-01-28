package redirector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"strings"
)

var _ = Describe("Map", func() {
	It("Should successfully load the map from a CSV/Pipe separated file", func() {
		m, err := loadMapCSV(strings.NewReader(`bananapi/Bullseye_current|bananapi/archive/Armbian_21.08.1_Bananapi_bullseye_current_5.10.60.img.xz|Aug 26 2021|332M`))

		Expect(err).To(BeNil())
		Expect(m["bananapi/Bullseye_current"]).To(Equal("bananapi/archive/Armbian_21.08.1_Bananapi_bullseye_current_5.10.60.img.xz"))
	})
	It("Should successfully load the map from a JSON file", func() {
		data := `{
  "assets": [
    {
      "board_slug": "aml-s9xx-box",
      "armbian_version": "23.11.1",
      "file_url": "https://dl.armbian.com/aml-s9xx-box/archive/Armbian_23.11.1_Aml-s9xx-box_bookworm_current_6.1.63.img.xz",
      "file_updated": "2023-11-30T01:14:49Z",
      "file_size": "566235552",
      "distro_release": "bookworm",
      "kernel_branch": "current",
      "image_variant": "server",
      "preinstalled_application": "",
      "promoted": "true",
      "download_repository": "archive",
      "file_extension": "img.xz"
    }
  ]
}`

		m, err := loadMapJSON(strings.NewReader(data))

		log.Println(m)
		Expect(err).To(BeNil())
		Expect(m["aml-s9xx-box/Bookworm_current"]).To(Equal("https://dl.armbian.com/aml-s9xx-box/archive/Armbian_23.11.1_Aml-s9xx-box_bookworm_current_6.1.63.img.xz"))
	})
})
