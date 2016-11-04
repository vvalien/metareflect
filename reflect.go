// A web app for Google App Engine that proxies HTTP requests and responses
package reflect

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
	"appengine"
	"appengine/urlfetch"
)

const (
	forwardURL = "https://metareflect.mooo.com/"
	// A timeout of 0 means to use the App Engine default (5 seconds).
	urlFetchTimeout = 20 * time.Second
)

var context appengine.Context

// Join two URL paths.
func pathJoin(a, b string) string {
	if len(a) > 0 && a[len(a)-1] == '/' {
		a = a[:len(a)-1]
	}
	if len(b) == 0 || b[0] != '/' {
		b = "/" + b
	}
	return a + b
}

// We reflect only a whitelisted set of header fields. 
var reflectedHeaderFields = []string{
	"Content-Type",
	"X-Session-Id",
    "User-Agent",
}

func getClientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Make a copy of r, with the URL being changed to be relative to forwardURL,
func copyRequest(r *http.Request) (*http.Request, error) {
	u, err := url.Parse(forwardURL)
	if err != nil {
		return nil, err
	}
	u.Path = pathJoin(u.Path, r.URL.Path)
	c, err := http.NewRequest(r.Method, u.String(), r.Body)
	if err != nil {
		return nil, err
	}
	for _, key := range reflectedHeaderFields {
		value := r.Header.Get(key)
		if value != "" {
			c.Header.Add(key, value)
		}
	}

	// c.Header.Add("IP", getClientAddr(r))
	return c, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	context = appengine.NewContext(r)
	fr, err := copyRequest(r)
	if err != nil {
		context.Errorf("copyRequest: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	transport := urlfetch.Transport{
		Context: context,
		Deadline: urlFetchTimeout,
	}
	resp, err := transport.RoundTrip(fr)
	if err != nil {
		context.Errorf("RoundTrip: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for _, key := range reflectedHeaderFields {
		value := resp.Header.Get(key)
		if value != "" {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		context.Errorf("io.Copy after %d bytes: %s", n, err)
	}
}

func init() {
	http.HandleFunc("/", handler)
}
