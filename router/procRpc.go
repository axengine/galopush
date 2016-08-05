package main

import (
	"encoding/json"
	"galopush/internal/logs"
	//	"galopush/internal/protocol"
	"galopush/internal/rpc"
)

//RpcAsyncHandle
/// RPC服务异步数据处理句柄
func (p *Router) RpcAsyncHandle(request interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	switch request.(type) {
	case *rpc.StateNotify:
		{
			msg := request.(*rpc.StateNotify)
			logs.Logger.Info("[rpc] StateNotify id=", msg.Id, " plat=", msg.Termtype, " state=", msg.State, " comet=", msg.CometId)
			if msg.State == 0 { //offline
				sess := p.store.FindSessions(msg.Id)
				if sess != nil {
					sess.CometId = msg.CometId
					for _, it := range sess.Sess {
						p.pool.cometSub(msg.CometId)
						if it.Plat == msg.Termtype && it.AuthCode == msg.Token {
							it.Online = false
							return
						}
					}
					p.store.SaveSessions(sess)
				}
			} else if msg.State == 1 {
				sess := p.store.FindSessions(msg.Id)
				if sess != nil {
					sess.CometId = msg.CometId
					p.pool.cometAdd(msg.CometId)
					for _, it := range sess.Sess {
						if it.Plat == msg.Termtype && it.AuthCode == msg.Token {
							it.Online = true
						}
					}
					p.store.SaveSessions(sess)
				}
			}
		}
	default:
		logs.Logger.Error("error RpcAsyncHandle quest type")
	}
}

//RpcSyncHandle
//RPC服务同步数据处理句柄
func (p *Router) RpcSyncHandle(request interface{}) int {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()

	code := rpc.RPC_RET_FAILED

	switch request.(type) {
	case *rpc.AuthRequest:
		{
			msg := request.(*rpc.AuthRequest)
			logs.Logger.Info("[rpc] Auth Receive id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
			{
				sess := p.store.FindSessions(msg.Id)
				if sess == nil {
					logs.Logger.Error("[rpc] Auth Failed Not Find session id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
					return code
				}
				for _, v := range sess.Sess {
					if v.Plat == msg.Termtype {
						if v.AuthCode == msg.Code && v.Login == true {
							code = rpc.RPC_RET_SUCCESS
							logs.Logger.Debug("[rpc] Auth success id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
							return code
						} else {
							logs.Logger.Debug("[rpc] Auth Failed Error Code or state id=", msg.Id, " item=", v)
						}
					}
				}
			}
		}
	case *rpc.CometRegister:
		{
			msg := request.(*rpc.CometRegister)
			logs.Logger.Info("[rpc] comet register comet=", msg.CometId, " rpc=", msg.RpcAddr, " tcp=", msg.TcpAddr, " ws=", msg.WsAddr)
			c := p.pool.findComet(msg.CometId)
			if c != nil {
				c.id = msg.CometId
				c.tcpAddr = msg.TcpAddr
				c.wsAddr = msg.WsAddr

				//先关闭原客户端连接
				c.rpcClient.Close()
				//再创建一个新连接
				client, err := p.NewRpcClient(c.id, msg.RpcAddr, c.ch)
				if err != nil {
					logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
					p.pool.deleteComet(msg.CometId)
					return code
				}
				c.rpcClient = client
				//替换原RPC CLIENT
				p.pool.insertComet(msg.CometId, c)
				p.checkRpc(c)
			} else {
				c = new(comet)
				c.id = msg.CometId
				c.tcpAddr = msg.TcpAddr
				c.wsAddr = msg.WsAddr
				c.ch = make(chan int, 1)

				client, err := p.NewRpcClient(c.id, msg.RpcAddr, c.ch)
				if err != nil {
					logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
					return code
				}
				c.rpcClient = client
				p.pool.insertComet(msg.CometId, c)
				p.checkRpc(c)
			}
			code = rpc.RPC_RET_SUCCESS
		}
	case *rpc.MsgUpwardRequst:
		{
			msg := request.(*rpc.MsgUpwardRequst)
			logs.Logger.Info("[rpc] upmsg id=", msg.Id, " plat=", msg.Termtype, " msg=", msg.Msg)
			var upMsg MsgUpward
			upMsg.Uid = msg.Id
			upMsg.Termtype = msg.Termtype
			upMsg.Body = msg.Msg
			b, _ := json.Marshal(&upMsg)

			//发送到NSQ
			//logs.Logger.Debug(b)
			if err := p.producer.Publish(p.upMsgTopic, b); err != nil {
				logs.Logger.Error("MsgUpwardRequst Failed ", err, " msg=", msg.Msg)
				return code
			}
			code = rpc.RPC_RET_SUCCESS
		}
	}
	return code
}

func (p *Router) checkRpc(c *comet) {
	go func(ch chan int) {
		for {
			select {
			case i := <-ch:
				switch i {
				case 0:
					{
						err := c.rpcClient.ReConnect()
						if err != nil {
							p.pool.deleteComet(c.id)
							logs.Logger.Critical("[rpc]reconnect to comet failed ", err)
							return
						}
					}
				case 1:
					//comet启动时注册到router
					{
						c.rpcClient.StartPing()
						logs.Logger.Debug("[rpc]conncet to comet success")
					}
				}
			}
		}
	}(c.ch)
}
