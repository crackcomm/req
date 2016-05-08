package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var userAgent = "req-v0.0.1"

func main() {
	req := newRequest()
	req.scheme = "http"
	req.host = os.Getenv("REQ_HOST")
	req.path = splitPath(os.Getenv("REQ_PATH"))
	req.format = os.Getenv("REQ_FORMAT")

	if len(os.Args) <= 2 {
		fmt.Println("Usage: req [--host] [--path] [--header] [--auth] [--verbose] [--scheme] <method> <path> [<path> ...] [--] [<key>=<value> ...]")
		os.Exit(1)
	}

	err := parseArgs(os.Args[1:], req)
	if err != nil {
		fatal(err)
	}

	r, err := req.build()
	if err != nil {
		fatal(err)
	}

	if req.debug {
		r.Write(os.Stdout)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	if req.debug {
		err = resp.Write(os.Stdout)
	} else {
		_, err = io.Copy(os.Stdout, resp.Body)
	}
	if err != nil {
		fatal(err)
	}
}

func parseArgs(args []string, req *request) (err error) {
	var state int
	for n, arg := range args {
		switch state {
		case -1:
			state = 0
		case 0:
			switch arg {
			case "-v", "--verbose", "-d", "--debug":
				req.debug = true
			case "--scheme":
				if len(args) < n+1 {
					return errors.New("no --scheme value")
				}
				req.scheme = args[n+1]
				state = -1
			case "--host":
				if len(args) < n+1 {
					return errors.New("no --host value")
				}
				req.host = args[n+1]
				state = -1
			case "--format":
				if len(args) < n+1 {
					return errors.New("no --format value")
				}
				f := args[n+1]
				switch f {
				case "json", "form":
					req.format = f
				default:
					return fmt.Errorf("unknown format %q", f)
				}
				state = -1
			case "--path":
				if len(args) < n+1 {
					return errors.New("no --path value")
				}
				req.path = splitPath(args[n+1])
				state = -1
			case "--head", "--header":
				if len(args) < n+1 {
					return fmt.Errorf("no %s value", arg)
				}
				if err = req.addHeader(args[n+1]); err != nil {
					return
				}
				state = -1
			case "--auth":
				if len(args) < n+1 {
					return errors.New("no --auth value")
				}
				req.head.Set("Authorization", args[n+1])
				state = -1
			default:
				if strings.HasPrefix(arg, "-") {
					return fmt.Errorf("unknown flag %q", arg)
				}
				req.method = strings.ToUpper(arg)
				state = 1
			}
		case 1:
			if arg == "--" {
				state = 2
			} else {
				if req.host == "" {
					req.host = arg
				} else {
					req.path = append(req.path, arg)
				}
			}
		case 2:
			key, value, ok := splitKV(arg, "=")
			if !ok {
				return fmt.Errorf("key-value pair %q is invalid", arg)
			}
			if strings.HasPrefix(value, "@") {
				req.file[key] = strings.TrimPrefix(value, "@")
			} else if req.format != "" && req.format != "json" {
				req.body[key] = value
			} else {
				value = wrapString(value)
				var v interface{}
				err = json.Unmarshal([]byte(value), &v)
				if err != nil {
					return
				}
				req.body[key] = v
			}
		}
	}
	return
}

type request struct {
	scheme string
	host   string
	method string
	path   []string
	file   map[string]string
	body   map[string]interface{}
	head   url.Values
	debug  bool
	format string
}

func newRequest() *request {
	return &request{
		file: make(map[string]string),
		body: make(map[string]interface{}),
		head: url.Values{"User-Agent": []string{userAgent}},
	}
}

func (req *request) url() (u string) {
	u = fmt.Sprintf("%s://%s/%s", req.scheme, req.host, strings.Join(req.path, "/"))
	if req.method == "GET" && len(req.body) != 0 {
		q := make(url.Values)
		for key, value := range req.body {
			q.Set(key, fmt.Sprintf("%v", value))
		}
		u = fmt.Sprintf("%s?%s", u, q.Encode())
	}
	return
}

func (req *request) build() (r *http.Request, err error) {
	reader, err := req.reader()
	if err != nil {
		return
	}
	r, err = http.NewRequest(req.method, req.url(), reader)
	if err != nil {
		return
	}
	for key, value := range req.head {
		r.Header[key] = value
	}
	return
}

func (req *request) reader() (_ io.Reader, err error) {
	if len(req.file) != 0 {
		return req.mimeReader()
	}
	if len(req.body) == 0 || req.method == "GET" {
		return
	}
	if req.format == "form" {
		return req.formReader()
	}
	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(req.body)
	if err != nil {
		return
	}
	req.head.Set("Content-Type", "application/json")
	return body, nil
}

func (req *request) formReader() (_ io.Reader, err error) {
	data := make(url.Values)
	for key, value := range req.body {
		data.Set(key, fmt.Sprintf("%v", value))
	}
	req.head.Set("Content-Type", "application/x-www-form-urlencoded")
	return strings.NewReader(data.Encode()), nil
}

func (req *request) mimeReader() (_ io.Reader, err error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	for key, fname := range req.file {
		var part io.Writer
		part, err = writer.CreateFormFile(key, filepath.Base(fname))
		if err != nil {
			return
		}

		var file *os.File
		file, err = os.Open(fname)
		if err != nil {
			return
		}
		defer file.Close()

		_, err = io.Copy(part, file)
		if err != nil {
			return
		}
	}
	for key, value := range req.body {
		err = writer.WriteField(key, fmt.Sprintf("%v", value))
		if err != nil {
			return
		}
	}
	err = writer.Close()
	if err != nil {
		return
	}
	req.head.Set("Content-Type", "multipart/form-data")
	return body, nil
}

func (req *request) addHeader(h string) (err error) {
	key, value, ok := splitKV(h, ":")
	if !ok {
		return fmt.Errorf("header %q is invalid", h)
	}
	req.head.Set(key, value)
	return
}

func splitKV(kv, del string) (key, value string, ok bool) {
	s := strings.Index(kv, del)
	if s == -1 {
		return
	}
	return kv[:s], kv[s+1:], true
}

func wrapString(s string) string {
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") || strings.HasPrefix(s, "\"") {
		return s
	}
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsLetter(r) {
			return fmt.Sprintf("%q", s)
		}
	}
	return s
}

func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

func fatal(err error) {
	fmt.Println(err.Error())
	os.Exit(1)
}
