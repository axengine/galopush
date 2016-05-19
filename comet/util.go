package main

import (
	"galopush/logs"
	"galopush/protocol"
	"net"
	"time"

	"code.google.com/p/go.net/websocket"
)

//鉴权接口
func (p *Comet) auth(id string, plat int, token string) error {
	return nil
}

//获取用户终端类型接口
func (p *Comet) userInfo(id string) []string {
	var plats []string
	plats = append(plats, "Android")
	plats = append(plats, "Web")
	return plats
}

//推送苹果APNS
func apnsPush(id string, msg []byte) error {
	return nil
}

//根据连接类型设定协议类型
func protoType(conn interface{}) int {
	switch conn.(type) {
	case *net.TCPConn:
		return protocol.PROTOCOL_TYPE_BINARY
	case *websocket.Conn:
		return protocol.PROTOCOL_TYPE_JSON
	}
	return protocol.PROTOCOL_TYPE_DEFAULT
}

//获取连接的远端地址ip:port
func connString(conn interface{}) string {
	var addr string
	switch conn.(type) {
	case *net.TCPConn:
		c := conn.(*net.TCPConn)
		addr = c.RemoteAddr().String()
	case *websocket.Conn:
		ws := conn.(*websocket.Conn)
		addr = ws.Request().RemoteAddr
	}
	return addr
}

func (p *Comet) closeConn(conn interface{}) {
	switch conn.(type) {
	case *net.TCPConn:
		c := conn.(*net.TCPConn)
		c.Close()
	case *websocket.Conn:
		ws := conn.(*websocket.Conn)
		ws.Close()
	}
}

func (p *Comet) write(conn interface{}, buf []byte) error {
	var err error
	logs.Logger.Debug("[send]", buf)
	switch conn.(type) {
	case *net.TCPConn:
		c := conn.(*net.TCPConn)
		_, err = c.Write(buf)
		if err != nil {
			logs.Logger.Error("Write:", err)
		}
	case *websocket.Conn:
		ws := conn.(*websocket.Conn)
		err = websocket.Message.Send(ws, buf)
		if err != nil {
			logs.Logger.Error("Write:", err)
		}
	}
	return err
}

//从connsocket中读取长度为len的数据，返回读取到的数据和错误
//如果timeout>0设置读超时，否则不设置
func (p *Comet) readTimeout(conn *net.TCPConn, len uint32, timeout int) (buffer []byte, err error) {
	buffer = make([]byte, len)
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(timeout)))
	} else {
		var t time.Time
		conn.SetReadDeadline(t)
	}
	if _, err = conn.Read(buffer); err != nil {
		logs.Logger.Error(conn.RemoteAddr().String(), " connection Read error: ", err)
	}

	logs.Logger.Debug("[conn]read data:", buffer)
	return
}

//取byte数组有效长度
func bytesValidLen(b [32]byte) int {
	var i int
	for i = 0; i < len(b); i++ {
		if b[i] == 0 {
			break
		}
	}
	return i
}
