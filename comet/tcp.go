package main

import (
	"galopush/logs"
	"galopush/protocol"
	"net"
	"time"
)

const (
	MAX_MSG_LEN = 1024 * 1024 * 10
)

type socketData struct {
	conn interface{}
	msg  interface{}
}

const (
	default_read_timeout = 10
)

func (p *Comet) startTcpServer() {
	go p.tcpServ()
}

func (p *Comet) tcpServ() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("TcpServ.recover:", r)
			go p.tcpServ()
		}
	}()
	var (
		conn   *net.TCPConn
		listen *net.TCPListener
		addr   *net.TCPAddr
		err    error
	)
	if addr, err = net.ResolveTCPAddr("tcp4", p.uaTcpAddr); err != nil {
		panic(err)
	}

	if listen, err = net.ListenTCP("tcp4", addr); err != nil {
		panic(err)
	}
	defer listen.Close()
	logs.Logger.Debug("start socketServ on ", addr, " wait for conn...")
	for {
		if conn, err = listen.AcceptTCP(); err != nil {
			logs.Logger.Error(err)
			continue
		}
		if err = conn.SetKeepAlive(true); err != nil {
			logs.Logger.Error(err)
			conn.Close()
			continue
		}
		//http://www.oschina.net/translate/tcp-keepalive-with-golang
		//real result:30second
		if err = conn.SetKeepAlivePeriod(time.Second * 30); err != nil {
			logs.Logger.Error(err)
			conn.Close()
			continue
		}

		logs.Logger.Debug("new tcp conn from ", conn.RemoteAddr().String(), " localaddr:", conn.LocalAddr().String())
		go p.handleTcpConn(conn)
	}
}

func (p *Comet) handleTcpConn(conn *net.TCPConn) {
	//count socket conn
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	p.cnt.Add("tcp")
	defer func() {
		conn.Close()
		p.procUnRegister(conn)
		p.cnt.Sub("tcp")
	}()

	for {
		var (
			h      *protocol.Header
			ah     *protocol.AddHeader
			buffer []byte
			err    error
		)

		//读固定头
		if buffer, err = p.readTimeout(conn, protocol.FIX_HEADER_LEN, 0); err != nil {
			return
		}

		//解析固定头
		if h, err = protocol.DecodeHeader(buffer); err != nil {
			logs.Logger.Error(connString(conn), " DecodeHeader error: ", err)
			return
		}

		//校验消息类型
		msgType := protocol.GetMsgType(h)
		if msgType == protocol.MSGTYPE_DEFAULT || msgType >= protocol.MSGTYPE_MAX {
			logs.Logger.Error(connString(conn), " Error msg type: ", msgType)
			return
		}

		//心跳消息无消息体，特殊处理
		if msgType == protocol.MSGTYPE_HEARTBEAT {
			p.Cache(conn, h)
			continue
		}

		//读取附加头
		if buffer, err = p.readTimeout(conn, protocol.ADD_HEADER_LEN, default_read_timeout); err != nil {
			return
		}

		//解析附加头
		if ah, err = protocol.DecodeAddHeader(buffer); err != nil {
			logs.Logger.Error(connString(conn), " DecodeAddHeader error: ", err)
			return
		}
		if ah.Len > MAX_MSG_LEN {
			logs.Logger.Error(connString(conn), " error body len=", ah.Len)
			return
		}
		switch msgType {
		case protocol.MSGTYPE_KICKRESP:
			//读取body 直接丢弃
			if _, err = p.readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}
		//应答
		case protocol.MSGTYPE_PUSHRESP, protocol.MSGTYPE_CBRESP, protocol.MSGTYPE_MSGRESP:
			//读取body
			var buffer []byte
			if buffer, err = p.readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			//解密
			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamResp
			if param, err = protocol.DecodeParamResp(buffer); err != nil {
				logs.Logger.Error(connString(conn), " DecodeParamResp error: ", err)
				return
			}

			var msg protocol.Resp
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamResp = *param
			p.Cache(conn, &msg)
		//注册
		case protocol.MSGTYPE_REGISTER:
			//读取body
			var buffer []byte
			if buffer, err = p.readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			//解密
			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamReg
			if param, err = protocol.DecodeParamReg(buffer); err != nil {
				logs.Logger.Error(connString(conn), " DecodeParamUaReg error: ", err)
				return
			}
			var msg protocol.Register
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamReg = *param
			p.Cache(conn, &msg)
		//IM
		case protocol.MSGTYPE_MESSAGE:
			//读取body
			var buffer []byte
			if buffer, err = p.readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			//解密
			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			var msg protocol.ImUp
			msg.Header = *h
			msg.AddHeader = *ah
			msg.Msg = buffer
			p.Cache(conn, &msg)
		default:
			logs.Logger.Error(connString(conn), " Error MsgType: ", msgType)
			return
		}
	}
}
