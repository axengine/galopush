package main

import (
	"fmt"
	"galopush/logs"
	"galopush/protocol"
	"galopush/rpc"
)

//RPC 异步句柄
func (p *Comet) RpcAsyncHandle(request interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	switch request.(type) {
	case *rpc.PushRequst:
		msg := request.(*rpc.PushRequst)
		switch msg.Tp {
		case protocol.MSGTYPE_PUSH:
			p.push(msg)
		case protocol.MSGTYPE_CALLBACK:
			p.callback(msg)
		case protocol.MSGTYPE_MESSAGE:
			p.message(msg)
		}
	case *rpc.KickRequst:
		msg := request.(*rpc.KickRequst)
		p.kick(msg)
	}
}

//RPC 同步句柄
func (p *Comet) RpcSyncHandle(request interface{}) int {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	switch request.(type) {
	case *rpc.KickRequst:
		{
			msg := request.(*rpc.KickRequst)
			logs.Logger.Debug("Kick id=", msg.Id, " plat=", msg.Termtype)

			ids := fmt.Sprintf("%s-%d", msg.Id, msg.Termtype)
			s := p.pool.findSessions(ids)
			if s != nil {
				p.rpcCli.Notify(s.id, p.cometId, rpc.STATE_OFFLINE, s.plat)
				s.destroy()
				p.pool.deleteSessions(ids)
				p.pool.deleteIds(connString(s.conn))
				p.closeConn(s.conn)
			}
			return 0
		}
	}
	return -1
}

func (p *Comet) push(msg *rpc.PushRequst) {
	id := msg.Id
	plat := msg.Termtype
	ptlType := protocol.PROTOCOL_TYPE_BINARY
	if plat == protocol.PLAT_WEB {
		ptlType = protocol.PROTOCOL_TYPE_JSON
	}
	logs.Logger.Debug("([>>>PUSH]  id=", msg.Id, " plat=", plat, " msg=", msg.Msg)

	//判断web是否在线
	iWebOnline := 0
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	if sWeb := p.pool.findSessions(idWeb); sWeb != nil {
		iWebOnline = 1
	}

	ids := fmt.Sprintf("%s-%d", id, plat)
	if sess := p.pool.findSessions(ids); sess != nil {
		var sendMsg protocol.Push
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_PUSH)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg) + 3)
		sendMsg.Msg = append(sendMsg.Msg, []byte(msg.Msg)...)
		sendMsg.Offline = 0
		sendMsg.Flag = uint8(iWebOnline)

		buf := protocol.Pack(&sendMsg, ptlType)
		logs.Logger.Debug("([PUSH]  to id=", id, " plat=", plat, " Tid=", sendMsg.Tid, " offline=", 0, " flag=", iWebOnline)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("PUSH write error:", err)
			if plat == protocol.PLAT_ANDROID {
				p.store.SavePushMsg(id, []byte(msg.Msg))
			} else if plat == protocol.PLAT_IOS {
				//APNS
				apnsPush(sess.appleToken, msg.Msg, "")
			}
		} else {
			//创建事务并保存
			trans := newTransaction()
			trans.tid = int(sendMsg.Tid)
			trans.msgType = protocol.MSGTYPE_PUSH
			trans.msg = append(trans.msg, msg.Msg...) //mem leak
			sess.saveTransaction(trans)
			sess.checkTrans(trans)
		}
	} else {
		//这里没有session 找不到ios的设备ID 直接存离线
		if plat != protocol.PLAT_WEB {
			p.store.SavePushMsg(id, []byte(msg.Msg))
		}
	}
}

