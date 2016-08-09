package main

import (
	"galopush/internal/counter"
	"galopush/internal/logs"
	"galopush/internal/rds"
	"galopush/internal/rpc"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/widuu/goini"
)

type Comet struct {
	//与router交互
	cometId       string //comet id 手工配置
	routerRpcAddr string //router RPC服务地址
	rpcCli        *rpc.RpcClient
	rpcStateChan  chan int //RPC链接状态通知通道

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
	store *rds.Storager

	//系统控制
	exit chan string
	wg   sync.WaitGroup
}

func (p *Comet) Init() {
	conf := goini.SetConfig("./config.ini")
	logs.Logger.Debug("--------OnInit... cometId----", p.cometId)
	//comet as rpc client
	{
		p.cometId = conf.GetValue("comet", "cometId")
		p.routerRpcAddr = conf.GetValue("router", "rpcAddr")
		p.rpcStateChan = make(chan int, 1)
		logs.Logger.Debug("----router rpc addr=", p.routerRpcAddr)
	}

	//comet as rpc server
	{
		p.cometRpcAddr = conf.GetValue("comet", "rpcAddr")
		s := conf.GetValue("comet", "rpcServerCache")
		p.maxRpcInFight, _ = strconv.Atoi(s)
		logs.Logger.Debug("----comet rpc addr=", p.cometRpcAddr, " cache=", p.maxRpcInFight)
	}

	//tcp&websocket server
	{
		p.uaTcpAddr = conf.GetValue("comet", "tcpAddr")
		p.uaWsAddr = conf.GetValue("comet", "wsAddr")
		logs.Logger.Debug("----tcp addr=", p.uaTcpAddr, " ws addr=", p.uaWsAddr, " cache=", p.maxUaInFight, " runtime=", p.runtime)
	}

	//ua数据缓存
	{
		s := conf.GetValue("comet", "socketServerCache")
		p.maxUaInFight, _ = strconv.Atoi(s)
		p.dataChan = make(chan *socketData, p.maxUaInFight)
		s = conf.GetValue("comet", "socketCacheRuntime")
		p.runtime, _ = strconv.Atoi(s)
		logs.Logger.Debug("----cache=", p.maxUaInFight, " runtime=", p.runtime)
	}

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
		p.store = rds.NewStorager(dbconn, password, database)
		logs.Logger.Debug("----redis addr=", dbconn, " password:", password, " database:", database)
	}

	//控制
	p.exit = make(chan string)
	//连接池
	p.pool = new(Pool)
	p.pool.ids = make(map[string]string)
	p.pool.sessions = make(map[string]*session)

	//统计类
	p.cnt = counter.NewCounter()

	logs.Logger.Debug("--------Init success----")
}

func (p *Comet) Start() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	//rpc server
	{
		logs.Logger.Debug("start rpc server listen on ", p.cometRpcAddr)
		p.rpcSrv = rpc.NewRpcServer(p.cometRpcAddr, p.maxRpcInFight, p.RpcSyncHandle, p.RpcAsyncHandle)
	}

	//rpc client
	{
		logs.Logger.Debug("start rpc client to router ", p.routerRpcAddr)
		client, err := rpc.NewRpcClient("", p.routerRpcAddr, p.rpcStateChan)
		if err != nil {
			logs.Logger.Error("Cann't connect to router ", err.Error())
			panic(err)
		}
		p.rpcCli = client
		p.checkRpc()
	}

	//tcp & ws server
	{
		logs.Logger.Debug("start tcp server listen on ", p.uaTcpAddr)
		p.startTcpServer()
		logs.Logger.Debug("start ws server listen on ", p.uaWsAddr)
		p.startWsServer()

		logs.Logger.Debug("start socket proc with ", p.runtime, " runtime")
		//开启socket数据处理runtine
		p.startSocketHandle()
	}

	//开启统计输出
	go p.stat()

	logs.Logger.Debug("start comet success")
}

func (p *Comet) Stop() error {
	debug.PrintStack()
	close(p.exit)

	return nil
}
func (p *Comet) checkRpc() {
	//RPC CLIENT STATE CHECK
	go func(ch chan int) {
		for {
			select {
			case i := <-ch:
				switch i {
				case 0:
					{
						err := p.rpcCli.ReConnect()
						if err != nil {
							logs.Logger.Critical("ReConnect to router failed ", err)
							return
						}
					}
				case 1:
					//comet启动时注册到router
					{
						p.rpcCli.StartPing()
						logs.Logger.Debug("register to router cometId=", p.cometId, " tcp=", p.uaTcpAddr, " ws=", p.uaWsAddr, " rpc=", p.cometRpcAddr)
						if err := p.rpcCli.Register(p.cometId, p.uaTcpAddr, p.uaWsAddr, p.cometRpcAddr); err != nil {
							logs.Logger.Critical("comet register to router error ", err)
						}
					}
				}
			}
		}
	}(p.rpcStateChan)
}
