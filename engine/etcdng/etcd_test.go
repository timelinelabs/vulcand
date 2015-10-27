package etcdng

import (
	"os"
	"strings"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/timelinelabs/vulcand/engine/test"
	"github.com/timelinelabs/vulcand/plugin/registry"
	"github.com/timelinelabs/vulcand/secret"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { TestingT(t) }

type EtcdSuite struct {
	ng          *ng
	suite       test.EngineSuite
	nodes       []string
	etcdPrefix  string
	consistency string
	client      *etcd.Client
	changesC    chan interface{}
	key         string
	stopC       chan bool
}

var _ = Suite(&EtcdSuite{
	etcdPrefix:  "/vulcandtest",
	consistency: etcd.STRONG_CONSISTENCY,
})

func (s *EtcdSuite) SetUpSuite(c *C) {
	log.InitWithConfig(log.Config{Name: "console"})

	key, err := secret.NewKeyString()
	if err != nil {
		panic(err)
	}
	s.key = key

	nodes_string := os.Getenv("VULCAND_TEST_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}

	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry

	key, err := secret.KeyFromString(s.key)
	c.Assert(err, IsNil)

	box, err := secret.NewBox(key)
	c.Assert(err, IsNil)

	engine, err := New(
		s.nodes,
		s.etcdPrefix,
		registry.GetRegistry(),
		Options{
			EtcdConsistency: s.consistency,
			Box:             box,
		})
	c.Assert(err, IsNil)
	s.ng = engine.(*ng)
	s.client = s.ng.client

	// Delete all values under the given prefix
	_, err = s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		// There's no key like this
		if !notFound(err) {
			// We haven't expected this error, oops
			c.Assert(err, IsNil)
		}
	} else {
		_, err = s.ng.client.Delete(s.etcdPrefix, true)
		c.Assert(err, IsNil)
	}

	s.changesC = make(chan interface{})
	s.stopC = make(chan bool)
	go s.ng.Subscribe(s.changesC, s.stopC)

	s.suite.ChangesC = s.changesC
	s.suite.Engine = engine
}

func (s *EtcdSuite) TearDownTest(c *C) {
	close(s.stopC)
	s.ng.Close()
}

func (s *EtcdSuite) TestEmptyParams(c *C) {
	s.suite.EmptyParams(c)
}

func (s *EtcdSuite) TestHostCRUD(c *C) {
	s.suite.HostCRUD(c)
}

func (s *EtcdSuite) TestHostWithKeyPair(c *C) {
	s.suite.HostWithKeyPair(c)
}

func (s *EtcdSuite) TestHostUpsertKeyPair(c *C) {
	s.suite.HostUpsertKeyPair(c)
}

func (s *EtcdSuite) TestHostWithOCSP(c *C) {
	s.suite.HostWithOCSP(c)
}

func (s *EtcdSuite) TestListenerCRUD(c *C) {
	s.suite.ListenerCRUD(c)
}

func (s *EtcdSuite) TestListenerSettingsCRUD(c *C) {
	s.suite.ListenerSettingsCRUD(c)
}

func (s *EtcdSuite) TestBackendCRUD(c *C) {
	s.suite.BackendCRUD(c)
}

func (s *EtcdSuite) TestBackendDeleteUsed(c *C) {
	s.suite.BackendDeleteUsed(c)
}

func (s *EtcdSuite) TestBackendDeleteUnused(c *C) {
	s.suite.BackendDeleteUnused(c)
}

func (s *EtcdSuite) TestServerCRUD(c *C) {
	s.suite.ServerCRUD(c)
}

func (s *EtcdSuite) TestServerExpire(c *C) {
	s.suite.ServerExpire(c)
}

func (s *EtcdSuite) TestFrontendCRUD(c *C) {
	s.suite.FrontendCRUD(c)
}

func (s *EtcdSuite) TestFrontendExpire(c *C) {
	s.suite.FrontendExpire(c)
}

func (s *EtcdSuite) TestFrontendBadBackend(c *C) {
	s.suite.FrontendBadBackend(c)
}

func (s *EtcdSuite) TestMiddlewareCRUD(c *C) {
	s.suite.MiddlewareCRUD(c)
}

func (s *EtcdSuite) TestMiddlewareExpire(c *C) {
	s.suite.MiddlewareExpire(c)

}

func (s *EtcdSuite) TestMiddlewareBadFrontend(c *C) {
	s.suite.MiddlewareBadFrontend(c)
}

func (s *EtcdSuite) TestMiddlewareBadType(c *C) {
	s.suite.MiddlewareBadType(c)
}
