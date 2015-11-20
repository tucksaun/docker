package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/autogen/dockerversion"
)

var (
	errConnectionFailed = errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")
)

type serverResponse struct {
	body       io.ReadCloser
	header     http.Header
	statusCode int
}

func (cli *DockerCli) callRaw(method, path string, data interface{}, headers map[string][]string) (*serverResponse, error) {
	params, err := cli.encodeData(data)
	if err != nil {
		sr := &serverResponse{
			body:       nil,
			header:     nil,
			statusCode: -1,
		}
		return sr, nil
	}

	if data != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	serverResp, err := cli.clientRawRequest(method, path, params, headers)
	return serverResp, err
}

func (cli *DockerCli) streamRaw(method, path string, opts *streamOpts) (*serverResponse, error) {
	serverResp, err := cli.clientRawRequest(method, path, opts.in, opts.headers)
	if err != nil {
		return serverResp, err
	}
	return serverResp, cli.streamBody(serverResp.body, serverResp.header.Get("Content-Type"), opts.rawTerminal, opts.out, opts.err)
}

func (cli *DockerCli) clientRawRequest(method, path string, in io.Reader, headers map[string][]string) (*serverResponse, error) {

	serverResp := &serverResponse{
		body:       nil,
		statusCode: -1,
	}

	expectedPayload := (method == "POST" || method == "PUT")
	if expectedPayload && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.Version, path), in)
	if err != nil {
		return serverResp, err
	}

	// Add CLI Config's HTTP Headers BEFORE we set the Docker headers
	// then the user can't change OUR headers
	for k, v := range cli.configFile.HttpHeaders {
		req.Header.Set(k, v)
	}

	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION+" ("+runtime.GOOS+")")
	req.URL.Host = cli.addr
	req.URL.Scheme = cli.scheme

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	resp, err := cli.HTTPClient().Do(req)
	if resp != nil {
		serverResp.statusCode = resp.StatusCode
	}

	if err != nil {
		if IsTimeout(err) || strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "dial unix") {
			return serverResp, errConnectionFailed
		}

		if cli.tlsConfig == nil && strings.Contains(err.Error(), "malformed HTTP response") {
			return serverResp, fmt.Errorf("%v.\n* Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}
		if cli.tlsConfig != nil && strings.Contains(err.Error(), "remote error: bad certificate") {
			return serverResp, fmt.Errorf("The server probably has client authentication (--tlsverify) enabled. Please check your TLS client certification settings: %v", err)
		}

		return serverResp, fmt.Errorf("An error occurred trying to connect: %v", err)
	}

	if serverResp.statusCode < 200 || serverResp.statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return serverResp, err
		}
		if len(body) == 0 {
			return serverResp, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(serverResp.statusCode), req.URL)
		}
		return serverResp, fmt.Errorf("Error response from daemon: %s", bytes.TrimSpace(body))
	}

	serverResp.body = resp.Body
	serverResp.header = resp.Header
	return serverResp, nil
}

// IsTimeout takes an error returned from (generally) the http package and determines if it is a timeout error.
func IsTimeout(err error) bool {
	switch e := err.(type) {
	case net.Error:
		return e.Timeout()
	case *url.Error:
		if t, ok := e.Err.(net.Error); ok {
			return t.Timeout()
		}
		return false
	default:
		return false
	}
}
