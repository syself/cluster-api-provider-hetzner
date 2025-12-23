/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

var _ = Describe("Test Image.GetDetails", func() {
	type testCaseImageGetDetails struct {
		name                string
		url                 string
		path                string
		expectImagePath     string
		expectNeedsDownload bool
		expectError         bool
	}

	DescribeTable("Test Image.GetDetails",
		func(tc testCaseImageGetDetails) {
			image := Image{
				Name: tc.name,
				URL:  tc.url,
				Path: tc.path,
			}

			imagePath, needsDownload, errMessage := image.GetDetails()
			Expect(imagePath).Should(Equal(tc.expectImagePath))
			Expect(needsDownload).Should(Equal(tc.expectNeedsDownload))
			Expect(errMessage != "").Should(Equal(tc.expectError))
		},
		Entry("image name, url and path set", testCaseImageGetDetails{
			name:                "image_name",
			url:                 "http://test-url.com/image.tar",
			path:                "/path/to/image",
			expectImagePath:     "/root/image_name.tar",
			expectNeedsDownload: true,
			expectError:         false,
		}),
		Entry("image name and url set", testCaseImageGetDetails{
			name:                "image_name",
			url:                 "http://test-url.com/image.tar",
			path:                "",
			expectImagePath:     "/root/image_name.tar",
			expectNeedsDownload: true,
			expectError:         false,
		}),
		Entry("image name and url set - other image type", testCaseImageGetDetails{
			name:                "image_name",
			url:                 "http://test-url.com/image.tgz",
			path:                "",
			expectImagePath:     "/root/image_name.tgz",
			expectNeedsDownload: true,
			expectError:         false,
		}),
		Entry("path set", testCaseImageGetDetails{
			name:                "",
			url:                 "",
			path:                "/path/to/image",
			expectImagePath:     "/path/to/image",
			expectNeedsDownload: false,
			expectError:         false,
		}),
		Entry("image name and path set", testCaseImageGetDetails{
			name:                "image_name",
			url:                 "",
			path:                "/path/to/image",
			expectImagePath:     "/path/to/image",
			expectNeedsDownload: false,
			expectError:         false,
		}),
		Entry("url and path set", testCaseImageGetDetails{
			name:                "",
			url:                 "http://test-url.com/image.tar",
			path:                "/path/to/image",
			expectImagePath:     "/path/to/image",
			expectNeedsDownload: false,
			expectError:         false,
		}),
		Entry("image name set", testCaseImageGetDetails{
			name:                "image_name",
			url:                 "",
			path:                "",
			expectImagePath:     "",
			expectNeedsDownload: false,
			expectError:         true,
		}),
		Entry("url set", testCaseImageGetDetails{
			name:                "",
			url:                 "http://test-url.com/image.tar",
			path:                "",
			expectImagePath:     "",
			expectNeedsDownload: false,
			expectError:         true,
		}),
	)
})

var _ = Describe("Test GetImageSuffix", func() {
	type testCaseGetImageSuffix struct {
		url          string
		expectSuffix string
		expectError  bool
	}

	DescribeTable("Test GetImageSuffix",
		func(tc testCaseGetImageSuffix) {
			suffix, err := GetImageSuffix(tc.url)
			Expect(errors.Is(err, errUnknownSuffix)).Should(Equal(tc.expectError))
			Expect(suffix).Should(Equal(tc.expectSuffix))
		},
		Entry("tar", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tar",
			expectSuffix: "tar",
			expectError:  false,
		}),
		Entry("tar.gz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tar.gz",
			expectSuffix: "tar.gz",
			expectError:  false,
		}),
		Entry("tar.bz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tar.bz",
			expectSuffix: "tar.bz",
			expectError:  false,
		}),
		Entry("tar.bz2", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tar.bz2",
			expectSuffix: "tar.bz2",
			expectError:  false,
		}),
		Entry("tar.xz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tar.xz",
			expectSuffix: "tar.xz",
			expectError:  false,
		}),
		Entry("tgz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tgz",
			expectSuffix: "tgz",
			expectError:  false,
		}),
		Entry("tbz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.tbz",
			expectSuffix: "tbz",
			expectError:  false,
		}),
		Entry("txz", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.txz",
			expectSuffix: "txz",
			expectError:  false,
		}),
		Entry("unknown ending", testCaseGetImageSuffix{
			url:          "http://test-url.com/image.other",
			expectSuffix: "",
			expectError:  true,
		}),
	)
})

var _ = Describe("Test HasHostAnnotation", func() {
	type testCaseHasHostAnnotation struct {
		annotations map[string]string
		expectBool  bool
	}

	DescribeTable("Test HasHostAnnotation",
		func(tc testCaseHasHostAnnotation) {
			bmMachine := HetznerBareMetalMachine{}
			bmMachine.SetAnnotations(tc.annotations)

			Expect(bmMachine.HasHostAnnotation()).Should(Equal(tc.expectBool))
		},
		Entry("has reboot annotation - one annotation in list", testCaseHasHostAnnotation{
			annotations: map[string]string{HostAnnotation: "reboot"},
			expectBool:  true,
		}),
		Entry("has reboot annotation - multiple annotations in list", testCaseHasHostAnnotation{
			annotations: map[string]string{"other": "annotation", HostAnnotation: "reboot"},
			expectBool:  true,
		}),
		Entry("has no reboot annotation", testCaseHasHostAnnotation{
			annotations: map[string]string{"other": "annotation", "another": "annotation"},
			expectBool:  false,
		}),
		Entry("nil annotations", testCaseHasHostAnnotation{
			annotations: nil,
			expectBool:  false,
		}),
	)
})

func Test_Image_String(t *testing.T) {
	for _, row := range []struct {
		image    Image
		expected string
	}{
		{
			Image{
				URL:  "",
				Name: "",
				Path: "",
			},
			"",
		},
		{
			Image{
				URL:  "https://user:pwd@example.com/images/Ubuntu-2404-noble-amd64-custom.tar.gz",
				Name: "Ubuntu-2404-noble",
				Path: "",
			},
			"Ubuntu-2404-noble (https://user:xxxxx@example.com/images/Ubuntu-2404-noble-amd64-custom.tar.gz)",
		},
		{
			Image{
				URL:  "https://example.com/foo.tgz",
				Name: "foo",
				Path: "",
			},
			"foo (https://example.com/foo.tgz)",
		},
		{
			Image{
				URL:  "https://example.com/nameless.tgz",
				Path: "",
			},
			"https://example.com/nameless.tgz",
		},
		{
			Image{
				Name: "nfs",
				Path: "/root/.oldroot/nfs/images/Ubuntu-2404-noble-amd64-base.tar.gz",
			},
			"nfs (/root/.oldroot/nfs/images/Ubuntu-2404-noble-amd64-base.tar.gz)",
		},
	} {
		require.Equal(t, row.expected, row.image.String())
	}
}
