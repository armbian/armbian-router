package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestArmbianMirror(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ArmbianMirror Suite")
}
