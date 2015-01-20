package main

import (
	log "github.com/Sirupsen/logrus"
	flags "github.com/jessevdk/go-flags"
	"github.com/rphillips/agent"
	"os"
	"runtime"
)

var defaultMonitoringSRVQueries = map[string]string{
	"dfw": "_monitoringagent._tcp.dfw1.prod.monitoring.api.rackspacecloud.com",
	"ord": "_monitoringagent._tcp.ord1.prod.monitoring.api.rackspacecloud.com",
	"lon": "_monitoringagent._tcp.lon3.prod.monitoring.api.rackspacecloud.com",
}

var opts struct {
	Token   string `long:"token" short:"t" description:"Token"`
	AgentId string `long:"agent-id" short:"a" description:"AgentId"`
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)

	if cpu := runtime.NumCPU(); cpu == 1 {
		runtime.GOMAXPROCS(2)
	} else {
		runtime.GOMAXPROCS(cpu)
	}
}

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "agent"
	parser.Usage = "[OPTIONS]"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	options := agent.StreamOptions{}
	options.Queries = defaultMonitoringSRVQueries

	info := agent.ClientInfo{}
	info.Version = "9.0.0-dev"
	info.Token = opts.Token
	info.AgentId = opts.AgentId
	info.AgentName = "rackspace-monitoring-go"

	stream := agent.NewStream(&options, &info)
	stream.Connect()
	stream.Wait()
}
