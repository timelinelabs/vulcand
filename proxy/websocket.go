package proxy

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/roundrobin"
	// "golang.org/x/net/websocket"
)

// Original developpement made by https://github.com/koding/websocketproxy
var (
	// DefaultUpgrader specifies the parameters for upgrading an HTTP
	// connection to a WebSocket connection.
	DefaultUpgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	// DefaultDialer is a dialer with all fields set to the default zero values.
	DefaultDialer = websocket.DefaultDialer
)

// WebsocketUpgrader is an HTTP middleware that detects websocket upgrade requests
// and establishes an HTTP connection via a chosen backend server
type WebsocketUpgrader struct {
	next http.Handler
	rr   *roundrobin.RoundRobin
	f    *frontend
}

type WebsocketProxy struct {
	URL      func(*http.Request) *url.URL
	Upgrader *websocket.Upgrader
	Dialer   *websocket.Dialer
}

// create the upgrader via a roundrobin and the expected next handler (if not websocket)
// also make sure a websocket server exists
func newWebsocketUpgrader(rr *roundrobin.RoundRobin, next http.Handler, f *frontend) *WebsocketUpgrader {
	return &WebsocketUpgrader{
		next: next,
		rr:   rr,
		f:    f,
	}
}

// ServeHTTP waits for a websocket upgrade request and creates a TCP connection between
// the backend server and the frontend
func (w *WebsocketUpgrader) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	if strings.Join(req.Header["Upgrade"], "") != "websocket" {
		w.next.ServeHTTP(wr, req)
		return
	}

	url, er := w.rr.NextServer()
	if er != nil {
		log.Errorf("Round robin failed in websocket middleware: %v", er)
		return
	}
	wsProxy(url).ServerHTTP(wr, req)
}

func wsProxy(u *url.URL) *WebsocketProxy {
	return &WebsocketProxy{
		URL: func(r *http.Request) *url.URL {
			uu := *u
			uu.Fragment = r.URL.Fragment
			uu.Path = r.URL.Path
			uu.RawQuery = r.URL.RawQuery
			switch u.Scheme {
			case "http":
				uu.Scheme = "ws"
			case "https":
				uu.Scheme = "wss"
			}
			return &uu
		},
	}
}

func (w *WebsocketProxy) ServerHTTP(rw http.ResponseWriter, req *http.Request) {
	if w.URL == nil {
		log.Errorf("websocketproxy: backend function is not defined")
		http.Error(rw, "internal server error (code: 1)", http.StatusInternalServerError)
		return
	}

	backendURL := w.URL(req)
	if backendURL == nil {
		log.Errorf("websocketproxy: backend URL is nil")
		http.Error(rw, "internal server error (code: 2)", http.StatusInternalServerError)
		return
	}

	dialer := w.Dialer
	if w.Dialer == nil {
		dialer = DefaultDialer
	}

	// Pass headers from the incoming request to the dialer to forward them to
	// the final destinations.
	requestHeader := http.Header{}
	requestHeader.Add("Origin", req.Header.Get("Origin"))
	for _, prot := range req.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
		requestHeader.Add("Sec-WebSocket-Protocol", prot)
	}
	for _, cookie := range req.Header[http.CanonicalHeaderKey("Cookie")] {
		requestHeader.Add("Cookie", cookie)
	}

	// Pass X-Forwarded-For headers too, code below is a part of
	// httputil.ReverseProxy. See http://en.wikipedia.org/wiki/X-Forwarded-For
	// for more information
	// TODO: use RFC7239 http://tools.ietf.org/html/rfc7239
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		requestHeader.Set("X-Forwarded-For", clientIP)
	}

	// Set the originating protocol of the incoming HTTP request. The SSL might
	// be terminated on our site and because we doing proxy adding this would
	// be helpful for applications on the backend.
	requestHeader.Set("X-Forwarded-Proto", "http")
	if req.TLS != nil {
		requestHeader.Set("X-Forwarded-Proto", "https")
	}

	// Connect to the backend URL, also pass the headers we get from the requst
	// together with the Forwarded headers we prepared above.
	// TODO: support multiplexing on the same backend connection instead of
	// opening a new TCP connection time for each request. This should be
	// optional:
	// http://tools.ietf.org/html/draft-ietf-hybi-websocket-multiplexing-01
	connBackend, resp, err := dialer.Dial(backendURL.String(), nil)
	if err != nil {
		log.Errorf("websocketproxy: couldn't dial to remote backend url %s, %s, %+v", backendURL.String(), err, resp)
		return
	}
	defer connBackend.Close()

	upgrader := w.Upgrader
	if w.Upgrader == nil {
		upgrader = DefaultUpgrader
	}

	// Only pass those headers to the upgrader.
	upgradeHeader := http.Header{}
	upgradeHeader.Set("Sec-WebSocket-Protocol",
		resp.Header.Get(http.CanonicalHeaderKey("Sec-WebSocket-Protocol")))
	upgradeHeader.Set("Set-Cookie",
		resp.Header.Get(http.CanonicalHeaderKey("Set-Cookie")))

	// Now upgrade the existing incoming request to a WebSocket connection.
	// Also pass the header that we gathered from the Dial handshake.
	connPub, err := upgrader.Upgrade(rw, req, upgradeHeader)
	if err != nil {
		log.Errorf("websocketproxy: couldn't upgrade %s\n", err)
		return
	}
	defer connPub.Close()

	var wg sync.WaitGroup
	cp := func(dst io.Writer, src io.Reader) {
		defer wg.Done()
		io.Copy(dst, src)
	}

	// Start our proxy now, everything is ready...
	wg.Add(2)
	go cp(connBackend.UnderlyingConn(), connPub.UnderlyingConn())
	go cp(connPub.UnderlyingConn(), connBackend.UnderlyingConn())
	wg.Wait()
}
