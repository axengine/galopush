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
			logs.Logger.Debug("[rpc] StateNotify id=", msg.Id, " plat=", msg.Termtype, " state=", msg.State, " comet=", msg.CometId)
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
					s.cometId = msg.CometId
					var find bool
					for _, v := range s.item {
						if v.plat == msg.Termtype {
							find = true
							v.online = true
						}
					}
					if !find {
						var i item
						i.online = true
						i.plat = msg.Termtype
						s.item = append(s.item, i)
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
			logs.Logger.Debug("[rpc] Auth id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
			sess := p.pool.findSessions(msg.Id)
			if sess == nil {
				logs.Logger.Info("[rpc] Auth Failed id=", msg.Id, " termtype=", msg.Termtype, " code=", msg.Code)
				return code
			}
			for _, v := range sess.item {
				//业务层已登录 终端类型相同 鉴权码相同
				if v.plat == msg.Termtype && v.authCode == msg.Code && v.login == true {
					code = rpc.RPC_RET_SUCCESS
					return code
				}
			}
		}
	case *rpc.CometRegister:
		{
			msg := request.(*rpc.CometRegister)
			logs.Logger.Debug("[rpc] comet register comet=", msg.CometId, " rpc=", msg.RpcAddr, " tcp=", msg.TcpAddr, " ws=", msg.WsAddr)
			c := p.pool.findComet(msg.CometId)
			if c != nil {
				c.id = msg.CometId
				//先关闭原客户端连接
				c.rpcClient.Close()
				//再创建一个新连接
				client, err := p.NewRpcClient(msg.RpcAddr)
				if err != nil {
					logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
					p.pool.deleteComet(msg.CometId)
					return code
				}
				c.rpcClient = client
				c.rpcClient.StartPing(p.cometExit, c.id)

				c.tcpAddr = msg.TcpAddr
				c.wsAddr = msg.WsAddr
			} else {
				c = new(comet)
				c.id = msg.CometId
				c.tcpAddr = msg.TcpAddr
				c.wsAddr = msg.WsAddr

				client, err := p.NewRpcClient(msg.RpcAddr)
				if err != nil {
					logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
					return code
				}
				c.rpcClient = client
				c.rpcClient.StartPing(p.cometExit, c.id)
				p.pool.insertComet(msg.CometId, c)
			}
			code = rpc.RPC_RET_SUCCESS
		}
	case *rpc.MsgUpwardRequst:
		{
			msg := request.(*rpc.MsgUpwardRequst)
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
