package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
)

type input struct {
	token  string
	cookie string
	url    *url.URL
	output string
}

var inp input

func init() {
	flag.StringVar(&inp.token, "bearer-token", "", "Authorization token for quay.io repository")
	flag.StringVar(&inp.token, "bt", "", "Authorization token for quay.io repository")

	flag.StringVar(&inp.cookie, "cookie", "", "Cookie for private Docker registry Authentication")
	flag.StringVar(&inp.cookie, "c", "", "Cookie for private Docker registry Authentication")

	flag.StringVar(&inp.output, "output-file", "", "Absolute path of the desired file for output of response body")
	flag.StringVar(&inp.output, "o", "", "Absolute path of the desired file for output of response body")

}

func main() {

	flag.Parse()

	fmt.Println("args", flag.Args())

	args := flag.Args()
	if len(args) != 1 {
		log.Printf("Invalid arguments (expected url only): %+v", args)
		os.Exit(1)
	}

	u, err := url.Parse(args[0])
	if err != nil {
		log.Printf("Invalid URL: %s", args[0])
	}

	inp.url = u

	err = download(inp)
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

}

func download(inp input) error {

	client := &http.Client{}
	// var err error
	// client.Jar, err = cookiejar.New(nil)
	// if err != nil {
	// 	return err
	// }

	// cookie := &http.Cookie{
	// 	Name:  "The cookie",
	// 	Value: inp.cookie,
	// }

	// client.Jar.SetCookies(inp.url, []*http.Cookie{cookie})

	req, err := http.NewRequest("GET", inp.url.String(), nil)

	if inp.token != "" {
		req.Header.Set("Authorization", inp.token)
	}

	if inp.cookie != "" {
		req.Header.Set("Cookie", inp.cookie)
	}

	byt, err := httputil.DumpRequest(req, false)
	if err != nil {
		return err
	}
	fmt.Printf("Request: %s\n", string(byt))

	resp, err := client.Do(req)

	byt, err = httputil.DumpResponse(resp, false)
	if err != nil {
		return err
	}
	fmt.Printf("Response: %s", string(byt))

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	if resp.Body == nil {
		return fmt.Errorf("Expected a response body")
	}
	defer resp.Body.Close()

	_, err = os.Stat(inp.output)
	if err == nil {
		os.Remove(inp.output)
	} else if !os.IsNotExist(err) {
		fmt.Println("err", err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(inp.output), os.FileMode(0755)); err != nil {
		return err
	}

	var f *os.File

	if inp.output != "" {
		f, err = os.Create(inp.output)
		if err != nil {
			fmt.Println("create")
			return err
		}
		defer f.Close()
	} else {
		f = os.Stdout
	}

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	} else if n != resp.ContentLength {
		return fmt.Errorf("Length of data read to file (%d) does not match content length in reponse headers: %d", n, resp.ContentLength)
	}

	return nil
}
