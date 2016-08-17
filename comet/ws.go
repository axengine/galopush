package main

import (
	//	"encoding/json"
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
	err := http.ListenAndServe(p.uaWsBindAddr, nil)
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

		msg, err := protocol.UnPackJson(buf)
		if err != nil {
			logs.Logger.Error("protocol.DecodeJson error=", err, " buf=", string(buf[:]))
			return
		}
		p.Cache(conn, msg)
	}
}
