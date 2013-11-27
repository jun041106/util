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
	"path"
)

// Wrap HTTP verbs for stronger typing.
type Method string

const (
	GET    = Method("GET")
	POST   = Method("POST")
	PUT    = Method("PUT")
	DELETE = Method("DELETE")
)

// Client represents a client bound to a given REST base URL.
type Client struct {
	// Driver is the *http.Client that performs requests.
	Driver *http.Client
	// base is the URL under which all REST-ful resources are available.
	base *url.URL
}

// New returns a *Client with the specified base URL endpoint, expected to
// include the port string and any path, if required. Returns an error if
// baseurl cannot be parsed as an absolute URL.
func New(baseurl string) (*Client, error) {
	base, err := url.ParseRequestURI(baseurl)
	if err != nil {
		return nil, err
	} else if !base.IsAbs() || base.Host == "" {
		return nil, fmt.Errorf("URL is not absolute: %s", baseurl)
	}
	return &Client{Driver: http.DefaultClient, base: base}, nil
}

// BaseURL returns a *url.URL to a copy of Client's base so the caller may
// modify it.
func (c *Client) BaseURL() *url.URL {
	return c.base.ResolveReference(&url.URL{})
}

// Get issues a GET request to the specified endpoint and parses the response in
// to v. It will return an error if it failed to send the request, or a
// *RestError if the response wasn't a 2xx status code.
func (c *Client) Get(endpoint string, v interface{}) error {
	return c.Do(c.NewJsonRequest(GET, endpoint, nil), v)
}

// Post issues a POST request to the specified endpoint with the obj payload
// encoded as JSON. It will parse the response and write it back to v. It will
// return an error if it failed to send the request, or a *RestError if the
// response wasn't a 2xx status code.
func (c *Client) Post(endpoint string, obj interface{}, v interface{}) error {
	return c.Do(c.NewJsonRequest(POST, endpoint, obj), v)
}

// Put issues a PUT request to the specified endpoint with the obj payload
// encoded as JSON. It will parse the response and write it back to v. It will
// return an error if it failed to send the request, or a *RestError if the
// response wasn't a 2xx status code.
func (c *Client) Put(endpoint string, obj interface{}, v interface{}) error {
	return c.Do(c.NewJsonRequest(PUT, endpoint, obj), v)
}

// Delete issues a DELETE request to the specified endpoint and parses the
// response in to v. It will return an error if it failed to send the request,
// or a *RestError if the response wasn't a 2xx status code.
func (c *Client) Delete(endpoint string, v interface{}) error {
	return c.Do(c.NewJsonRequest(DELETE, endpoint, nil), v)
}

// Do performs the HTTP request described by req and unmarshals any resulting
// value into v unless v is nil. Returns a *RestError if an error occurs. The
// caller should Close() the *RestError.Response if it is set.
func (c *Client) Do(req *Request, v interface{}) error {
	hreq, err := req.HTTPRequest()
	if err != nil {
		return &RestError{Req: hreq, err: fmt.Errorf("error preparing request: %s", err)}
	}
	// Internally, this uses c.Driver's CheckRedirect policy.
	resp, err := c.Driver.Do(hreq)
	if err != nil {
		return &RestError{Req: hreq, err: fmt.Errorf("error sending request: %s", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &RestError{Req: hreq, Response: resp, err: fmt.Errorf("error in response: %s", resp.Status)}
	}

	// Don't Unmarshal if v is nil
	if v == nil {
		resp.Body.Close() // Not going to read resp.Body
		return nil
	}

	ctype, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch {
	case err != nil:
		return &RestError{Req: hreq, Response: resp, err: err}
	case ctype == "application/json":
		defer resp.Body.Close()
		return json.NewDecoder(resp.Body).Decode(v)
	default:
		return &RestError{Req: hreq, Response: resp, err: fmt.Errorf("unexpected response: %s %s", resp.Status, ctype)}
	}

	return nil
}

// NewJsonRequest generates a new Request object and JSON encodes the provided
// obj. The JSON object will be set as the body and included in the request.
func (c *Client) NewJsonRequest(method Method, endpoint string, obj interface{}) *Request {
	req := c.newRequest(method, endpoint)

	// set how to generate the body if obj isn't null
	if obj != nil {
		req.prepare = func(httpReq *http.Request) error {
			var buffer bytes.Buffer
			encoder := json.NewEncoder(&buffer)
			if err := encoder.Encode(obj); err != nil {
				return err
			}

			// set to the request
			httpReq.Body = ioutil.NopCloser(&buffer)
			httpReq.ContentLength = int64(buffer.Len())
			httpReq.Header.Set("Content-Type", "application/json")
			return nil
		}
	}

	return req
}

// NewFormRequest generates a new Request object with a form encoded body based
// on the params map.
func (c *Client) NewFormRequest(method Method, endpoint string, params map[string]string) *Request {
	req := c.newRequest(method, endpoint)

	// set how to generate the body
	req.prepare = func(httpReq *http.Request) error {
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

// newRequest returns a *Request ready to be used by one of Client's exported
// methods like NewFormRequest.
func (c *Client) newRequest(method Method, endpoint string) *Request {
	return &Request{
		Method:  method,
		URL:     resourceURL(c.BaseURL(), endpoint),
		Headers: make(map[string]string),
	}
}

// Request encapsulates functionality making it easier to build REST requests.
type Request struct {
	Method  Method
	URL     *url.URL
	Headers map[string]string

	prepare func(*http.Request) error
}

// HTTPRequest returns an *http.Request populated with data from r. It may be
// executed by any http.Client.
func (r *Request) HTTPRequest() (*http.Request, error) {
	req, err := http.NewRequest(string(r.Method), r.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	// merge headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	// generate the body
	if r.prepare != nil {
		if err := r.prepare(req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// resourceURL returns a *url.URL with the path resolved for a resource under base.
func resourceURL(base *url.URL, relPath string) *url.URL {
	ref := &url.URL{Path: path.Join(base.Path, relPath)}
	return base.ResolveReference(ref)
}

// RestError is returned from REST transmissions to allow for inspection of
// failed request and response contents.
type RestError struct {
	// The Request that may have triggered the error.
	Req *http.Request
	// The Response that may have triggered the error, embedded for convenience
	*http.Response
	// err is the original error
	err error
}

func (r *RestError) Error() string {
	return fmt.Sprintf("REST error: %s", r.err)
}
