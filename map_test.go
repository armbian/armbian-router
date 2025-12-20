package redirector

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testExtensions = map[string]string{
	"boot-sms.img.xz":  "-boot-sms",
	"boot-boe.img.xz":  "-boot-boe",
	"boot-csot.img.xz": "-boot-csot",
	"rootfs.img.xz":    "-rootfs",
	"img.qcow2":        "-qcow2",
	"img.qcow2.xz":     "-qcow2",
	"boot.bin.xz":      "-uboot-bin",
}

var _ = Describe("Map", func() {
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

		m, err := loadMapJSON(strings.NewReader(data), testExtensions)

		Expect(err).To(BeNil())
		Expect(m["aml-s9xx-box/Bookworm_current_server"]).To(Equal("/aml-s9xx-box/archive/Armbian_23.11.1_Aml-s9xx-box_bookworm_current_6.1.63.img.xz"))
	})

	It("Should successfully load the map from a JSON file, rewriting extension paths as necessary", func() {
		data := `{
		  "assets": [
		    {
      "board_slug": "khadas-vim1",
      "board_name": "Khadas VIM1",
      "board_vendor": "khadas",
      "armbian_version": "25.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz",
      "file_url_asc": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.asc",
      "file_url_sha": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.sha",
      "file_url_torrent": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.torrent",
      "redi_url": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce",
      "redi_url_asc": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.asc",
      "redi_url_sha": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.sha",
      "redi_url_torrent": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.torrent",
      "file_updated": "2025-11-22T15:39:59Z",
      "file_size": "1482867344",
      "distro_release": "noble",
      "kernel_branch": "current",
      "image_variant": "xfce",
      "preinstalled_application": "",
      "promoted": "false",
      "download_repository": "archive",
      "file_extension": "img.xz"
    },
    {
      "board_slug": "khadas-vim1",
      "board_name": "Khadas VIM1",
      "board_vendor": "khadas",
      "armbian_version": "25.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim1/archive2/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz",
      "file_url_asc": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.asc",
      "file_url_sha": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.sha",
      "file_url_torrent": "https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.torrent",
      "redi_url": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce",
      "redi_url_asc": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.asc",
      "redi_url_sha": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.sha",
      "redi_url_torrent": "https://dl.armbian.com/khadas-vim1/Noble_current_xfce.torrent",
      "file_updated": "2025-11-22T15:39:59Z",
      "file_size": "1482867344",
      "distro_release": "noble",
      "kernel_branch": "current",
      "image_variant": "xfce",
      "preinstalled_application": "test",
      "promoted": "false",
      "download_repository": "archive",
      "file_extension": "img.xz"
    }
		  ]
		}`

		m, err := loadMapJSON(strings.NewReader(data), testExtensions)

		Expect(err).To(BeNil())
		Expect(m["khadas-vim1/Noble_current_xfce"]).To(Equal("/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz"))
		Expect(m["khadas-vim1/Noble_current_xfce.sha"]).To(Equal("https://dl.armbian.com/khadas-vim1/archive/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz.sha"))
		Expect(m["khadas-vim1/Noble_current_xfce-test"]).To(Equal("/khadas-vim1/archive2/Armbian_25.11.1_Khadas-vim1_noble_current_6.12.58_xfce_desktop.img.xz"))
	})
	It("Should work with files that have weird extensions", func() {
		data := `{
  "assets": [
	{
      "board_slug": "khadas-vim4",
      "armbian_version": "23.11.1",
      "file_url": "https://dl.armbian.com/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz",
      "file_url_sha": "sha_test_url_vim4",
      "file_url_asc": "asc_test_url_vim4",
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
      "board_slug": "uefi-arm64",
      "armbian_version": "24.5.5",
      "file_url": "https://dl.armbian.com/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2",
      "file_url_sha": "sha_test_url_uefi",
      "file_url_asc": "asc_test_url_uefi",
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
      "board_slug": "qemu-uboot-arm64",
      "armbian_version": "24.8.0-trunk.542",
      "file_url": "https://github.com/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz",
      "file_url_sha": "sha_test_url_qemu",
      "file_url_asc": "asc_test_url_qemu",
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

		m, err := loadMapJSON(strings.NewReader(data), testExtensions)

		Expect(err).To(BeNil())
		Expect(m["khadas-vim4/Bookworm_legacy_server"]).To(Equal("/khadas-vim4/archive/Armbian_23.11.1_Khadas-vim4_bookworm_legacy_5.4.180.oowow.img.xz"))
		Expect(m["khadas-vim4/Bookworm_legacy_server.asc"]).To(Equal("asc_test_url_vim4"))
		Expect(m["khadas-vim4/Bookworm_legacy_server.sha"]).To(Equal("sha_test_url_vim4"))

		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2"]).To(Equal("/uefi-arm64/archive/Armbian_24.5.5_Uefi-arm64_bookworm_current_6.6.42_minimal.img.qcow2"))
		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2.asc"]).To(Equal("asc_test_url_uefi"))
		Expect(m["uefi-arm64/Bookworm_current_minimal-qcow2.sha"]).To(Equal("sha_test_url_uefi"))
		Expect(m["nightly/qemu-uboot-arm64/Bookworm_current_minimal-uboot-bin"]).To(Equal("/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz"))
		Expect(m["nightly/qemu-uboot-arm64/Bookworm_current_minimal-uboot-bin.boot.bin.xz"]).To(Equal("/armbian/os/releases/download/24.8.0-trunk.542/Armbian_24.8.0-trunk.542_Qemu-uboot-arm64_bookworm_current_6.6.44_minimal.u-boot.bin.xz"))

	})
})
