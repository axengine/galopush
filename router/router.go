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
	nsqdTcpAddr   string
	producer      *nsq.Producer //nsq生产者

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
	Debug("--------OnInit--------")
	//RPC
	{
		p.routerRpcAddr = conf.GetValue("router", "rpcAddr")
		s := conf.GetValue("router", "rpcServerCache")
		p.maxRpcInFight, _ = strconv.Atoi(s)
		Debug("----router rpc addr=", p.routerRpcAddr, " cache=", p.maxRpcInFight)
	}

	//NSQ
	{
		s := conf.GetValue("nsq", "nsqlookupAddr")
		p.nsqlookupAddr = strings.Split(s, ",")
		s = conf.GetValue("nsq", "topics")
		p.topics = strings.Split(s, ",")
		p.nsqdTcpAddr = conf.GetValue("nsq", "tcpAddr")
		Debug("----nsqd nsqlookup addr=", p.nsqlookupAddr, " topics=", p.topics)
	}

	//HTTP
	{
		p.httpBindAddr = conf.GetValue("http", "bindAddr")
		Debug("----http addr=", p.httpBindAddr, " cache=", p.maxRpcInFight)
	}

	p.cometExit = make(chan string)

	p.pool = new(Pool)
	p.pool.comets = make(map[string]*comet)
	p.pool.sessions = make(map[string]*session)

	//REDIS
	{
		dbconn := conf.GetValue("redis", "conn")
		password := conf.GetValue("redis", "password")
		password = strings.TrimSpace(password)
		databaseS := conf.GetValue("redis", "database")
		database, err := strconv.Atoi(databaseS)
		if err != nil {
			database = 0
		}
		p.store = redisstore.NewStorager(dbconn, password, database)
		Debug("----redis addr=", dbconn, " password:", password, " database:", database)
	}

	Debug("--------Init success--------")
}

func (p *Router) Start() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("Start.recover:", r)
			go p.Start()
		}
	}()
	p.rpcServer = rpc.NewRpcServer(p.routerRpcAddr, p.maxRpcInFight, p.RpcSyncHandle, p.RpcAsyncHandle)

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

	p.discover = nsq.NewTopicDiscoverer(p.topics, p.maxRpcInFight, p.nsqlookupAddr, p.NsqHandler)
	producer, err := nsq.NewProducer(p.nsqdTcpAddr)
	if err != nil {
		panic(err)
	}
	p.producer = producer

	p.startHttpServer()

	Debug("--------Start Router success--------")
}

func (p *Router) Stop() error {
	debug.PrintStack()
	close(p.exit)
	return nil
}

//newRpcClient 返回一个RPC客户端
func (p *Router) NewRpcClient(addr string) (*rpc.RpcClient, error) {
	c, err := rpc.NewRpcClient(addr)
	if err != nil {
		logs.Logger.Error("NewRpcClient ", err)
		return c, err
	}
	return c, err
}