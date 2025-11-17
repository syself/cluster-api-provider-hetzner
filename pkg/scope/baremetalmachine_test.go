package scope

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test splitHostKey", func() {
	namespace, name := splitHostKey("namespace/name")
	Expect(namespace).To(Equal("namespace"))
	Expect(name).To(Equal("name"))
})
