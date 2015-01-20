package client

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"io"
	"net"
	"sync"
	"time"
)

var ErrShutdown = errors.New("connection is shut down")

// Call represents an active RPC.
type Call struct {
	ServiceMethod string
	Args          interface{}
	Reply         *ClientMessage
	Error         error
	Done          chan *Call

	Seq    uint64
	Result interface{}
}

type Message struct {
	Version string `json:"v"`
	Id      uint64 `json:"id"`
	Target  string `json:"target"`
	Source  string `json:"source"`
}

type ClientMessage struct {
	Message

	Method string      `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`

	Result interface{} `json:"result,omitempty"`
	Error  interface{} `json:"error"`
}

type Client struct {
	Source string
	Target string

	Dec *json.Decoder
	Enc *json.Encoder

	C io.ReadWriteCloser

	Mutex    sync.Mutex // protects following
	Seq      uint64
	Closing  bool // user has called Close
	Shutdown bool // server has told us to stop

	RecvChan chan *ClientMessage

	Pending map[uint64]*Call
}

func (client *Client) sendResponse(call *Call) {
	client.Mutex.Lock()
	defer client.Mutex.Unlock()

	// Encode and send the request.
	var request ClientMessage
	request.Id = call.Seq
	if call.Result != nil {
		request.Result = call.Result
	} else {
		request.Params = call.Args
	}
	err := client.WriteMessage(&request)
	if err != nil {
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) send(call *Call) {
	client.Mutex.Lock()
	defer client.Mutex.Unlock()

	// Register this call.
	if client.Shutdown || client.Closing {
		call.Error = ErrShutdown
		call.done()
		return
	}
	seq := client.Seq
	client.Seq++
	client.Pending[seq] = call

	// Encode and send the request.
	var request ClientMessage
	request.Id = seq
	request.Method = call.ServiceMethod
	if call.Result != nil {
		request.Result = call.Result
	} else {
		request.Params = call.Args
	}
	err := client.WriteMessage(&request)
	if err != nil {
		call = client.Pending[seq]
		delete(client.Pending, seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) WriteMessage(r *ClientMessage) error {
	r.Version = "1"
	r.Source = client.Source
	r.Target = client.Target

	data, _ := json.Marshal(r)
	log.WithFields(log.Fields{
		"data": string(data),
	}).Debug("write message")

	return client.Enc.Encode(r)
}

func (client *Client) ReadMessage(r *ClientMessage) error {
	return client.Dec.Decode(r)
}

func (client *Client) Input() {
	var err error
	var msg *ClientMessage
	for err == nil {
		msg = &ClientMessage{}
		err = client.ReadMessage(msg)
		if err != nil {
			break
		}

		data, _ := json.Marshal(msg)
		log.WithFields(log.Fields{"data": string(data)}).Debug(string("received message"))

		seq := msg.Id
		client.Mutex.Lock()
		call := client.Pending[seq]
		delete(client.Pending, seq)
		client.Mutex.Unlock()

		switch {
		case call == nil:
			client.RecvChan <- msg
		default:
			call.Reply = msg
			call.done()
		}
	}
	// Terminate pending calls.
	client.Mutex.Lock()
	client.Shutdown = true
	closing := client.Closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range client.Pending {
		call.Error = err
		call.done()
	}
	client.Mutex.Unlock()
}

func (call *Call) done() {
	select {
	case call.Done <- call:
	default:
	}
}

func (client *Client) Close() error {
	client.Mutex.Lock()
	if client.Closing {
		client.Mutex.Unlock()
		return ErrShutdown
	}
	client.Closing = true
	client.Mutex.Unlock()
	return client.Close()
}

func (client *Client) Send(msg *ClientMessage, result interface{}) *Call {
	call := new(Call)
	call.Result = result
	call.Seq = msg.Id
	client.sendResponse(call)
	return call
}

func (client *Client) Go(serviceMethod string, args interface{}, reply *ClientMessage, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel.  If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("rpc: done channel is unbuffered")
		}
	}
	call.Done = done
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args interface{}, reply *ClientMessage) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	*reply = *call.Reply
	return call.Error
}

func DialWithTimeout(network, address string, timeout time.Duration) (*tls.Conn, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10}
	dialer := &net.Dialer{Timeout: timeout}
	return tls.DialWithDialer(dialer, network, address, tlsConfig)
}
