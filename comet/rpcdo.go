package main

import (
	"fmt"
	"galopush/logs"
	"galopush/protocol"
	"galopush/rpc"
)

//RPC SERVER HANDLE
func (p *Comet) rpcAsyncHandle(request interface{}) {
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
	}
}

func (p *Comet) rpcSyncHandle(request interface{}) int {
	return 0
}

//push从nsq中接收消息并推送到客户端
//推送规则：
//1、web在线则推送给web
//2、android在线推android，不在线存离线消息
//3、iOS在线推iOS，不在线推送apns
//4、如果不是android或者ios用户，对应逻辑不处理
func (p *Comet) push(msg *rpc.PushRequst) {
	id := msg.Id
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	idAndroid := fmt.Sprintf("%s-%d", id, protocol.PLAT_ANDROID)
	idIos := fmt.Sprintf("%s-%d", id, protocol.PLAT_IOS)
	logs.Logger.Debug("([>>>PUSH]  id=", msg.Id, " msg=", string(msg.Msg[:]))

	var (
		bWebPushed = false
	)

	//按照推送规则
	//1:首先推web
	if sess := p.pool.findSessions(idWeb); sess != nil {
		bWebPushed = true
		var sendMsg protocol.Push
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_PUSH)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg) + 3)
		sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)
		sendMsg.Offline = 0
		sendMsg.Flag = 1

		buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_JSON)
		logs.Logger.Debug("([PUSH]  to web id=", msg.Id, " Tid=", sendMsg.Tid, " offline=", sendMsg.Offline, " flag=", sendMsg.Flag)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("push write error:", err)
		}
	}

	//获取用户终端类型 是android还是iOS
	plats := p.userInfo(id)
	for _, plat := range plats {
		if plat == "Android" {
			if sess := p.pool.findSessions(idAndroid); sess != nil {
				var sendMsg protocol.Push
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_PUSH)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg) + 3)
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)
				sendMsg.Offline = 0
				if !bWebPushed {
					sendMsg.Flag = 1
				}

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([PUSH]  to Android id=", msg.Id, " Tid=", sendMsg.Tid, " offline=", sendMsg.Offline, " flag=", sendMsg.Flag)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...) //mem leak
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				} else {
					//推送失败存储离线消息
					logs.Logger.Error("write to addr ", connString(sess.conn), " err ", err)
					p.store.SavePushMsg(id, msg.Msg)
				}
			} else {
				//android 不在线
				//存储为离线消息
				p.store.SavePushMsg(id, msg.Msg)
			}
		}
		if plat == "iOS" {
			if sess := p.pool.findSessions(idIos); sess != nil {
				var sendMsg protocol.Push
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_PUSH)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg) + 3)
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)
				sendMsg.Offline = 0
				if !bWebPushed {
					sendMsg.Flag = 1
				}

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([PUSH]  to iOS id=", msg.Id, " Tid=", sendMsg.Tid, " offline=", sendMsg.Offline, " flag=", sendMsg.Flag)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...)
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				}
			} else {
				//ios不在线
				//推送APNS
				apnsPush(id, msg.Msg)
			}
		}
	}
}

