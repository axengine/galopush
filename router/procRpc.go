package main

import (
	"encoding/json"
	"galopush/logs"
	"galopush/rpc"
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
				s := p.pool.findSessions(msg.Id)
				if s != nil {
					s.cometId = msg.CometId
					for _, v := range s.item {
						if v.plat == msg.Termtype {
							v.online = false
							//负载-1
							p.pool.cometSub(s.cometId)
							return
						}
					}
				}
			} else if msg.State == 1 {
				//online
				s := p.pool.findSessions(msg.Id)
				if s != nil {
					logs.Logger.Debug("StateNotify Find SESSION=", s)
					s.cometId = msg.CometId
					var find bool
					for _, v := range s.item {
						if v.plat == msg.Termtype {
							logs.Logger.Debug("StateNotify Find ITEM=", v)
							find = true
							v.online = true
						}
					}
					if !find {
						var it item
						it.online = true
						it.plat = msg.Termtype
						s.item = append(s.item, &it)
					}
					p.pool.cometAdd(s.cometId)
				} /* else {
					s = new(session)
					s.id = msg.Id
					s.cometId = msg.CometId
					var i item
					i.online = true
					i.plat = msg.Termtype
					s.item = append(s.item, i)

					p.pool.insertSessions(msg.Id, s)
					p.pool.cometAdd(s.cometId)
				}*/ //应该必须能找到session 因为要先通过业务层信息鉴权
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
			sess := p.pool.findSessions(msg.Id)
			if sess == nil {
				logs.Logger.Error("[rpc] Auth Failed Not Find session id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
				return code
			}
			for _, v := range sess.item {
				//业务层已登录 终端类型相同 鉴权码相同
				if v.plat == msg.Termtype && v.authCode == msg.Code && v.login == true {
					code = rpc.RPC_RET_SUCCESS
					logs.Logger.Debug("[rpc] Auth success id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
					return code
				}
			}
			logs.Logger.Debug("[rpc] Auth Failed Error Code or state id=", msg.Id, " sess=", sess.item)
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
			logs.Logger.Debug(b)
			if err := p.producer.Publish(p.topics[4], b); err != nil {
				logs.Logger.Error("MsgUpwardRequst Failed ", err)
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
