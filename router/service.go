package main

import (
	"fmt"
	"galopush/logs"
	"galopush/protocol"
	"galopush/rpc"
)

//rpcAsyncHandle RPC服务异步数据处理句柄
func (p *Router) rpcAsyncHandle(request interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	switch request.(type) {
	case *rpc.StateRequst:
		msg := request.(*rpc.StateRequst)
		logs.Logger.Debug("[rpc] state id=", msg.Id, " comet=", msg.CometId, " state=", msg.State, " plat=", msg.Termtype)
		if msg.State == 0 { //offline
			s := p.pool.findSessions(msg.Id)
			if s != nil {
				for i, v := range s.item {
					if v.plat == msg.Termtype {
						s.item = append(s.item[:i], s.item[i:]...)
						return
					}
				}
				//负载-1
				p.pool.cometSub(s.cometId)
			}
		} else if msg.State == 1 {
			//online
			s := p.pool.findSessions(msg.Id)
			if s != nil {
				var i item
				i.online = true
				i.plat = msg.Termtype
				s.item = append(s.item, i)
				p.pool.cometAdd(s.cometId)
			} else {
				s = new(session)
				s.id = msg.Id
				s.cometId = msg.CometId
				var i item
				i.online = true
				i.plat = msg.Termtype
				s.item = append(s.item, i)

				p.pool.insertSessions(msg.Id, s)
				p.pool.cometAdd(s.cometId)
			}
		}
	default:
		logs.Logger.Error("error rpcAsyncHandle quest type")
	}
}

//rpcSyncHandle RPC服务同步数据处理句柄
func (p *Router) rpcSyncHandle(request interface{}) int {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	var err error
	var code int
	switch request.(type) {
	case *rpc.LoadRequst:
		msg := request.(*rpc.LoadRequst)
		logs.Logger.Debug("[rpc] load comet=", msg.CometId, " rpc=", msg.RpcAddr, " tcp=", msg.TcpAddr, " ws=", msg.WsAddr)
		c := p.pool.findComet(msg.CometId)
		if c != nil {
			c.id = msg.CometId
			err, c.rpcClient = p.newRpcClient(msg.RpcAddr)
			if err != nil {
				code = 1
				logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
				p.pool.deleteComet(msg.CometId)
				return code
			}
			c.rpcClient.StartPing(p.cometExit, c.id)

			c.tcpAddr = msg.TcpAddr
			c.wsAddr = msg.WsAddr
		} else {
			c = new(comet)
			c.id = msg.CometId
			err, c.rpcClient = p.newRpcClient(msg.RpcAddr)
			if err != nil {
				code = 1
				logs.Logger.Error("[rpc] connect to comet ", msg.RpcAddr, " error ", err)
				return code
			}
			c.rpcClient.StartPing(p.cometExit, c.id)
			c.tcpAddr = msg.TcpAddr
			c.wsAddr = msg.WsAddr
			p.pool.insertComet(msg.CometId, c)
		}
	}
	return code
}

//balancer HTTP接口 根据id和plat返回socket地址
func (p *Router) balancer(id string, plat int) string {
	s := p.pool.findSessions(id)
	if s != nil {
		c := p.pool.findComet(s.cometId)
		if c != nil {
			if plat == protocol.PLAT_WEB {
				return c.wsAddr
			} else {
				return c.tcpAddr
			}
		}
	}
	//系统指配
	c := p.pool.balancer()
	if c != nil {
		if plat == 8 {
			return c.wsAddr
		} else {
			return c.tcpAddr
		}
	}
	return ""
}

//nsqHandle NSQ CHANNEL句柄
//根据不同的主题和数据进行分发
func (p *Router) nsqHandle(topic string, i interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	b := i.([]byte)
	logs.Logger.Debug("[nsq] topic=", topic, " msg=", string(b[:]))

	//待定义与后台交互的数据格式

	//取uid
	//根据uid找session.cometid
	//根据cometid找到rpcclient
	//调用rpclient

	//失败存离线
	//	c := rpc.RpcClient
	switch topic {
	case p.topics[0]: //push
		sess := p.pool.findSessions("testid")
		if sess != nil {
			comet := p.pool.findComet(sess.cometId)
			if comet != nil {
				err := comet.rpcClient.Push(protocol.MSGTYPE_PUSH, "testid", b)
				if err != nil {
					logs.Logger.Error(err)
					p.store.SavePushMsg("testid", b)
				}
			} else {
				//comet offline need save msg
			}
		}
	case p.topics[1]: //callback
		sess := p.pool.findSessions("testid")
		if sess != nil {
			comet := p.pool.findComet(sess.cometId)
			if comet != nil {
				err := comet.rpcClient.Push(protocol.MSGTYPE_CALLBACK, "testid", b)
				if err != nil {
					logs.Logger.Error(err)
					//offline.SaveCallbackMsg("testid", sess.item, b)
				}
			}
		}
	case p.topics[2]: //im
		sess := p.pool.findSessions("testid")
		if sess != nil {
			comet := p.pool.findComet(sess.cometId)
			if comet != nil {
				err := comet.rpcClient.Push(protocol.MSGTYPE_MESSAGE, "testid", b)
				if err != nil {
					logs.Logger.Error(err)
				}
			}
		}
	}
	fmt.Println(topic, "--", string(i.([]byte)))
}