//callback 从nsq中接收消息并推送到客户端
//回调规则：
//1、web在线则推送给web
//2、android在线推android，不在线存离线消息
//3、iOS在线推iOS，不在线存离线消息
//4、如果不是android或者ios用户，对应逻辑不处理
func (p *Comet) callback(msg *rpc.PushRequst) {
	id := msg.Id
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	idAndroid := fmt.Sprintf("%s-%d", id, protocol.PLAT_ANDROID)
	idIos := fmt.Sprintf("%s-%d", id, protocol.PLAT_IOS)
	logs.Logger.Debug("([>>>CALLBACK]  id=", msg.Id, " msg=", string(msg.Msg[:]))

	//按照推送规则
	//1:首先推web
	if sess := p.pool.findSessions(idWeb); sess != nil {
		var sendMsg protocol.Callback
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_CALLBACK)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg))
		sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

		buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_JSON)
		logs.Logger.Debug("([CALLBACK]  to web id=", msg.Id, " Tid=", sendMsg.Tid)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("push write error:", err)
		}
	}

	//获取用户终端类型 是android还是iOS
	plats := p.userInfo(id)
	for _, plat := range plats {
		if plat == "Android" {
			if sess := p.pool.findSessions(idAndroid); sess != nil {
				var sendMsg protocol.Callback
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_CALLBACK)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg))
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([CALLBACK]  to Android id=", msg.Id, " Tid=", sendMsg.Tid)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...)
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				} else {
					//推送失败 存离线
					p.store.SaveCallbackMsg(id, protocol.PLAT_ANDROID, msg.Msg)
				}
			} else {
				//不在线 存储为离线消息
				p.store.SaveCallbackMsg(id, protocol.PLAT_ANDROID, msg.Msg)
			}
		}
		if plat == "iOS" {
			if sess := p.pool.findSessions(idIos); sess != nil {
				var sendMsg protocol.Callback
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_CALLBACK)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg))
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([CALLBACK]  to iOS id=", msg.Id, " Tid=", sendMsg.Tid)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...)
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				} else {
					//推送失败 存离线
					p.store.SaveCallbackMsg(id, protocol.PLAT_IOS, msg.Msg)
				}
			} else {
				//不在线存储为离线消息
				p.store.SaveCallbackMsg(id, protocol.PLAT_IOS, msg.Msg)
			}
		}
	}
}

//message 从nsq中接收消息并推送到客户端
//IM规则：
//1、web在线则推送给web
//2、android在线推android，不在线存离线消息
//3、iOS在线推iOS，不在线存离线消息
//4、如果不是android或者ios用户，对应逻辑不处理
func (p *Comet) message(msg *rpc.PushRequst) {
	id := msg.Id
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	idAndroid := fmt.Sprintf("%s-%d", id, protocol.PLAT_ANDROID)
	idIos := fmt.Sprintf("%s-%d", id, protocol.PLAT_IOS)
	logs.Logger.Debug("([>>>MESSAGE]  id=", msg.Id, " msg=", string(msg.Msg[:]))

	//按照推送规则
	//1:首先推web
	if sess := p.pool.findSessions(idWeb); sess != nil {
		var sendMsg protocol.Im
		protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_MESSAGE)
		protocol.SetEncode(&sendMsg.Header, sess.encode)
		sendMsg.Tid = uint32(sess.nextTid())
		sendMsg.Len = uint32(len(msg.Msg))
		sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

		buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_JSON)
		logs.Logger.Debug("([MESSAGE]  to web id=", msg.Id, " Tid=", sendMsg.Tid)
		if err := p.write(sess.conn, buf); err != nil {
			logs.Logger.Error("push write error:", err)
		}
	}

	//获取用户终端类型 是android还是iOS
	plats := p.userInfo(id)
	for _, plat := range plats {
		if plat == "Android" {
			if sess := p.pool.findSessions(idAndroid); sess != nil {
				var sendMsg protocol.Im
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_MESSAGE)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg))
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([MESSAGE]  to Android id=", msg.Id, " Tid=", sendMsg.Tid)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...)
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				}
			} else {
				//存储为离线消息
				p.store.SaveImMsg(id, protocol.PLAT_IOS, msg.Msg)
			}
		}
		if plat == "iOS" {
			if sess := p.pool.findSessions(idIos); sess != nil {
				var sendMsg protocol.Im
				protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_MESSAGE)
				protocol.SetEncode(&sendMsg.Header, sess.encode)
				sendMsg.Tid = uint32(sess.nextTid())
				sendMsg.Len = uint32(len(msg.Msg))
				sendMsg.Msg = append(sendMsg.Msg, msg.Msg...)

				buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
				logs.Logger.Debug("([MESSAGE]  to iOS id=", msg.Id, " Tid=", sendMsg.Tid)
				if err := p.write(sess.conn, buf); err == nil {
					//创建事务并保存
					trans := newTransaction()
					trans.tid = int(sendMsg.Tid)
					trans.msg = append(trans.msg, msg.Msg...)
					sess.saveTransaction(trans)
					sess.checkTrans(trans)
				}
			} else {
				//存储为离线消息
				p.store.SaveImMsg(id, protocol.PLAT_IOS, msg.Msg)
			}
		}
	}
}
