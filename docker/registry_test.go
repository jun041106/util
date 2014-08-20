// Copyright 2014 Apcera Inc. All rights reserved.
// Borrowing general test structure from Docker mock registry test.

package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/apcera/cntm-deps/mux"

	tt "github.com/apcera/util/testtool"
)

var (
	testVerbose    = false // Change to true in order to see HTTP requests in test output.
	testHttpServer *httptest.Server
	testLayers     = map[string]map[string]string{
		"badcafe": {
			"json":     `{"id":"badcafe","k1": "v1"}`,
			"ancestry": `["badcafe"]`,
			"layer":    string([]byte{0xa1, 0xb2, 0xc3}),
			"checksum": "1ac330d",
		},
		"deadbeef": {
			"json":     `{"id":"deadbeef","k2": "v2"}`,
			"ancestry": `["deadbeef", "badcafe"]`,
			"layer":    string([]byte{0xd4, 0xe5, 0xf6}),
			"checksum": "2bd330f",
		},
	}
	testRepositories = map[string]map[string]string{
		"foo/bar": {
			"latest": "deadbeef",
			"base":   "badcafe",
		},
		"library/base": {
			"latest": "badcafe",
		},
	}
)

func init() {
	r := mux.NewRouter()
	r.HandleFunc("/v1/images/{image_id:[^/]+}/{data_type:json|layer|ancestry}", handlerImage).Methods("GET")
	r.HandleFunc("/v1/repositories/{repository:.+}/tags", handlerTags).Methods("GET")
	r.HandleFunc("/v1/repositories/{repository:.+}/images", handlerImages).Methods("GET")

	testHttpServer = httptest.NewServer(logHandler(r))
	dockerIndexURL = testHttpServer.URL
}

func logHandler(handler http.Handler) http.Handler {
	if !testVerbose {
		return handler
	}
	lh := func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s \"%s %s\"\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(lh)
}

func writeResponse(w http.ResponseWriter, httpStatus int, payload interface{}) {
	w.WriteHeader(httpStatus)
	body, err := json.Marshal(payload)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	w.Write(body)
}

func checkAuth(w http.ResponseWriter, r *http.Request) bool {
	writeAuth := func() {
		sessionID := fmt.Sprintf("FAKE-SESSION-%d", time.Now().UnixNano())
		token := fmt.Sprintf("FAKE-TOKEN-%d", time.Now().UnixNano())

		http.SetCookie(w, &http.Cookie{Name: "session", Value: sessionID, MaxAge: 3600})
		w.Header().Add("X-Docker-Token", token)
	}

	// Mock registry doesn't really perform any auth.
	if len(r.Cookies()) > 0 {
		writeAuth()
		return true
	}

	if (len(r.Header.Get("Authorization"))) > 0 {
		writeAuth()
		return true
	}
	w.Header().Add("WWW-Authenticate", "token")
	writeResponse(w, 401, "Bad auth")
	return false
}

func handlerImages(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(testHttpServer.URL) // Ignoring error, URL is guaranteed to be OK.

	w.Header().Add("X-Docker-Endpoints", fmt.Sprintf("%s     ,    %s", u.Host, "test.example.com"))
	w.Header().Add("X-Docker-Token", fmt.Sprintf("FAKE-SESSION-%d", time.Now().UnixNano()))

	var images []map[string]string

	for imageID, layer := range testLayers {
		image := make(map[string]string)
		image["id"] = imageID
		image["checksum"] = layer["checksum"]
		images = append(images, image)
	}
	writeResponse(w, 200, images)
}

func handlerImage(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	layer, exists := testLayers[vars["image_id"]]
	if !exists {
		http.NotFound(w, r)
		return
	}

	layer_size := len(layer["layer"])
	w.Header().Add("X-Docker-Size", strconv.Itoa(layer_size))
	io.WriteString(w, layer[vars["data_type"]])
}

func handlerTags(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		return
	}

	vars := mux.Vars(r)
	tags, exists := testRepositories[vars["repository"]]
	if !exists {
		http.NotFound(w, r)
		return
	}

	writeResponse(w, 200, tags)
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