func (p *Comet) callback(msg *rpc.PushRequst) {
	id := msg.Id
	plat := msg.Termtype
	ptlType := protocol.PROTOCOL_TYPE_BINARY
	if plat == protocol.PLAT_WEB {
		ptlType = protocol.PROTOCOL_TYPE_JSON
	}
	logs.Logger.Debug("([>>>CALLBACK]  id=", msg.Id, " plat=", plat, " msg=", msg.Msg)

	ids := fmt.Sprintf("%s-%d", id, plat)
	if sess := p.pool.findSessions(ids); sess != nil {
		var sendMsg protocol.Callback
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_CALLBACK)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg))
		sendMsg.Msg = append(sendMsg.Msg, []byte(msg.Msg)...)

		buf := protocol.Pack(&sendMsg, ptlType)
		logs.Logger.Debug("([CALLBACK]  to id=", id, " plat=", plat, " Tid=", sendMsg.Tid)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("CALLBACK write error:", err)
			if plat != protocol.PLAT_WEB {
				p.store.SaveCallbackMsg(id, plat, []byte(msg.Msg))
			}
		} else {
			//创建事务并保存
			trans := newTransaction()
			trans.tid = int(sendMsg.Tid)
			trans.msgType = protocol.MSGTYPE_CALLBACK
			trans.msg = append(trans.msg, []byte(msg.Msg)...) //mem leak
			sess.saveTransaction(trans)
			sess.checkTrans(trans)
		}
	} else {
		if plat != protocol.PLAT_WEB {
			p.store.SaveCallbackMsg(id, plat, []byte(msg.Msg))
		}
	}
}

func (p *Comet) message(msg *rpc.PushRequst) {
	id := msg.Id
	plat := msg.Termtype
	ptlType := protocol.PROTOCOL_TYPE_BINARY
	if plat == protocol.PLAT_WEB {
		ptlType = protocol.PROTOCOL_TYPE_JSON
	}
	logs.Logger.Debug("([>>>MESSAGE]  id=", msg.Id, " plat=", plat, " msg=", msg.Msg)

	//判断web是否在线
	iWebOnline := 0
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	if sWeb := p.pool.findSessions(idWeb); sWeb != nil {
		iWebOnline = 1
	}

	ids := fmt.Sprintf("%s-%d", id, plat)
	if sess := p.pool.findSessions(ids); sess != nil {
		var sendMsg protocol.ImDown
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_MESSAGE)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg) + 1)
		sendMsg.Flag = uint8(iWebOnline)
		sendMsg.Msg = append(sendMsg.Msg, []byte(msg.Msg)...)

		buf := protocol.Pack(&sendMsg, ptlType)
		logs.Logger.Debug("([MESSAGE]  to id=", id, " plat=", plat, " Tid=", sendMsg.Tid, " flag=", iWebOnline)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("MESSAGE write error:", err)
			if plat != protocol.PLAT_WEB {
				p.store.SaveImMsg(id, plat, []byte(msg.Msg))
			}
		} else {
			//创建事务并保存
			trans := newTransaction()
			trans.tid = int(sendMsg.Tid)
			trans.msgType = protocol.MSGTYPE_MESSAGE
			trans.msg = append(trans.msg, []byte(msg.Msg)...) //mem leak
			sess.saveTransaction(trans)
			sess.checkTrans(trans)
		}
	} else {
		if plat != protocol.PLAT_WEB {
			p.store.SaveImMsg(id, plat, []byte(msg.Msg))
		}
	}
}

func (p *Comet) kick(msg *rpc.KickRequst) {
	id := msg.Id
	plat := msg.Termtype
	ptlType := protocol.PROTOCOL_TYPE_BINARY
	if plat == protocol.PLAT_WEB {
		ptlType = protocol.PROTOCOL_TYPE_JSON
	}
	logs.Logger.Debug("([>>>KICK]  id=", msg.Id, " plat=", plat, " reason=", msg.Reason)

	ids := fmt.Sprintf("%s-%d", id, plat)
	if sess := p.pool.findSessions(ids); sess != nil {
		var sendMsg protocol.Kick
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_KICK)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = 1
		sendMsg.Reason = uint8(msg.Reason)

		buf := protocol.Pack(&sendMsg, ptlType)
		logs.Logger.Debug("([KICK]  to id=", id, " plat=", plat, " Tid=", sendMsg.Tid, " reason=", sendMsg.Reason)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("KICK write error:", err)
		}

		//上报router
		p.rpcCli.Notify(sess.id, p.cometId, rpc.STATE_OFFLINE, sess.plat)
		//清除session
		sess.destroy()
		p.pool.deleteSessions(ids)
		p.pool.deleteIds(connString(sess.conn))
		//关闭连接
		p.closeConn(sess.conn)
	}
}
