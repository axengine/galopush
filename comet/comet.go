package main

import (
	"galopush/counter"
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

type Comet struct {
	//与nsq交互
	nsqdTcpAddr string        //nsqd 地址
	producer    *nsq.Producer //nsq生产者
	topic       string        //im上行生产者主题

	//与router交互
	cometId       string //comet id 手工配置
	routerRpcAddr string //router RPC服务地址
	rpcCli        *rpc.RpcClient

	cometRpcAddr  string //comet RPC服务地址
	rpcSrv        *rpc.RpcServer
	maxRpcInFight int //数据缓冲大小

	//与ua交互
	uaTcpAddr string //tcp 服务地址
	uaWsAddr  string //websocket服务地址

	dataChan     chan *socketData //数据缓冲通道
	maxUaInFight int              //数据缓冲大小
	runtime      int              //处理socket数据的携程数

	//业务控制
	pool *Pool            //session池
	cnt  *counter.Counter //计数器

	//离线消息
	//offStore
	store *redisstore.Storager

	//系统控制
	exit chan string
	wg   sync.WaitGroup
}

func (p *Comet) Init() {
	conf := goini.SetConfig("./config.ini")

	//[nsq]
	p.nsqdTcpAddr = conf.GetValue("nsq", "tcpAddr")
	p.topic = conf.GetValue("nsq", "topic")

	//comet as rpc client
	p.cometId = conf.GetValue("comet", "cometId")
	p.routerRpcAddr = conf.GetValue("router", "rpcAddr")

	//comet as rpc server
	p.cometRpcAddr = conf.GetValue("comet", "rpcAddr")
	s := conf.GetValue("comet", "rpcServerCache")
	p.maxRpcInFight, _ = strconv.Atoi(s)

	//tcp&websocket server
	p.uaTcpAddr = conf.GetValue("comet", "tcpAddr")
	p.uaWsAddr = conf.GetValue("comet", "wsAddr")

	//ua数据缓存
	s = conf.GetValue("comet", "socketServerCache")
	p.maxUaInFight, _ = strconv.Atoi(s)
	p.dataChan = make(chan *socketData, p.maxUaInFight)
	s = conf.GetValue("comet", "socketCacheRuntime")
	p.runtime, _ = strconv.Atoi(s)

	//连接池
	p.pool = new(Pool)
	p.pool.ids = make(map[string]string)
	p.pool.sessions = make(map[string]*session)

	//统计类
	p.cnt = counter.NewCounter()

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

	//控制
	p.exit = make(chan string)

	Debug("--------OnInit... cometId----", p.cometId)
	Debug("----nsqd tcp addr=", p.nsqdTcpAddr)
	Debug("----router rpc addr=", p.routerRpcAddr)
	Debug("----comet rpc addr=", p.cometRpcAddr, " cache=", p.maxRpcInFight)
	Debug("----tcp addr=", p.uaTcpAddr, " ws addr=", p.uaWsAddr, " cache=", p.maxUaInFight, " runtime=", p.runtime)
	Debug("----redis addr=", dbconn, " password:", password, " database:", database)
	Debug("--------Init success----")
}

func (p *Comet) Start() {
	//nsq prodecer
	Debug("start nsq producer to nsqd ", p.nsqdTcpAddr)
	producer, err := nsq.NewProducer(p.nsqdTcpAddr)
	if err != nil {
		panic(err)
	}
	p.producer = producer

	//rpc server
	Debug("start rpc server listen on ", p.cometRpcAddr)
	p.rpcSrv = rpc.NewRpcServer(p.cometRpcAddr, p.maxRpcInFight, p.rpcSyncHandle, p.rpcAsyncHandle)

	//rpc client
	Debug("start rpc client to router ", p.routerRpcAddr)
	err, p.rpcCli = rpc.NewRpcClient(p.routerRpcAddr)
	if err != nil {
		logs.Logger.Critical("Cann't connect to router ", err.Error())
		panic(err)
	}
	p.rpcCli.StartPing(p.exit, "")

	//tcp server
	Debug("start tcp server listen on ", p.uaTcpAddr)
	p.startTcpServer()

	//websocket server
	Debug("start ws server listen on ", p.uaWsAddr)
	p.startWsServer()

	//开启socket数据处理runtine
	Debug("start socket proc with ", p.runtime, " runtime")
	p.startSocketHandle()

	//comet启动时注册到router
	Debug("register to router cometId=", p.cometId, " tcp=", p.uaTcpAddr, " ws=", p.uaWsAddr, " rpc=", p.cometRpcAddr)
	if err := p.rpcCli.Register(p.cometId, p.uaTcpAddr, p.uaWsAddr, p.cometRpcAddr); err != nil {
		logs.Logger.Error("comet register to router error ", err)
		panic(err)
	}

	//开启统计输出
	go p.stat()
	//p.wg.Add(1)
	Debug("start comet success")
}

func (p *Comet) Stop() error {
	debug.PrintStack()
	close(p.exit)
	//p.wg.Wait()
	return nil
}
