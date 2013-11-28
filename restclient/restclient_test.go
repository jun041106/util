// Copyright 2012-2013 Apcera Inc. All rights reserved.

package restclient

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	tt "github.com/apcera/util/testtool"
)

type person struct {
	Name string
	Age  int
}

func TestResourceURL(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	base := "http://example.com:8080/v1/resources"
	baseURL, err := url.Parse(base)
	tt.TestExpectSuccess(t, err)

	examples := []struct{ in, out string }{
		// Returns base when path == ""
		{"", "http://example.com:8080/v1/resources"},
		// Adds relative paths to end
		{"items/1234", "http://example.com:8080/v1/resources/items/1234"},
		{"/items/1234", "http://example.com:8080/v1/resources/items/1234"},
	}

	for i, ex := range examples {
		u := resourceURL(baseURL, ex.in)
		if u.String() != ex.out {
			t.Errorf("%d. resourceURL(..., %q) resolved incorrectly.\nhave: %s\nwant: %s",
				i, ex.in, u.String(), ex.out)
		}
	}
}

func TestNewRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	client, err := New("http://example.com/resources")
	tt.TestExpectSuccess(t, err)
	req := client.newRequest(GET, "/foos")

	tt.TestEqual(t, req.Method, GET)
	tt.TestEqual(t, req.URL.String(), "http://example.com/resources/foos")
	tt.TestEqual(t, req.Headers, map[string]string{})
}

func TestBasicJsonRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	method, path, body := "", "", ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		method = req.Method
		path = req.URL.Path

		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"foo":"bar"}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/foo", map[string]string{"bar": "baz"})

	var res map[string]string
	err = client.Result(req, &res)
	tt.TestExpectSuccess(t, err)

	// Verify request as received by server
	tt.TestEqual(t, method, "POST")
	tt.TestEqual(t, path, "/foo")
	tt.TestEqual(t, body, `{"bar":"baz"}`+"\n")

	// Verify response was parsed by client
	tt.TestEqual(t, len(res), 1)
	tt.TestEqual(t, res["foo"], "bar")
}

func TestJsonStructRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	var receivedPerson *person

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&receivedPerson)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"Name":"Molly","Age":45}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/", person{Name: "John", Age: 56})

	var responsePerson person
	err = client.Result(req, &responsePerson)
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, receivedPerson.Name, "John")
	tt.TestEqual(t, receivedPerson.Age, 56)

	tt.TestEqual(t, responsePerson.Name, "Molly")
	tt.TestEqual(t, responsePerson.Age, 45)
}

func TestFormRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		form = req.Form
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `{"foo":"bar"}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("POST", "/", map[string]string{"name": "Tim"})
	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)

	// Verify form data as received by server
	tt.TestEqual(t, form.Get("name"), "Tim")
}

func TestErrorResult(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		io.WriteString(w, "Didn't work")
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	err = client.Result(req, nil)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "REST error: "+rerr.err.Error())
}

func TestErrorResponse(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(500)
		io.WriteString(w, "Didn't work")
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewFormRequest("GET", "/", nil)
	resp, err := client.Do(req)
	tt.TestExpectError(t, err)

	rerr, ok := err.(*RestError)
	tt.TestEqual(t, ok, true, "Error should be of type *RestError")
	tt.TestEqual(t, rerr.Error(), "REST error: "+rerr.err.Error())

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, string(body), "Didn't work")

}

func TestInvalidJsonResponse(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		io.WriteString(w, `"Name":"Molly","Age":45}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("GET", "/", nil)

	var responsePerson person
	err = client.Result(req, &responsePerson)
	tt.TestExpectError(t, err)
	tt.TestNotEqual(t, err.(*json.UnmarshalTypeError), nil, "Should have been a json unmarshal error")
}

func TestParseMimetype(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		io.WriteString(w, `{"Name":"Molly","Age":45}`)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("GET", "/", nil)

	var responsePerson person
	err = client.Result(req, &responsePerson)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, responsePerson.Name, "Molly")
	tt.TestEqual(t, responsePerson.Age, 45)
}

func TestEmptyPostRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	body := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf("Error reading request: %v", err)
			w.WriteHeader(500)
			return
		}
		body = string(b)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, err := New(server.URL)
	tt.TestExpectSuccess(t, err)
	req := client.NewJsonRequest("POST", "/", nil)

	err = client.Result(req, nil)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, body, "")
}
