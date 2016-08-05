package main

import (
	"encoding/binary"
	"fmt"
	"galopush/internal/logs"
	"galopush/internal/protocol"
	"galopush/internal/rpc"
)

//startSocketHandle 启动runtime个协程处理客户端数据
func (p *Comet) startSocketHandle() {
	for i := 0; i < p.runtime; i++ {
		go p.handleMessage()
	}
}

//handleMessage 从dataChan读取数据并处理
//dataChan数据为客户端请求或响应数据
func (p *Comet) handleMessage() {
	for {
		select {
		case data, ok := <-p.dataChan:
			if !ok {
				return
			}
			p.procTrans(data)
		}
	}
}

//procTrans 处理客户端数据
func (p *Comet) procTrans(data interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	conn := data.(*socketData).conn
	msg := data.(*socketData).msg
	//logs.Logger.Debug("Receive Tran type=", reflect.TypeOf(msg), " msg=", reflect.ValueOf(msg))
	switch msg.(type) {
	//注册
	case *protocol.Register:
		reg := msg.(*protocol.Register)
		p.procRegister(conn, reg)
		//心跳
	case *protocol.Header:
		head := msg.(*protocol.Header)
		p.procPing(conn, head)
		//应答
	case *protocol.Resp:
		push := msg.(*protocol.Resp)
		p.procResp(conn, push)
		//即时消息
	case *protocol.ImUp:
		push := msg.(*protocol.ImUp)
		p.procIm(conn, push)
	}
}

//procRegister 处理用户注册消息
func (p *Comet) procRegister(conn interface{}, msg *protocol.Register) {
	request := msg
	id := string(request.Id[:bytesValidLen(request.Id)])
	token := string(request.Token[:bytesValidLen(request.Token)])
	plat := int(request.TerminalType)
	msgType := protocol.GetMsgType(&request.Header)
	encode := protocol.GetEncode(&request.Header)
	addr := connString(conn)
	pType := protoType(conn)

	logs.Logger.Info("[>>>register] request id=", id, " plat=", plat, " token=", token, " addr=", addr)

	var authCode byte

	//鉴权
	if err := p.auth(id, plat, token); err != nil {
		logs.Logger.Error("[register] auth err=", err, " id=", id, " plat=", plat, " token=", token, " addr=", addr)
		authCode = 3
		p.response(conn, msgType+1, encode, request.Tid, authCode, pType)
		return
	}

	//是否重复发送注册消息
	if ids := p.pool.findId(addr); ids != "" {
		logs.Logger.Error("[register] repeat register  id=", id, " plat=", plat, " token=", token, " addr=", addr)
		authCode = 0
		p.response(conn, msgType+1, encode, request.Tid, authCode, pType)
		return
	}

	{
		//相同类型终端登录
		idf := fmt.Sprintf("%s-%d", id, plat)
		if sess := p.pool.findSessions(idf); sess != nil {
			logs.Logger.Info("register kick id=", id, " plat=", plat)
			p.closeConn(sess.conn)
		}
		//互斥登录
		if plat == 1 && plat == 2 {
			var idf string
			if plat == 1 {
				idf = fmt.Sprintf("%s-%d", id, 2)
			} else {
				idf = fmt.Sprintf("%s-%d", id, 1)
			}
			if sess := p.pool.findSessions(idf); sess != nil {
				logs.Logger.Info("register kick ids=", idf)
				p.closeConn(sess.conn)
			}
		}
	}

	//建立session 并初始化
	sess := new(session)
	sess.id = id
	sess.plat = plat
	sess.conn = conn
	sess.encode = encode
	sess.token = token
	sess.tid = 0

	//保存session
	idf := fmt.Sprintf("%s-%d", id, plat)
	p.pool.insertSessions(idf, sess)
	p.pool.insertIds(addr, idf)

	logs.Logger.Debug("[register] success sess=", sess, " addr=", addr)

	//登陆成功
	p.response(conn, msgType+1, encode, request.Tid, authCode, pType)

	//上报到router
	p.rpcCli.Notify(id, plat, token, rpc.STATE_ONLINE, p.cometId)

	//处理离线消息
	p.ProcOfflineMsg(sess, id, plat)
}

