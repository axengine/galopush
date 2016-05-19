package rpc

import (
	"errors"
	"galopush/logs"
	"net/rpc"
	"sync"
	"time"
)

type RpcClient struct {
	serverAddr string
	mutex      sync.Mutex
	rpcHandle  *rpc.Client
}

//NewClient return a *Client wicth contant a rpc.handle
func NewRpcClient(serverAddr string) (error, *RpcClient) {
	var client RpcClient
	client.serverAddr = serverAddr

	var cli *rpc.Client
	var err error

	if cli, err = rpc.DialHTTP("tcp", serverAddr); err != nil {
		return err, &client
	}
	client.rpcHandle = cli
	return err, &client
}

func (p *RpcClient) connect() error {
	var err error
	var cli *rpc.Client
	cli, err = rpc.DialHTTP("tcp", p.serverAddr)
	if err == nil {
		p.rpcHandle = cli
	}
	return err
}

func (p *RpcClient) reConnect() error {
	//先关闭旧连接
	p.rpcHandle.Close()

	var err error
	if err = p.connect(); err == nil {
		logs.Logger.Debug("connect to rpc server ", p.serverAddr, "success")
		return nil
	}
	return err
}

/*
func (p *RpcClient) reConnectTimeout(timeout int) {
	//先关闭旧连接
	p.rpcHandle.Close()

	for {
		var err error
		if err = p.connect(); err == nil {
			logs.Logger.Debug("connect to rpc server ", p.serverAddr, "success")
			return
		}
		logs.Logger.Error("cannot connect to rpc server ", p.serverAddr, " err ", err, " reconnect after ", timeout, " second")
		select {
		case <-time.After(time.Duration(timeout) * time.Second):
		}
	}
}
*/
//StartPing start heartbeat with the ticker 30 s
func (p *RpcClient) StartPing(exit chan string, cometId string) {
	go func(client *RpcClient) {
		tk := time.NewTicker(time.Second * 30)
		for {
			select {
			case <-tk.C:
				err := client.Ping()
				if err != nil {
					logs.Logger.Error("[rpc]ping rpc server err ", err, " server addr  ", client.serverAddr)
					err := client.reConnect()
					if err != nil {
						logs.Logger.Error("[rpc]reconnect to rpc server err ", err, " addr  ", client.serverAddr, " exit ping runtime")
						exit <- cometId
						return
					}
				}
			}
		}
	}(p)
}

//Push 下发消息
func (p *RpcClient) Push(msgType int, id string, msg []byte) error {
	var request PushRequst
	request.Tp = msgType
	request.Id = id
	request.Msg = msg

	var respone Response

	err := p.rpcHandle.Call("RpcServer.Push", &request, &respone)
	if err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Push ", err)
		return err
	}
	if respone.Code != 0 {
		return errors.New("PUSH ERROR")
	}
	return nil
}

//Login 用户状态登记
func (p *RpcClient) State(id, cometId string, state, termtype int) error {
	logs.Logger.Debug("[rpcclient] report state id=", id, " cometId=", cometId, " plat=", termtype, " state=", state)
	var request StateRequst
	request.Id = id
	request.CometId = cometId
	request.Termtype = termtype
	request.State = state

	var respone Response

	if err := p.rpcHandle.Call("RpcServer.State", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.State ", err)
		return err
	}
	if respone.Code != 0 {
		return ERROR_RESPONSE
	}
	return nil
}

//Load comet登记
func (p *RpcClient) Register(cometId, tcpAddr, wsAddr, rpcAddr string) error {
	var request LoadRequst
	request.CometId = cometId
	request.TcpAddr = tcpAddr
	request.WsAddr = wsAddr
	request.RpcAddr = rpcAddr

	var respone Response

	if err := p.rpcHandle.Call("RpcServer.Register", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Register ", err)
		return err
	}
	if respone.Code != 0 {
		return ERROR_RESPONSE
	}
	return nil
}

//Ping heartbeat
func (p *RpcClient) Ping() error {
	var request PingRequest
	request.TimeStamp = int(time.Now().Unix())
	var respone Response

	if err := p.rpcHandle.Call("RpcServer.Ping", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Ping ", err)
		return err
	}
	if respone.Code != RPC_RET_SUCCESS {
		return ERROR_RESPONSE
	}
	return nil
}
