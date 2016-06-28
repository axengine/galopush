package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"galopush/logs"
	"galopush/protocol"
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
	var data protocol.JsonData
	if err := json.Unmarshal(buffer, &data); err != nil {
		return err
	}

	switch data.Cmd {
	case protocol.MSGTYPE_REGISTER:
		var msg protocol.Register

		//固定头
		protocol.SetMsgType(&msg.Header, data.Cmd)
		protocol.SetEncode(&msg.Header, data.EnCode)
		msg.Tid = uint32(data.Tid)

		//可变头
		msg.Len = 66

		//解析消息体

		//将消息体转为字符数组
		var b []byte
		if data.EnCode > 0 {
			b, _ = base64.StdEncoding.DecodeString(data.Data)
		} else {
			b = []byte(data.Data)
		}

		//对消息体进行解密
		protocol.CodecDecode(b, len(b), data.EnCode)

		//将消息体还原为二进制结构
		var param protocol.JsonRegiter
		if err := json.Unmarshal(b, &param); err != nil {
			return err
		}

		//消息体
		msg.Version = byte(param.Version)
		msg.TerminalType = byte(param.TerminalType)
		idBuff := []byte(param.Id)
		for i := 0; i < 32 && i < len(idBuff); i++ {
			msg.Id[i] = idBuff[i]
		}
		tokenBuff := []byte(param.Token)
		for i := 0; i < 32 && i < len(tokenBuff); i++ {
			msg.Token[i] = tokenBuff[i]
		}
		p.Cache(conn, &msg)
	case protocol.MSGTYPE_HEARTBEAT:
		var msg protocol.Header

		//固定头
		protocol.SetMsgType(&msg, data.Cmd)
		protocol.SetEncode(&msg, data.EnCode)
		msg.Tid = uint32(data.Tid)
		p.Cache(conn, &msg)
	case protocol.MSGTYPE_PUSHRESP, protocol.MSGTYPE_CBRESP, protocol.MSGTYPE_MSGRESP:
		var msg protocol.Resp
		//固定头
		protocol.SetMsgType(&msg.Header, data.Cmd)
		protocol.SetEncode(&msg.Header, data.EnCode)
		msg.Tid = uint32(data.Tid)

		//可变头
		msg.Len = 1
		//解析消息体

		//将消息体转为字符数组
		var b []byte
		if data.EnCode > 0 {
			b, _ = base64.StdEncoding.DecodeString(data.Data)
		} else {
			b = []byte(data.Data)
		}

		//对消息体进行解密
		protocol.CodecDecode(b, len(b), data.EnCode)

		//将消息体还原为二进制结构
		var param protocol.ParamResp
		if err := json.Unmarshal(b, &param); err != nil {
			return err
		}

		//消息体
		msg.Code = param.Code
		p.Cache(conn, &msg)
	case protocol.MSGTYPE_MESSAGE:
		var msg protocol.Im
		//固定头
		protocol.SetMsgType(&msg.Header, data.Cmd)
		protocol.SetEncode(&msg.Header, data.EnCode)
		msg.Tid = uint32(data.Tid)

		//解析消息体

		//将消息体转为字符数组
		var b []byte
		if data.EnCode > 0 {
			b, _ = base64.StdEncoding.DecodeString(data.Data)
		} else {
			b = []byte(data.Data)
		}

		//附加头
		msg.Len = uint32(len(b))

		//对消息体进行解密
		protocol.CodecDecode(b, len(b), data.EnCode)

		//将消息体还原为二进制结构
		msg.Msg = b

		p.Cache(conn, &msg)
	default:
		return errors.New("error msg type")
	}
	return nil
}
