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
	rpcClient  *rpc.Client
}

//NewClient return a *Client wicth contant a rpc.handle
func NewRpcClient(serverAddr string) (*RpcClient, error) {
	var client RpcClient
	client.serverAddr = serverAddr

	var cli *rpc.Client
	var err error

	if cli, err = rpc.DialHTTP("tcp", serverAddr); err != nil {
		return &client, err
	}
	client.rpcClient = cli
	return &client, err
}

func (p *RpcClient) connect() error {
	var err error
	var cli *rpc.Client
	cli, err = rpc.DialHTTP("tcp", p.serverAddr)
	if err == nil {
		p.rpcClient = cli
	}
	return err
}

func (p *RpcClient) reConnect() error {
	//先关闭旧连接
	p.rpcClient.Close()

	var err error
	if err = p.connect(); err == nil {
		logs.Logger.Debug("connect to rpc server ", p.serverAddr, "success")
		return nil
	}
	return err
}

func (p *RpcClient) Close() error {
	return p.rpcClient.Close()
}

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

//Register comet登记
func (p *RpcClient) Register(cometId, tcpAddr, wsAddr, rpcAddr string) error {
	var request CometRegister
	request.CometId = cometId
	request.TcpAddr = tcpAddr
	request.WsAddr = wsAddr
	request.RpcAddr = rpcAddr

	var respone Response

	if err := p.rpcClient.Call("RpcServer.Register", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Register ", err)
		return err
	}
	if respone.Code != 0 {
		return ERROR_RESPONSE
	}
	return nil
}

//鉴权
func (p *RpcClient) Auth(id string, termtype int, code string) error {
	var request AuthRequest
	request.Id = id
	request.Termtype = termtype
	request.Code = code
	var respone Response

	if err := p.rpcClient.Call("RpcServer.Auth", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Auth ", err)
		return err
	}
	if respone.Code != 0 {
		return ERROR_RESPONSE
	}
	return nil
}

//Notify 用户状态登记
func (p *RpcClient) Notify(id, cometId string, state, termtype int) error {
	logs.Logger.Debug("[rpcclient] report state id=", id, " cometId=", cometId, " plat=", termtype, " state=", state)
	var request StateNotify
	request.Id = id
	request.CometId = cometId
	request.Termtype = termtype
	request.State = state

	var respone Response

	if err := p.rpcClient.Call("RpcServer.Notify", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Notify ", err)
		return err
	}
	return nil
}

//MsgUpward 消息上行
func (p *RpcClient) MsgUpward(id string, termtype int, msg string) error {
	var request MsgUpwardRequst
	request.Id = id
	request.Termtype = termtype
	request.Msg = msg

	var respone Response

	if err := p.rpcClient.Call("RpcServer.MsgUpward", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.MsgUpward ", err)
		return err
	}
	if respone.Code != 0 {
		return ERROR_RESPONSE
	}
	return nil
}

//Kick 踢人下线
func (p *RpcClient) Kick(id string, termtype int) error {
	var request KickRequst
	request.Id = id
	request.Termtype = termtype

	var respone Response

	err := p.rpcClient.Call("RpcServer.Kick", &request, &respone)
	if err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Kick ", err)
		return err
	}
	return nil
}

//Push 下发消息
func (p *RpcClient) Push(msgType int, id string, termtype int, msg string) error {
	var request PushRequst
	request.Tp = msgType
	request.Id = id
	request.Msg = msg

	var respone Response

	err := p.rpcClient.Call("RpcServer.Push", &request, &respone)
	if err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Push ", err)
		return err
	}
	if respone.Code != 0 {
		return errors.New("PUSH ERROR")
	}
	return nil
}

//Ping heartbeat
func (p *RpcClient) Ping() error {
	var request Ping
	var respone Response

	if err := p.rpcClient.Call("RpcServer.Ping", &request, &respone); err != nil {
		logs.Logger.Error("[rpc client] RpcServer.Ping ", err)
		return err
	}
	if respone.Code != RPC_RET_SUCCESS {
		return ERROR_RESPONSE
	}
	return nil
}
