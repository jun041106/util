// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"io/ioutil"
	"net/http"
	"strings"
)

// Test an HTTP GET to the given URL. If expectedReturnCode is a value other
// than -1, the test will fail if the response status code doesn't match the
// exptected code. Method returns the string value of the response body and the
// status code.
func TestHttpGet(t Logger, url string, expectedReturnCode int) (string, int) {
	// issue the request
	resp, err := http.Get(url)
	TestExpectSuccess(t, err)
	TestNotEqual(t, resp.Body, nil, "Response body shouldn't ever be nil")

	// read the response
	all, err := ioutil.ReadAll(resp.Body)
	TestExpectSuccess(t, err)
	TestExpectSuccess(t, resp.Body.Close())

	// check if a status code was given and check it if it wasn't -1
	if resp.StatusCode != expectedReturnCode && expectedReturnCode != -1 {
		Fatalf(
			t, "Failed unexpected statuscode for url %s: code=%d, "+
				"expectedCode=%d msg=%s",
			url, resp.StatusCode, expectedReturnCode, string(all))
	}
	return string(all), resp.StatusCode
}

// Test an HTTP POST to the given URL, with the given content type and request
// body.. If expectedReturnCode is a value other than -1, the test will fail if
// the response status code doesn't match the exptected code. Method returns the
// string value of the response body and the status code.
func TestHttpPost(
	t Logger, url string, contentType string, body string, expectedReturnCode int,
) (string, int) {
	// issue the request
	resp, err := http.Post(url, contentType, strings.NewReader(body))
	TestExpectSuccess(t, err)
	TestNotEqual(t, resp.Body, nil, "Response body shouldn't ever be nil")

	// read the response
	all, err := ioutil.ReadAll(resp.Body)
	TestExpectSuccess(t, err)
	TestExpectSuccess(t, resp.Body.Close())

	// check if a status code was given and check it if it wasn't -1
	if resp.StatusCode != expectedReturnCode && expectedReturnCode != -1 {
		Fatalf(
			t, "Failed unexpected statuscode for url %s: code=%d, "+
				"expectedCode=%d msg=%s",
			url, resp.StatusCode, expectedReturnCode, string(all))
	}
	return string(all), resp.StatusCode
}

// Test an HTTP PUT to the given URL, with the given content type and request
// body.. If expectedReturnCode is a value other than -1, the test will fail if
// the response status code doesn't match the exptected code. Method returns the
// string value of the response body and the status code.
func TestHttpPut(
	t Logger, url string, contentType string, body string, expectedReturnCode int,
) (string, int) {
	// create the request
	request, err := http.NewRequest("PUT", url, strings.NewReader(body))
	TestExpectSuccess(t, err)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	// issue the request
	resp, err := http.DefaultClient.Do(request)
	TestExpectSuccess(t, err)
	TestNotEqual(t, resp.Body, nil, "Response body shouldn't ever be nil")

	// read the response
	all, err := ioutil.ReadAll(resp.Body)
	TestExpectSuccess(t, err)
	TestExpectSuccess(t, resp.Body.Close())

	// check if a status code was given and check it if it wasn't -1
	if resp.StatusCode != expectedReturnCode && expectedReturnCode != -1 {
		Fatalf(
			t, "Failed unexpected statuscode for url %s: code=%d, "+
				"expectedCode=%d msg=%s",
			url, resp.StatusCode, expectedReturnCode, string(all))
	}
	return string(all), resp.StatusCode
}
