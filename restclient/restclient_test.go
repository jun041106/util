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

	client := New(server.Listener.Addr().String())
	req := client.NewJsonRequest("POST", "/foo", map[string]string{"bar": "baz"})

	var res map[string]string
	err := req.Result(&res)
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, len(res), 1)
	tt.TestEqual(t, res["foo"], "bar")
	tt.TestEqual(t, method, "POST")
	tt.TestEqual(t, path, "/foo")
	tt.TestEqual(t, body, `{"bar":"baz"}`+"\n")
}

type person struct {
	Name string
	Age  int
}

func TestJsonStructRequest(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	var receivedPerson *person
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

	client := New(server.Listener.Addr().String())
	req := client.NewJsonRequest("POST", "/", person{Name: "John", Age: 56})

	var responsePerson person
	err := req.Result(&responsePerson)
	tt.TestExpectSuccess(t, err)

	tt.TestNotEqual(t, receivedPerson, nil)
	tt.TestNotEqual(t, responsePerson, nil)

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
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := New(server.Listener.Addr().String())
	req := client.NewFormRequest("POST", "/", map[string]string{"name": "Tim"})
	err := req.Result(nil)
	tt.TestExpectSuccess(t, err)

	tt.TestEqual(t, form.Get("name"), "Tim")
}

func TestErrorResponse(t *testing.T) {
	tt.StartTest(t)
	defer tt.FinishTest(t)

	// create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "Didn't work")
	}))
	defer server.Close()

	client := New(server.Listener.Addr().String())
	req := client.NewFormRequest("GET", "/", nil)
	err := req.Result(nil)
	tt.TestExpectError(t, err)

	herr, ok := err.(*HttpError)
	tt.TestEqual(t, ok, true, "Should have casted to an *HttpError")
	tt.TestEqual(t, herr.StatusCode, 500)
	tt.TestEqual(t, herr.Status, "500 Internal Server Error")
	tt.TestEqual(t, string(herr.Body), "Didn't work")
	tt.TestEqual(t, err.Error(), "[500] 500 Internal Server Error, Body: Didn't work")
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

	client := New(server.Listener.Addr().String())
	req := client.NewJsonRequest("GET", "/", nil)

	var responsePerson person
	err := req.Result(&responsePerson)
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

	client := New(server.Listener.Addr().String())
	req := client.NewJsonRequest("GET", "/", nil)

	var responsePerson person
	err := req.Result(&responsePerson)
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

	client := New(server.Listener.Addr().String())
	req := client.NewJsonRequest("POST", "/", nil)

	err := req.Result(nil)
	tt.TestExpectSuccess(t, err)
	tt.TestEqual(t, body, "")
}
