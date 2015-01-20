package agent

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rphillips/agent/client"
	"net"
	"sync"
	"time"
)

type StreamOptions struct {
	Queries map[string]string
}

type Stream struct {
	options *StreamOptions

	clientsLock sync.Mutex
	clients     map[string]*MonitoringClient

	Info *ClientInfo

	WaitChan chan struct{}
}

func NewStream(options *StreamOptions, info *ClientInfo) *Stream {
	return &Stream{
		WaitChan: make(chan struct{}),
		clients:  make(map[string]*MonitoringClient),
		options:  options,
		Info:     info,
	}
}

func (s *Stream) AddClient(datacenter string, cli *MonitoringClient) {
	s.clientsLock.Lock()
	s.clients[datacenter] = cli
	s.clientsLock.Unlock()
}

func (s *Stream) connect(datacenter string, query string) {
	_, addrs, err := net.LookupSRV("", "", query)
	if err != nil {
		log.Error(err)
		return
	}

	if len(addrs) == 0 {
		log.WithFields(log.Fields{"query": query}).Error("no servers")
		return
	}

	address := fmt.Sprintf("%s:%d", addrs[0].Target, addrs[0].Port)
	log.WithFields(log.Fields{"address": address}).Debug("using address")

	conn, err := client.DialWithTimeout("tcp", address, time.Duration(10)*time.Second)
	if err != nil {
		log.Error(err.Error())
		return
	}

	client := NewClient(conn, datacenter, s.Info)
	s.AddClient(datacenter, client)
	client.Handshake()
}

func (s *Stream) Connect() {
	for datacenter, query := range s.options.Queries {
		go s.connect(datacenter, query)
	}
}

func (s *Stream) Wait() {
	<-s.WaitChan
}
