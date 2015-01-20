package agent

import (
	"encoding/json"
	"io"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/rphillips/agent/client"
)

type HandlerFunc func(client *MonitoringClient, msg *client.ClientMessage)

type MonitoringClient struct {
	*client.Client

	Datacenter string
	Info       *ClientInfo
	handlers   map[string]HandlerFunc
}

type ClientInfo struct {
	AgentId   string
	AgentName string
	Token     string
	Version   string
	Source    string
}

func (c *MonitoringClient) handleMessage(msg *client.ClientMessage) {
	handler, ok := c.handlers[msg.Method]
	if ok {
		handler(c, msg)
	} else {
		log.WithFields(log.Fields{"msg": msg}).Debug("unhandled message")
	}
}

func (c *MonitoringClient) beginAsyncRead() {
	for {
		select {
		case msg := <-c.RecvChan:
			c.handleMessage(msg)
		}
	}
}

func (c *MonitoringClient) start() {
	go c.beginAsyncRead()
	go c.Input()
}

func (c *MonitoringClient) StartHeartbeat(interval uint64) {
	for {
		req := struct {
			Timestamp int64 `json:"timestamp"`
		}{
			Timestamp: time.Now().UnixNano() / 100000,
		}
		time.Sleep(time.Duration(interval) * time.Millisecond)
		log.Debug("heartbeat")
		var res client.ClientMessage
		c.Go("heartbeat.post", req, &res, nil)
	}
}

func (c *MonitoringClient) Handshake() error {
	params := make(map[string]interface{})
	params["token"] = c.Info.Token
	params["agent_id"] = c.Info.AgentId
	params["agent_name"] = c.Info.AgentName
	params["process_version"] = c.Info.Version
	params["bundle_version"] = c.Info.Version

	var resp client.ClientMessage
	err := c.Call("handshake.hello", params, &resp)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{"response": resp}).Debug("received handshake response")

	go c.StartHeartbeat(40000)

	return err
}

func NewClient(conn io.ReadWriteCloser, datacenter string, info *ClientInfo) *MonitoringClient {
	cli := &MonitoringClient{
		&client.Client{
			RecvChan: make(chan *client.ClientMessage),
			Target:   "endpoint",
			Source:   info.Source,
			Dec:      json.NewDecoder(conn),
			Enc:      json.NewEncoder(conn),
			C:        conn,
			Seq:      0,
			Pending:  make(map[uint64]*client.Call),
		},
		datacenter,
		info,
		make(map[string]HandlerFunc),
	}
	cli.handlers["host_info.get"] = hostInfoGet
	cli.start()
	return cli
}
