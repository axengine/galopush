package main

import (
	"galopush/logs"
	"galopush/nsq"
	"galopush/redisstore"
	"galopush/rpc"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/widuu/goini"
)

var Debug = logs.Logger.Debug
var Error = logs.Logger.Error
var Critical = logs.Logger.Critical

type Router struct {
	//rpc服务
	routerRpcAddr string
	maxRpcInFight int
	rpcServer     *rpc.RpcServer

	//nsq服务
	topics        []string
	discover      *nsq.TopicDiscoverer
	nsqlookupAddr []string

	//负载路由
	httpBindAddr string
	cometExit    chan string //cometId exit channel
	pool         *Pool

	//离线消息
	store *redisstore.Storager

	//系统控制
	exit chan struct{}
	wg   sync.WaitGroup
}

func (p *Router) Init() {
	conf := goini.SetConfig("./config.ini")

	p.routerRpcAddr = conf.GetValue("router", "rpcAddr")
	s := conf.GetValue("router", "rpcServerCache")
	p.maxRpcInFight, _ = strconv.Atoi(s)

	//nsq costmer
	s = conf.GetValue("nsq", "nsqlookupAddr")
	p.nsqlookupAddr = strings.Split(s, ",")
	s = conf.GetValue("nsq", "topics")
	p.topics = strings.Split(s, ",")

	p.httpBindAddr = conf.GetValue("http", "bindAddr")
	p.cometExit = make(chan string)

	p.pool = new(Pool)
	p.pool.comets = make(map[string]*comet)
	p.pool.sessions = make(map[string]*session)

	//离线消息
	dbconn := conf.GetValue("redis", "conn")
	password := conf.GetValue("redis", "password")
	password = strings.TrimSpace(password)
	databaseS := conf.GetValue("redis", "database")
	database, err := strconv.Atoi(databaseS)
	if err != nil {
		database = 0
	}
	p.store = redisstore.NewStorager(dbconn, password, database)

	Debug("--------OnInit--------")
	Debug("----nsqd nsqlookup addr=", p.nsqlookupAddr, " topics=", p.topics)
	Debug("----router rpc addr=", p.routerRpcAddr, " cache=", p.maxRpcInFight)
	Debug("----http addr=", p.httpBindAddr, " cache=", p.maxRpcInFight)
	Debug("----redis addr=", dbconn, " password:", password, " database:", database)
	Debug("--------Init success--------")
}

func (p *Router) Start() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("Start.recover:", r)
			go p.Start()
		}
	}()
	p.rpcServer = rpc.NewRpcServer(p.routerRpcAddr, p.maxRpcInFight, p.rpcSyncHandle, p.rpcAsyncHandle)

	//处理comet异常中断 清除comet以及comet上注册的用户
	go func() {
		for {
			select {
			case id := <-p.cometExit:
				p.pool.deleteComet(id)
				p.pool.deleteSessionsWithCometId(id)
			}
		}
	}()

	p.discover = nsq.NewTopicDiscoverer(p.topics, p.maxRpcInFight, p.nsqlookupAddr, p.nsqHandle)

	p.startHttpServer()

	//p.wg.Add(1)
	Debug("start router success")
}

func (p *Router) Stop() error {
	debug.PrintStack()
	close(p.exit)
	//p.wg.Wait()
	return nil
}

//newRpcClient 返回一个RPC客户端
func (p *Router) newRpcClient(addr string) (error, *rpc.RpcClient) {
	err, c := rpc.NewRpcClient(addr)
	if err != nil {
		logs.Logger.Error(err)
		return err, c
	}
	//c.StartPing()
	return err, c
}