//procUnRegister 用户离线、异常离线
func (p *Comet) procUnRegister(conn interface{}) {
	addr := connString(conn)

	//session校验
	id := p.pool.findId(addr)
	if id == "" {
		logs.Logger.Debug("[unregister] id is nil  addr=", addr)
		return
	}
	sess := p.pool.findSessions(id)
	if sess == nil {
		logs.Logger.Debug("[unregister] sess is nil id=", id, " addr=", addr)
		return
	}
	logs.Logger.Debug("[unregister] success sess=", sess, " addr=", addr)

	//通知router
	p.rpcCli.Notify(sess.id, sess.plat, sess.token, rpc.STATE_OFFLINE, p.cometId)

	sess.destroy()

	//清除连接池
	p.pool.deleteIds(addr)
	p.pool.deleteSessions(id)
}

//procIm 处理用户IM即时消息
func (p *Comet) procIm(conn interface{}, msg *protocol.ImUp) {
	request := msg
	msgType := protocol.GetMsgType(&request.Header)
	encode := protocol.GetEncode(&request.Header)
	addr := connString(conn)
	pType := protoType(conn)
	//session校验
	id := p.pool.findId(addr)
	if id == "" {
		logs.Logger.Error("[>>>IM]  id is nil addr=", addr)
		p.closeConn(conn)
		return
	}
	sess := p.pool.findSessions(id)
	if sess == nil {
		logs.Logger.Error("[>>>IM]  session is nil addr=", addr, " id=", id)
		p.closeConn(conn)
		return
	}
	logs.Logger.Info("[>>>IM]   addr=", addr, " sess=", sess, " msg=", string(request.Msg[:]))

	err := p.rpcCli.MsgUpward(sess.id, sess.plat, string(request.Msg))
	if err != nil {
		//应答
		logs.Logger.Error("[IM]  publish to nsq err=", err, " addr=", addr, " sess=", sess)
		p.response(conn, msgType+1, encode, request.Tid, 1, pType)
	} else {
		p.response(conn, msgType+1, encode, request.Tid, 0, pType)
	}
	return
}

//procPing 处理心跳消息
func (p *Comet) procPing(conn interface{}, msg *protocol.Header) {
	request := msg
	msgType := protocol.GetMsgType(request)
	encode := protocol.GetEncode(request)
	addr := connString(conn)
	pType := protoType(conn)
	id := p.pool.findId(addr)
	if id != "" {
		logs.Logger.Debug("[ping] request addr=", addr, " encode=", encode, " pType=", pType)
		p.response(conn, msgType+1, encode, request.Tid, 0, pType)
	} else {
		logs.Logger.Error("[ping] connot find id with addr ", addr)
		p.response(conn, msgType+1, encode, request.Tid, 1, pType)
	}
}

//procPushResp 处理push/callback/im响应
func (p *Comet) procResp(conn interface{}, msg *protocol.Resp) {
	addr := connString(conn)
	logs.Logger.Debug("[resp] type=", protocol.GetMsgType(&msg.Header), " addr=", addr)
	id := p.pool.findId(addr)
	sess := p.pool.findSessions(id)
	if sess != nil {
		trans := sess.getTrans(int(msg.Tid))
		if trans != nil {
			trans.timer.Stop()
			trans.exit <- 1
		} else {
			logs.Logger.Error("[resp] connot find trans type=", protocol.GetMsgType(&msg.Header), " addr=", addr, " id=", id, " tid=", msg.Tid)
		}
	} else {
		logs.Logger.Error("[resp] connot find session type=", protocol.GetMsgType(&msg.Header), " addr=", addr, " id=", id)
	}
}

func (p *Comet) response(conn interface{}, msgType int, encode int, tid uint32, code byte, protocolType int) error {
	logs.Logger.Debug("[response>>>] type=", msgType, " code=", code, " addr=", connString(conn), " protocolType=", protocolType)
	var resp protocol.Resp
	protocol.SetMsgType(&resp.Header, msgType)
	protocol.SetEncode(&resp.Header, encode)
	resp.Tid = tid
	resp.Len = uint32(binary.Size(resp.ParamResp))
	resp.Code = code
	buf := protocol.Pack(&resp, protocolType)
	//logs.Logger.Debug("send:", buf)
	return p.write(conn, buf)
}

func (p *Comet) Cache(conn interface{}, msg interface{}) {
	var data socketData
	data.conn = conn
	data.msg = msg
	p.dataChan <- &data
}
