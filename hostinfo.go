package agent

import (
	log "github.com/Sirupsen/logrus"
	"github.com/cloudfoundry/gosigar"
	"github.com/rphillips/agent/client"
	"time"
)

func hostinfoMemory(cli *MonitoringClient, msg *client.ClientMessage) {
	metrics := make(map[string]interface{})
	params := make(map[string]interface{})

	mem := sigar.Mem{}
	swap := sigar.Swap{}

	mem.Get()
	swap.Get()

	metrics["actual_free"] = mem.ActualFree
	metrics["actual_used"] = mem.ActualUsed
	metrics["total"] = mem.Total
	metrics["used"] = mem.Used
	metrics["free"] = mem.Free
	metrics["swap_total"] = swap.Total
	metrics["swap_used"] = swap.Used
	metrics["swap_free"] = swap.Free
	metrics["used_percent"] = float64(mem.Used) / float64(mem.Total) * 100
	metrics["free_percent"] = float64(mem.Free) / float64(mem.Total) * 100

	params["metrics"] = metrics
	params["timestamp"] = time.Now().UnixNano() / 100000
	cli.Send(msg, params)
}

func hostinfoCpu(cli *MonitoringClient, msg *client.ClientMessage) {
	cpus := sigar.CpuList{}
	cpus.Get()

	metrics := make([]map[string]interface{}, 0, 1)
	params := make(map[string]interface{})

	for _, cpu := range cpus.List {
		obj := make(map[string]interface{})
		obj["user"] = cpu.User
		obj["nice"] = cpu.Nice
		obj["sys"] = cpu.Sys
		obj["idle"] = cpu.Idle
		metrics = append(metrics, obj)
	}

	params["metrics"] = metrics
	params["timestamp"] = time.Now().UnixNano() / 100000
	cli.Send(msg, params)
}

func hostInfoNil(cli *MonitoringClient, msg *client.ClientMessage) {
	params := make(map[string]interface{})
	cli.Send(msg, params)
}

func hostinfoFilesystem(cli *MonitoringClient, msg *client.ClientMessage) {
	filesystems := sigar.FileSystemList{}
	filesystems.Get()

	metrics := make([]map[string]interface{}, 0, 1)
	params := make(map[string]interface{})

	for _, fs := range filesystems.List {
		obj := make(map[string]interface{})
		obj["dir_name"] = fs.DirName
		obj["dev_name"] = fs.DevName
		obj["sys_type_name"] = fs.SysTypeName

		usage := sigar.FileSystemUsage{}
		usage.Get(fs.DirName)

		obj["total"] = usage.Total
		obj["free"] = usage.Free
		obj["used"] = usage.Used
		obj["avail"] = usage.Avail
		obj["files"] = usage.Files
		obj["free_files"] = usage.FreeFiles

		metrics = append(metrics, obj)
	}

	params["metrics"] = metrics
	params["timestamp"] = time.Now().UnixNano() / 100000
	cli.Send(msg, params)
}

func hostinfoProcs(cli *MonitoringClient, msg *client.ClientMessage) {
	pids := sigar.ProcList{}
	pids.Get()

	metrics := make([]map[string]interface{}, 0, 1)
	params := make(map[string]interface{})

	for _, pid := range pids.List {
		obj := make(map[string]interface{})
		obj["pid"] = pid

		obj["exe_name"] = ""
		obj["exe_cwd"] = ""
		obj["exe_root"] = ""

		state := sigar.ProcState{}
		state.Get(pid)
		obj["state_name"] = state.Name
		obj["state_ppid"] = state.Ppid
		obj["state_priority"] = state.Priority
		obj["state_nice"] = state.Nice

		mem := sigar.ProcMem{}
		mem.Get(pid)
		obj["memory_size"] = mem.Size
		obj["memory_resident"] = mem.Resident
		obj["memory_page_faults"] = mem.PageFaults

		time := sigar.ProcTime{}
		time.Get(pid)
		obj["time_start_time"] = time.StartTime
		obj["time_user"] = time.User
		obj["time_sys"] = time.Sys
		obj["time_total"] = time.Total

		metrics = append(metrics, obj)
	}

	params["metrics"] = metrics
	params["timestamp"] = time.Now().UnixNano() / 100000
	cli.Send(msg, params)
}

func hostInfoGet(cli *MonitoringClient, msg *client.ClientMessage) {
	hostInfoMap := map[string]HandlerFunc{
		"MEMORY":     hostinfoMemory,
		"CPU":        hostinfoCpu,
		"FILESYSTEM": hostinfoFilesystem,
		"PROCS":      hostinfoProcs,
	}

	var params map[string]interface{}
	params = msg.Params.(map[string]interface{})
	typ := params["type"].(string)

	log.WithFields(log.Fields{"type": typ}).Debug("host_info.get")

	fn, ok := hostInfoMap[typ]
	if ok {
		fn(cli, msg)
	} else {
		hostInfoNil(cli, msg)
		log.WithFields(log.Fields{"type": params["type"]}).Error("host info not implemented")
	}
}
