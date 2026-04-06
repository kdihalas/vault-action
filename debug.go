package main

import (
	"net/http"
	"net/http/httputil"

	"github.com/sethvargo/go-githubactions"
)

type debugTransport struct {
	rt http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface, allowing us to log outgoing HTTP
// requests and responses for debugging purposes.
func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(req, true)
	githubactions.Debugf("vault-client request:\n%s", dump)
	resp, err := d.rt.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	githubactions.Debugf("vault-client response: %s %s → %d", req.Method, req.URL.String(), resp.StatusCode)
	if resp.StatusCode >= 400 {
		respDump, _ := httputil.DumpResponse(resp, false)
		githubactions.Debugf("vault-client error response headers:\n%s", respDump)
	}
	return resp, err
}
