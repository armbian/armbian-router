package redirector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("Map", func() {
	It("Should successfully load the map", func() {
		m, err := loadMap(strings.NewReader(`bananapi/Bullseye_current|bananapi/archive/Armbian_21.08.1_Bananapi_bullseye_current_5.10.60.img.xz|Aug 26 2021|332M`))

		Expect(err).To(BeNil())
		Expect(m["bananapi/Bullseye_current"]).To(Equal("bananapi/archive/Armbian_21.08.1_Bananapi_bullseye_current_5.10.60.img.xz"))
	})
})
