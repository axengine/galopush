package main

import (
	"encoding/json"
	"galopush/internal/logs"
	"galopush/internal/protocol"
	"net/http"

	"code.google.com/p/go.net/websocket"
)

func (p *Comet) startWsServer() {
	go p.wsServ()
}

func (p *Comet) wsServ() {
	http.Handle("/", websocket.Handler(p.handlerWsconn))
	err := http.ListenAndServe(p.uaWsAddr, nil)
	if err != nil {
		panic("ListenANdServe: " + err.Error())
	}
}

func (p *Comet) handlerWsconn(conn *websocket.Conn) {
	logs.Logger.Debug("new ws conn from ", conn.Request().RemoteAddr, " localaddr:", conn.LocalAddr().String())
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	p.cnt.Add("ws")

	defer func() {
		p.closeConn(conn)
		p.procUnRegister(conn)
		p.cnt.Sub("ws")
	}()

	for {
		var buf []byte
		err := websocket.Message.Receive(conn, &buf)
		if err != nil {
			logs.Logger.Debug("websocket.Message.Receive error=", err)
			return
		}
		logs.Logger.Debug("ws receive ", string(buf))
		if err = p.unMarshal(conn, buf); err != nil {
			logs.Logger.Error("protocol.DecodeJson error=", err, " buf=", string(buf[:]))
			return
		}
	}
}

//unMarshal 将json字符串解析并转换为二进制结构，并发送到数据处理channel
//让业务处理模块能对tcp/ws一致性处理
//返回值:error
func (p *Comet) unMarshal(conn *websocket.Conn, buffer []byte) error {
	var body map[string]interface{}
	if err := json.Unmarshal(buffer, &body); err != nil {
		logs.Logger.Error(err, " msg=", string(buffer))
		return err
	}
	cmd := int(body["cmd"].(float64))
	tid := int(body["tid"].(float64))
	data := body["data"].(map[string]interface{})
	switch cmd {
	case protocol.MSGTYPE_REGISTER:
		{
			var msg protocol.Register
			protocol.SetMsgType(&msg.Header, cmd)
			protocol.SetEncode(&msg.Header, 0)
			msg.Tid = uint32(tid)
			msg.Len = 66

			//消息体
			msg.Version = byte(data["version"].(float64))
			msg.TerminalType = byte(data["termType"].(float64))
			idBuff := []byte(data["id"].(string))
			for i := 0; i < 32 && i < len(idBuff); i++ {
				msg.Id[i] = idBuff[i]
			}
			tokenBuff := []byte(data["token"].(string))
			for i := 0; i < 32 && i < len(tokenBuff); i++ {
				msg.Token[i] = tokenBuff[i]
			}
			p.Cache(conn, &msg)
		}
	case protocol.MSGTYPE_HEARTBEAT:
		{
			var msg protocol.Header
			//固定头
			protocol.SetMsgType(&msg, cmd)
			protocol.SetEncode(&msg, 0)
			msg.Tid = uint32(tid)
			p.Cache(conn, &msg)
		}

	case protocol.MSGTYPE_PUSHRESP, protocol.MSGTYPE_CBRESP, protocol.MSGTYPE_MSGRESP, protocol.MSGTYPE_KICKRESP:
		{
			var msg protocol.Resp
			//固定头
			protocol.SetMsgType(&msg.Header, cmd)
			protocol.SetEncode(&msg.Header, 0)
			msg.Tid = uint32(tid)

			//可变头
			msg.Len = 1
			msg.Code = byte(data["code"].(float64))
			p.Cache(conn, &msg)
		}
	case protocol.MSGTYPE_MESSAGE:
		var msg protocol.ImUp
		//固定头
		protocol.SetMsgType(&msg.Header, cmd)
		protocol.SetEncode(&msg.Header, 0)
		msg.Tid = uint32(tid)

		content := data["msg"].(string)
		contentBuff := []byte(content)

		msg.Len = uint32(len(contentBuff))

		msg.Msg = append(msg.Msg, contentBuff...)

		p.Cache(conn, &msg)
	}

	return nil
}
