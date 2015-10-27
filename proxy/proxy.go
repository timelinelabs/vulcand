package proxy

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/timelinelabs/vulcand/engine"
	"github.com/timelinelabs/vulcand/plugin"
)

type Proxy interface {
	engine.StatsProvider

	UpsertHost(engine.Host) error
	DeleteHost(engine.HostKey) error

	UpsertListener(engine.Listener) error
	DeleteListener(engine.ListenerKey) error

	UpsertBackend(engine.Backend) error
	DeleteBackend(engine.BackendKey) error

	UpsertFrontend(engine.Frontend) error
	DeleteFrontend(engine.FrontendKey) error

	UpsertMiddleware(engine.FrontendKey, engine.Middleware) error
	DeleteMiddleware(engine.MiddlewareKey) error

	UpsertServer(engine.BackendKey, engine.Server) error
	DeleteServer(engine.ServerKey) error

	// TakeFiles takes file descriptors representing sockets in listening state to start serving on them
	// instead of binding. This is nessesary if the child process needs to inherit sockets from the parent
	// (e.g. for graceful restarts)
	TakeFiles([]*FileDescriptor) error

	// GetFiles exports listening socket's underlying dupped file descriptors, so they can later
	// be passed to child process or to another Server
	GetFiles() ([]*FileDescriptor, error)

	Start() error
	Stop(wait bool)
}

type Options struct {
	MetricsClient      metrics.Client
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	MaxHeaderBytes     int
	DefaultListener    *engine.Listener
	Files              []*FileDescriptor
	TimeProvider       timetools.TimeProvider
	NotFoundMiddleware plugin.Middleware
}

type NewProxyFn func(id int) (Proxy, error)

type FileDescriptor struct {
	Address engine.Address
	File    *os.File
}

func (fd *FileDescriptor) ToListener() (net.Listener, error) {
	listener, err := net.FileListener(fd.File)
	if err != nil {
		return nil, err
	}
	fd.File.Close()
	return listener, nil
}

func (fd *FileDescriptor) String() string {
	return fmt.Sprintf("FileDescriptor(%s, %d)", fd.Address, fd.File.Fd())
}
