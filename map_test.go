package redirector

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

		Expect(err).To(BeNil())
		Expect(m["aml-s9xx-box/Bookworm_current_server"]).To(Equal("/aml-s9xx-box/archive/Armbian_23.11.1_Aml-s9xx-box_bookworm_current_6.1.63.img.xz"))
	})

	It("Should successfully load the map from a JSON file, rewriting extension paths as necessary", func() {
		data := `{
		  "assets": [
		    {
				"board_slug": "khadas-vim1",
				"file_url": "https://dl.armbian.com/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz",
				"file_updated": "2023-11-30T01:06:34Z",
				"file_size": "1605260504",
				"distro_release": "bookworm",
				"kernel_branch": "current",
				"image_variant": "xfce",
				"preinstalled_application": "",
				"promoted": "false",
				"download_repository": "archive",
				"file_extension": "img.xz"
			},
			{
				"board_slug": "khadas-vim1",
				"file_url": "https://dl.armbian.com/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz.sha",
				"file_updated": "2023-11-30T01:06:34Z",
				"file_size": "1605260504",
				"distro_release": "bookworm",
				"kernel_branch": "current",
				"image_variant": "xfce",
				"preinstalled_application": "",
				"promoted": "false",
				"download_repository": "archive",
				"file_extension": "img.xz.sha"
			},
			{
				"board_slug": "khadas-vim1",
				"file_url": "https://dl.armbian.com/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz",
				"file_updated": "2023-11-30T01:06:34Z",
				"file_size": "1605260504",
				"distro_release": "bookworm",
				"kernel_branch": "current",
				"image_variant": "xfce",
				"preinstalled_application": "test",
				"promoted": "false",
				"download_repository": "archive",
				"file_extension": "img.xz"
			}
		  ]
		}`

		m, err := loadMapJSON(strings.NewReader(data))

		Expect(err).To(BeNil())
		Expect(m["khadas-vim1/Bookworm_current_xfce"]).To(Equal("/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz"))
		Expect(m["khadas-vim1/Bookworm_current_xfce.sha"]).To(Equal("/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz.sha"))
		Expect(m["khadas-vim1/Bookworm_current_xfce-test"]).To(Equal("/khadas-vim1/archive/Armbian_23.11.1_Khadas-vim1_bookworm_current_6.1.63_xfce_desktop.img.xz"))
	})
	It("Should work with files that have weird extensions", func() {
		data := `{
  "assets": [
	{
      "board_slug": "khadas-vim4",
      "armbian_version": "23.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz",
      "file_updated": "2023-11-30T01:03:05Z",
      "file_size": "477868032",
      "distro_release": "bookworm",
      "kernel_branch": "legacy",
      "image_variant": "server",
      "preinstalled_application": "",
      "promoted": "true",
      "download_repository": "archive",
      "file_extension": "oowow.img.xz"
    },
    {
      "board_slug": "khadas-vim4",
      "armbian_version": "23.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz.asc",
      "file_updated": "2023-11-30T01:03:05Z",
      "file_size": "833",
      "distro_release": "bookworm",
      "kernel_branch": "legacy",
      "image_variant": "server",
      "preinstalled_application": "",
      "promoted": "true",
      "download_repository": "archive",
      "file_extension": "oowow.img.xz.asc"
    },
    {
      "board_slug": "khadas-vim4",
      "armbian_version": "23.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz.sha",
      "file_updated": "2023-11-30T01:03:05Z",
      "file_size": "178",
      "distro_release": "bookworm",
      "kernel_branch": "legacy",
      "image_variant": "server",
      "preinstalled_application": "",
      "promoted": "true",
      "download_repository": "archive",
      "file_extension": "oowow.img.xz.sha"
    },
	{
      "board_slug": "uefi-arm64",
      "armbian_version": "24.5.5",
      "file_url": "https://dl.armbian.com/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2",
      "redi_url": "https://dl.armbian.com/uefi-arm64/Bookworm_current_minimal",
      "file_updated": "2024-07-25T18:01:20Z",
      "file_size": "673315888",
      "distro_release": "bookworm",
      "kernel_branch": "current",
      "image_variant": "minimal",
      "preinstalled_application": "",
      "promoted": "false",
      "download_repository": "archive",
      "file_extension": "img.qcow2"
    },
    {
      "board_slug": "uefi-arm64",
      "armbian_version": "24.5.5",
      "file_url": "https://dl.armbian.com/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2.asc",
      "redi_url": "https://dl.armbian.com/uefi-arm64/Bookworm_current_minimal.asc",
      "file_updated": "2024-07-25T18:01:20Z",
      "file_size": "833",
      "distro_release": "bookworm",
      "kernel_branch": "current",
      "image_variant": "minimal",
      "preinstalled_application": "",
      "promoted": "false",
      "download_repository": "archive",
      "file_extension": "img.qcow2.asc"
    },
    {
      "board_slug": "uefi-arm64",
      "armbian_version": "24.5.5",
      "file_url": "https://dl.armbian.com/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2.sha",
      "redi_url": "https://dl.armbian.com/uefi-arm64/Bookworm_current_minimal.sha",
      "file_updated": "2024-07-25T18:01:20Z",
      "file_size": "194",
      "distro_release": "bookworm",
      "kernel_branch": "current",
      "image_variant": "minimal",
      "preinstalled_application": "",
      "promoted": "false",
      "download_repository": "archive",
      "file_extension": "img.qcow2.sha"
    },
	{
      "board_slug": "qemu-uboot-arm64",
      "armbian_version": "24.8.0-trunk.542",
      "file_url": "https://github.com/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz",
      "redi_url": "https://dl.armbian.com/qemu-uboot-arm64/Bookworm_current_minimal",
      "file_updated": "2024-08-09T10:07:43Z",
      "file_size": "314832",
      "distro_release": "bookworm",
      "kernel_branch": "current",
      "image_variant": "minimal",
      "preinstalled_application": "",
      "promoted": "false",
      "download_repository": "os",
      "file_extension": "boot.bin.xz"
    }
  ]
}`

		m, err := loadMapJSON(strings.NewReader(data))

		Expect(err).To(BeNil())
		Expect(m["khadas-vim4/Bookworm_legacy_server"]).To(Equal("/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz"))
		Expect(m["khadas-vim4/Bookworm_legacy_server.asc"]).To(Equal("/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz.asc"))
		Expect(m["khadas-vim4/Bookworm_legacy_server.sha"]).To(Equal("/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz.sha"))

		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2"]).To(Equal("/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2"))
		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2.asc"]).To(Equal("/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2.asc"))
		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2.sha"]).To(Equal("/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2.sha"))

		Expect(m["nightly/qemu-uboot-arm64/Bookworm_current_minimal-uboot-bin"]).To(Equal("/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz"))
		Expect(m["nightly/qemu-uboot-arm64/Bookworm_current_minimal-uboot-bin.boot.bin.xz"]).To(Equal("/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz"))

	})
})
