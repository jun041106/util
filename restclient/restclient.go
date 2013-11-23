// Copyright 2013 Apcera Inc. All rights reserved.

package restclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
)

// Method is simply a convenience wrapper around HTTP verbs to allow them to be
// strongly typed rather than loose strings.
type Method string

const (
	GET    = Method("GET")
	POST   = Method("POST")
	PUT    = Method("PUT")
	DELETE = Method("DELETE")
)

// Client represents a common base that requests are issued from.
type Client struct {
	Host string
}

// New creates a new *Client with the specified host as the base endpoint. Host
// is expected to include the port string if a port is required, such as
// "host:8080".
func New(host string) *Client {
	return &Client{
		Host: host,
	}
}

// Get issues a GET request to the specified endpoint and parses the response in
// to v. It will return an error if it failed to send the request, or a
// *HttpError if the response wasn't a 2xx status code.
func (c *Client) Get(endpoint string, v interface{}) error {
	return c.NewJsonRequest(GET, endpoint, nil).Result(&v)
}

// Post issues a POST request to the specified endpoint with the obj payload
// encoded as JSON. It will parse the response and write it back to v. It will
// return an error if it failed to send the request, or a *HttpError if the
// response wasn't a 2xx status code.
func (c *Client) Post(endpoint string, obj interface{}, v interface{}) error {
	return c.NewJsonRequest(POST, endpoint, obj).Result(&v)
}

// Put issues a PUT request to the specified endpoint with the obj payload
// encoded as JSON. It will parse the response and write it back to v. It will
// return an error if it failed to send the request, or a *HttpError if the
// response wasn't a 2xx status code.
func (c *Client) Put(endpoint string, obj interface{}, v interface{}) error {
	return c.NewJsonRequest(PUT, endpoint, obj).Result(&v)
}

// Delete issues a DELETE request to the specified endpoint and parses the
// response in to v. It will return an error if it failed to send the request,
// or a *HttpError if the response wasn't a 2xx status code.
func (c *Client) Delete(endpoint string, v interface{}) error {
	return c.NewJsonRequest(DELETE, endpoint, nil).Result(&v)
}

// NewJsonRequest generates a new Request object and JSON encodes the provided
// obj. The JSON object will be set as the body and included in the request.
func (c *Client) NewJsonRequest(method Method, endpoint string, obj interface{}) *Request {
	req := c.newRequest(method, endpoint)

	// set how to generate the body if obj isn't null
	if obj != nil {
		req.processRequest = func(httpReq *http.Request) error {
			buffer := bytes.NewBuffer([]byte{})
			encoder := json.NewEncoder(buffer)
			if err := encoder.Encode(obj); err != nil {
				return err
			}

			// set to the request
			httpReq.Body = ioutil.NopCloser(buffer)
			httpReq.ContentLength = int64(buffer.Len())
			httpReq.Header.Set("Content-Type", "application/json")
			return nil
		}
	}

	return req
}

// NewFormRequest generates a new Request object with a form encoded body based
// on the params that are passed in.
func (c *Client) NewFormRequest(method Method, endpoint string, params map[string]string) *Request {
	req := c.newRequest(method, endpoint)

	// set how to generate the body
	req.processRequest = func(httpReq *http.Request) error {
		form := url.Values{}
		for k, v := range params {
			form.Set(k, v)
		}
		encoded := form.Encode()

		// set to the request
		httpReq.Body = ioutil.NopCloser(bytes.NewReader([]byte(encoded)))
		httpReq.ContentLength = int64(len(encoded))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return nil
	}

	return req
}

func (c *Client) newRequest(method Method, endpoint string) *Request {
	return &Request{
		client:  c,
		Method:  method,
		Path:    endpoint,
		Headers: make(map[string]string),
	}
}

func (c *Client) httpClient() *http.Client {
	return http.DefaultClient
}

// Request represents an individual request issued against the Client it was
// created from.
type Request struct {
	client  *Client
	Method  Method
	Path    string
	Headers map[string]string

	processRequest func(*http.Request) error
}

func (r *Request) buildRequest() (*http.Request, error) {
	// build the url
	u := url.URL{
		Scheme: "http",
		Host:   r.client.Host,
		Path:   r.Path,
	}

	// build the request
	req, err := http.NewRequest(string(r.Method), u.String(), nil)
	if err != nil {
		return nil, err
	}

	// apply headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	// generate the body
	if r.processRequest != nil {
		if err := r.processRequest(req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// Response returns the raw *http.Response object and error from the request.
func (r *Request) Response() (*http.Response, error) {
	// get the request
	req, err := r.buildRequest()
	if err != nil {
		return nil, err
	}

	// get the response
	return r.client.httpClient().Do(req)
}

// Result triggers the request to be issued and parses the response into v
// based on the Content-Type of the response. It will return an error if the
// request fails or if a non-2xx status code is returned in the response.
func (r *Request) Result(v interface{}) error {
	// get the response
	resp, err := r.Response()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// check if it was an error and parse it
	// FIXME for now treat >=300 as error, handle redirects later
	if resp.StatusCode >= 300 {
		return parseError(resp)
	}

	// If we had a nil target and no error, then just return.
	if v == nil {
		return nil
	}

	// parse the content type string
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))

	// see what the response is and parse it
	switch contentType {
	case "application/json":
		return parseJson(resp, v)
	default:
		return parseError(resp)
	}
}

func parseJson(resp *http.Response, v interface{}) error {
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(v)
}

func parseError(resp *http.Response) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return &HttpError{
		Body:       body,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}

// HttpError is the type used when a request returns a non-2xx response.
type HttpError struct {
	Body       []byte
	StatusCode int
	Status     string
}

func (he *HttpError) Error() string {
	return fmt.Sprintf("[%d] %s, Body: %s", he.StatusCode, he.Status, string(he.Body))
}
