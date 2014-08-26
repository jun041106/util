// Copyright 2014 Apcera Inc. All rights reserved.
// Borrowing general test structure from Docker mock registry test.

package docker

import (
	"io/ioutil"
	"sort"
	"testing"

	"github.com/apcera/util/dockertest"

	tt "github.com/apcera/util/testtool"
)

func init() {
	registry := dockertest.RunMockRegistry()
	IndexURL = registry.URL
}

func TestGetImage(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "image name is empty")

	img, err = GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, img.Name, "foo/bar")

	img, err = GetImage("base")
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, img.Name, "library/base")
}

func TestGetImageHistory(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	h, err := img.History("tag2")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "can't find tag 'tag2' for image 'foo/bar'")

	h, err = img.History("latest")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(h), 2)
	tt.TestEqual(t, h[0], "deadbeef")
	tt.TestEqual(t, h[1], "badcafe")
}

func TestGetImageTags(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	tags := img.Tags()
	sort.Strings(tags)
	tt.TestEqual(t, tags, []string{"base", "latest"})
}

func TestGetImageTagLayerID(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	_, err = img.TagLayerID("tag2")
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "can't find tag 'tag2' for image 'foo/bar'")

	id, err := img.TagLayerID("latest")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, id, "deadbeef")

	id, err = img.TagLayerID("base")
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, id, "badcafe")
}

func TestGetImageMetadata(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	var m1 map[string]interface{}
	err = img.Metadata("tag2", &m1)
	tt.TestExpectError(t, err)
	tt.TestEqual(t, err.Error(), "can't find tag 'tag2' for image 'foo/bar'")

	var m2 map[string]interface{}
	err = img.Metadata("latest", &m2)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(m2), 2)
	tt.TestEqual(t, m2["id"], "deadbeef")
	tt.TestEqual(t, m2["k2"], "v2")

	var m3 map[string]interface{}
	err = img.Metadata("base", &m3)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, len(m3), 2)
	tt.TestEqual(t, m3["id"], "badcafe")
	tt.TestEqual(t, m3["k1"], "v1")
}

func TestReadLayer(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	img, err := GetImage("foo/bar")
	tt.TestExpectSuccess(t, err)

	r, err := img.LayerReader("deadbeef")
	tt.TestExpectSuccess(t, err)
	body, err := ioutil.ReadAll(r)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, body, []byte{0xd4, 0xe5, 0xf6})

	r, err = img.LayerReader("badcafe")
	tt.TestExpectSuccess(t, err)
	body, err = ioutil.ReadAll(r)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, body, []byte{0xa1, 0xb2, 0xc3})

	r, err = img.LayerReader("badbad")
	tt.TestExpectError(t, err)
}
