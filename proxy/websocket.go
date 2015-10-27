package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/roundrobin"
	"golang.org/x/net/websocket"
)

// WebsocketUpgrader is an HTTP middleware that detects websocket upgrade requests
// and establishes an HTTP connection via a chosen backend server
type WebsocketUpgrader struct {
	next     http.Handler
	rr       *roundrobin.RoundRobin
	f        *frontend
	wsServer *websocket.Server
}

// create the upgrader via a roundrobin and the expected next handler (if not websocket)
// also make sure a websocket server exists
func newWebsocketUpgrader(rr *roundrobin.RoundRobin, next http.Handler, f *frontend) *WebsocketUpgrader {
	wsServer := &websocket.Server{}
	wu := WebsocketUpgrader{
		next:     next,
		rr:       rr,
		f:        f,
		wsServer: wsServer,
	}
	wu.wsServer.Handler = websocket.Handler(wu.proxyWS)
	return &wu
}

// ServeHTTP waits for a websocket upgrade request and creates a TCP connection between
// the backend server and the frontend
func (u *WebsocketUpgrader) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// If request is websocket, serve with golang websocket server to do protocol handshake
	if strings.Join(req.Header["Upgrade"], "") == "websocket" {
		u.wsServer.ServeHTTP(w, req)
		return
	}

	u.next.ServeHTTP(w, req)
}

func (w *WebsocketUpgrader) proxyWS(ws *websocket.Conn) {
	url, er := w.rr.NextServer()
	if er != nil {
		log.Errorf("Can't round robin")
		return
	}

	u := url.String()
	switch url.Scheme {
	case "http":
		u = strings.Replace(u, "http", "ws", 1)
	case "https":
		u = strings.Replace(u, "https", "wss", 1)
	}

	ws2, err := websocket.Dial(strings.Join([]string{u, ws.Request().URL.String()}, ""),
		"", strings.Join([]string{url.Scheme, url.Host}, "://"))
	if err != nil {
		log.Errorf("Couldn't connect to backend server: %v", err)
		return
	}
	defer ws2.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go copyConn(ws, ws2, &wg)
	go copyConn(ws2, ws, &wg)
	wg.Wait()
}

func copyConn(in, out *websocket.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Infof("Begin copy...")
	io.Copy(in, out)
	log.Infof("Done")
}
