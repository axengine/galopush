package main

import (
	"fmt"
	"galopush/internal/logs"
	"galopush/internal/protocol"
)

func (p *Comet) ProcOfflineMsg(sess *session, id string, plat int) {
	//web端没有离线消息
	if plat == protocol.PLAT_WEB {
		return
	}

	//判断web是否在线
	iWebOnline := 0
	idWeb := fmt.Sprintf("%s-%d", id, protocol.PLAT_WEB)
	if sWeb := p.pool.findSessions(idWeb); sWeb != nil {
		iWebOnline = 1
	}

	//离线消息处理
	if plat == protocol.PLAT_ANDROID {
		//需要处理离线push
		offPush := p.store.GetPushMsg(id, plat)
		if offPush != nil {
			var sendMsg protocol.Push
			protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_PUSH)
			protocol.SetEncode(&sendMsg.Header, sess.encode)
			sendMsg.Tid = uint32(sess.nextTid())
			sendMsg.Len = uint32(len(offPush.Msg) + 3)
			sendMsg.Msg = append(sendMsg.Msg, offPush.Msg...)
			sendMsg.Offline = offPush.OffCnt
			sendMsg.Flag = uint8(iWebOnline)

			buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
			logs.Logger.Info("[on register] send offMsg push msg id=", id, " count=", offPush.OffCnt, " buff=", string(offPush.Msg), " flag=", iWebOnline)
			if err := p.write(sess.conn, buf); err == nil {
				//创建事务并保存
				trans := newTransaction()
				trans.tid = int(sendMsg.Tid)
				trans.msgType = protocol.MSGTYPE_PUSH
				trans.msg = append(trans.msg, offPush.Msg...) //mem leak
				sess.saveTransaction(trans)
				sess.checkTrans(trans)
			} else {
				//推送失败存储离线消息
				logs.Logger.Error("write to addr ", connString(sess.conn), " err ", err)
				p.store.SavePushMsg(id, plat, offPush.Msg)
			}
		}
	}

	//需处理离线callback & im
	offCb := p.store.GetCallbackMsg(id, plat)
	if offCb != nil {
		for _, v := range offCb {
			buff := v.Msg
			var sendMsg protocol.Callback
			protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_CALLBACK)
			protocol.SetEncode(&sendMsg.Header, sess.encode)
			sendMsg.Tid = uint32(sess.nextTid())
			sendMsg.Len = uint32(len(buff))
			sendMsg.Msg = append(sendMsg.Msg, buff...)

			buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
			logs.Logger.Info("[on register] send offMsg callback msg id=", id, " msg=", string(buff))
			if err := p.write(sess.conn, buf); err == nil {
				//创建事务并保存
				trans := newTransaction()
				trans.tid = int(sendMsg.Tid)
				trans.msgType = protocol.MSGTYPE_CALLBACK
				trans.msg = append(trans.msg, buff...) //mem leak
				sess.saveTransaction(trans)
				sess.checkTrans(trans)
			} else {
				//推送失败存储离线消息
				logs.Logger.Error("write to addr ", connString(sess.conn), " err ", err)
				p.store.SaveCallbackMsg(id, plat, buff)
			}
		}
	}

	offIm := p.store.GetImMsg(id, plat)
	if offIm != nil {
		for _, v := range offIm {
			it := v
			var sendMsg protocol.ImDown
			protocol.SetMsgType(&sendMsg.Header, protocol.MSGTYPE_MESSAGE)
			protocol.SetEncode(&sendMsg.Header, sess.encode)
			sendMsg.Tid = uint32(sess.nextTid())
			sendMsg.Len = uint32(len(it.Msg) + 1)
			if it.WebOnline {
				sendMsg.Flag = 1
			}

			sendMsg.Msg = append(sendMsg.Msg, it.Msg...)

			buf := protocol.Pack(&sendMsg, protocol.PROTOCOL_TYPE_BINARY)
			logs.Logger.Info("[on register] send offMsg Im msg id=", id, " msg=", string(it.Msg), " flag=", it.WebOnline)
			if err := p.write(sess.conn, buf); err == nil {
				//创建事务并保存
				trans := newTransaction()
				trans.tid = int(sendMsg.Tid)
				trans.msgType = protocol.MSGTYPE_MESSAGE
				trans.msg = append(trans.msg, it.Msg...) //mem leak
				sess.saveTransaction(trans)
				sess.checkTrans(trans)
			} else {
				//推送失败存储离线消息
				logs.Logger.Error("write to addr ", connString(sess.conn), " err ", err)
				p.store.SaveImMsg(id, plat, it.Msg)
			}
		}
	}
}
