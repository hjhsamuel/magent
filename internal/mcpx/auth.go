package mcpx

import "net/http"

type Transport struct {
	Headers map[string]string
	Base    http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	return t.Base.RoundTrip(req)
}
