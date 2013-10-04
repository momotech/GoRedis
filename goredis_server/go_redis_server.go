package goredis_server

import (
	. "../goredis"
	"./monitor"
	"./storage"
	"errors"
	"strings"
	"sync"
)

var (
	WrongKindError = errors.New("Wrong kind opration")
	WrongKindReply = ErrorReply(WrongKindError)
)

// GoRedisServer
type GoRedisServer struct {
	CommandHandler
	RedisServer
	// 数据源
	directory  string
	datasource storage.DataSource
	// counters
	cmdCounters  *monitor.Counters
	syncCounters *monitor.Counters
	// logger
	statusLogger *monitor.StatusLogger
	syncMonitor  *monitor.StatusLogger
	// 从库
	slaveMgr *SlaveServerManager
	// 从库状态
	ReplicationInfo ReplicationInfo
	// locks
	stringMutex sync.Mutex
}

func NewGoRedisServer(directory string) (server *GoRedisServer) {
	server = &GoRedisServer{}
	// set as itself
	server.SetHandler(server)
	// default datasource
	server.directory = directory
	var e1 error
	server.datasource, e1 = storage.NewLevelDBDataSource(server.directory + "/db0")
	if e1 != nil {
		panic(e1)
	}
	// server.datasource = storage.NewMemoryDataSource()
	// counter
	server.cmdCounters = monitor.NewCounters()
	server.syncCounters = monitor.NewCounters()
	// monitor
	server.initCommandMonitor(server.directory + "/cmd.log")
	server.initSyncMonitor(server.directory + "/sync.log")
	// slave
	server.slaveMgr = NewSlaveServerManager(server)
	server.ReplicationInfo = ReplicationInfo{}
	return
}

func (server *GoRedisServer) Listen(host string) {
	server.RedisServer.Listen(host)
}

// 命令执行监控
func (server *GoRedisServer) initCommandMonitor(path string) {
	// monitor
	server.statusLogger = monitor.NewStatusLogger(path)
	server.statusLogger.Add(monitor.NewTimeFormater("Time", 8))
	cmds := []string{"TOTAL", "GET", "SET", "HSET", "HGET", "HGETALL", "INCR", "DEL", "ZADD"}
	for _, cmd := range cmds {
		padding := len(cmd) + 1
		if padding < 7 {
			padding = 7
		}
		server.statusLogger.Add(monitor.NewCountFormater(server.cmdCounters.Get(cmd), cmd, padding))
	}
	server.statusLogger.Start()
}

// 从库同步监控
func (server *GoRedisServer) initSyncMonitor(path string) {
	server.syncMonitor = monitor.NewStatusLogger(path)
	server.syncMonitor.Add(monitor.NewTimeFormater("Time", 8))
	cmds := []string{"string", "hash", "set", "list", "zset", "total"}
	for _, cmd := range cmds {
		server.syncMonitor.Add(monitor.NewCountFormater(server.syncCounters.Get(cmd), cmd, 8))
	}
	server.syncMonitor.Start()
}

// for CommandHandler
func (server *GoRedisServer) On(cmd *Command, session *Session) {
	go func() {
		cmdName := strings.ToUpper(cmd.Name())
		server.cmdCounters.Get(cmdName).Incr(1)
		server.cmdCounters.Get("TOTAL").Incr(1)
	}()
}

func (server *GoRedisServer) OnUndefined(cmd *Command, session *Session) (reply *Reply) {
	return ErrorReply("Not Supported: " + cmd.String())
}
