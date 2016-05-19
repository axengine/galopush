//tcp客户端并发测试
package main

import (
	"flag"
	"fmt"
	"galopush/counter"
	"galopush/protocol"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	loadblancer           = "http://192.168.1.63:5150/v1/gComet.addr"
	default_read_timeout  = 5
	MAX_SOCKET_CONNECTION = 60000
)

var cnt *counter.Counter

type UA struct {
	uid    string
	plat   int
	token  string
	encode int
	tid    uint32
	ch     chan interface{}
	conn   net.Conn
}

func main() {
	var userid string
	var laddr string
	flag.StringVar(&userid, "u", "testid", "current userid,default testid ")
	flag.StringVar(&laddr, "l", "192.168.1.241:0", "current laddr,default 192.168.1.241:0 ")
	flag.Parse()
	fmt.Println("uid=", userid, " laddr=", laddr)
	cnt = counter.NewCounter()
	//统计数据
	go func() {
		t := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-t.C:
				fmt.Println(cnt.String())
			}
		}
	}()
	for i := 0; i < MAX_SOCKET_CONNECTION; i++ {
		ua := new(UA)
		ua.uid = fmt.Sprintf("%s%d", userid, i)
		ua.plat = 1
		ua.token = fmt.Sprintf("token%d", i)
		ua.encode = protocol.ENCODE_LOOP_XOR
		ua.tid = uint32(i)
		ua.ch = make(chan interface{})
		go ua.start(laddr)
		time.Sleep(time.Millisecond * 5)
	}

	select {}
}
func (p *UA) start(bind string) {
	cometAddr, err := p.getCometAddr()
	if err != nil {
		cnt.Add("getCometAddr")
		return
	}
	//conn, err := net.Dial("tcp", cometAddr)
	laddr, err := net.ResolveTCPAddr("tcp", bind)
	if err != nil {
		panic(err)
	}
	raddr, err := net.ResolveTCPAddr("tcp", cometAddr)
	if err != nil {
		panic(err)
	}
	conn, err := net.DialTCP("tcp", laddr, raddr)
	if err != nil {
		cnt.Add("net.Dial")
		fmt.Println("connot connect to cometAddr ", err)
		return
	}
	p.conn = conn
	go p.service()
	go p.readRuntime()
	//sendAnyThing(conn)
	p.register()
	cnt.Add("new ua")
}
func (p *UA) service() {
	for {
		select {
		case m := <-p.ch:
			p.proc(m)
		}
	}
}

func (p *UA) proc(msg interface{}) error {
	switch msg.(type) {
	case *protocol.Resp:
		recv := msg.(*protocol.Resp)
		msgType := protocol.GetMsgType(&recv.Header)
		code := recv.Code
		//fmt.Println("receive resp with msgType=", msgType, " code=", code)
		if msgType == protocol.MSGTYPE_REGRESP && code == 0 {
			cnt.Add("register success")
			//fmt.Println("register success")
			go p.pingRuntime()
		} else if msgType == protocol.MSGTYPE_REGRESP && code != 0 {
			cnt.Add("register failed")
		}
	case *protocol.Push:
		recv := msg.(*protocol.Push)
		fmt.Println("receive Push with offline=", recv.Offline, " flag=", recv.Flag, " msg=", string(recv.Msg[:]))
		p.resp(protocol.MSGTYPE_MSGRESP, recv.Tid, 0)
	case *protocol.Callback:
		recv := msg.(*protocol.Callback)
		fmt.Println("receive callback with  msg=", string(recv.Msg[:]))
		p.resp(protocol.MSGTYPE_CBRESP, recv.Tid, 0)
	case *protocol.Im:
		recv := msg.(*protocol.Im)
		fmt.Println("receive IM with  msg=", string(recv.Msg[:]))
		p.resp(protocol.MSGTYPE_MSGRESP, recv.Tid, 0)
	}
	return nil
}

func (p *UA) sendAnyThing(conn net.Conn) error {
	b := make([]byte, 1024)
	b[0] = 11
	b[1] = 1

	_, err := conn.Write(b)
	if err != nil {
		fmt.Println("connot write to socket ", err)
		return err
	}
	return nil
}

func (p *UA) register() error {
	var msg protocol.Register
	protocol.SetMsgType(&msg.Header, protocol.MSGTYPE_REGISTER)
	protocol.SetEncode(&msg.Header, p.encode)
	p.tid = p.tid + 1
	msg.Tid = p.tid

	msg.Len = 66

	msg.ParamReg.Version = 0x01
	msg.ParamReg.TerminalType = protocol.PLAT_ANDROID

	bufId := []byte(p.uid)
	for i := 0; i < len(bufId) && i < 32; i++ {
		msg.ParamReg.Id[i] = bufId[i]
	}

	bufToken := []byte(p.token)
	for i := 0; i < len(bufToken) && i < 32; i++ {
		msg.ParamReg.Token[i] = bufToken[i]
	}
	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)
	//fmt.Println("send ", b)
	_, err := p.conn.Write(b)
	if err != nil {
		cnt.Add("net.Write")
		return err
	}
	return nil
}

func (p *UA) resp(msgType int, tid uint32, code int) error {
	var msg protocol.Resp
	protocol.SetMsgType(&msg.Header, msgType)
	protocol.SetEncode(&msg.Header, p.encode)
	msg.Tid = tid

	msg.Len = 1

	msg.Code = byte(code)

	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)

	_, err := p.conn.Write(b)
	if err != nil {
		log.Println(err)
	}
	return nil
}

func (p *UA) pingRuntime() {
	t := time.NewTicker(time.Second * 300)
	for {
		select {
		case <-t.C:
			p.heartbeat()
		}
	}
}

func (p *UA) heartbeat() error {
	var msg protocol.Header
	protocol.SetMsgType(&msg, protocol.MSGTYPE_HEARTBEAT)
	protocol.SetEncode(&msg, p.encode)
	p.tid = p.tid + 1
	msg.Tid = p.tid

	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)

	_, err := p.conn.Write(b)
	if err != nil {
		fmt.Println(err, " ", b)
		return err
	}
	return nil
}
func (p *UA) im(conn net.Conn, body string) error {
	var msg protocol.Im
	protocol.SetMsgType(&msg.Header, protocol.MSGTYPE_MESSAGE)
	protocol.SetEncode(&msg.Header, p.encode)
	p.tid = p.tid + 1
	msg.Tid = p.tid
	data := []byte(body)
	msg.Len = uint32(len(data))

	msg.Msg = data

	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)
	//protocol.CodecEncode(b[protocol.HEADER_LEN:], int(msg.Len), protocol.GetEncode(&msg.Header))

	_, err := conn.Write(b)
	if err != nil {
		fmt.Println("connot write to socket ", err)
		return err
	}
	return nil
}

func (p *UA) readRuntime() {
	for {
		var (
			h      *protocol.Header
			ah     *protocol.AddHeader
			buffer []byte
			err    error
		)

		//读固定头
		if buffer, err = readTimeout(p.conn, protocol.FIX_HEADER_LEN, 0); err != nil {
			fmt.Println(err)
			return
		}

		//解析固定头
		if h, err = protocol.DecodeHeader(buffer); err != nil {
			log.Println(p.conn.RemoteAddr().String(), " DecodeHeader error: ", err)
			return
		}

		//校验消息类型
		msgType := protocol.GetMsgType(h)
		if msgType == protocol.MSGTYPE_DEFAULT || msgType >= protocol.MSGTYPE_MAX {
			log.Println(p.conn.RemoteAddr().String(), " Error msg type: ", msgType)
			return
		}

		//心跳消息无消息体，特殊处理
		//if msgType == protocol.MSGTYPE_HEARTBEAT || msgType == protocol.MSGTYPE_HBRESP {
		if msgType == protocol.MSGTYPE_HEARTBEAT {
			p.ch <- h
			continue
		}

		//读取附加头
		if buffer, err = readTimeout(p.conn, protocol.ADD_HEADER_LEN, default_read_timeout); err != nil {
			log.Println("read error on add header ", err)
			return
		}

		//解析附加头
		if ah, err = protocol.DecodeAddHeader(buffer); err != nil {
			log.Println(p.conn.RemoteAddr().String(), " DecodeAddHeader error: ", err)
			return
		}

		switch msgType {
		//应答
		case protocol.MSGTYPE_REGRESP, protocol.MSGTYPE_HBRESP, protocol.MSGTYPE_MSGRESP:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(p.conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamResp
			if param, err = protocol.DecodeParamResp(buffer); err != nil {
				log.Println(p.conn.RemoteAddr().String(), " DecodeParamResp error: ", err)
				return
			}

			var msg protocol.Resp
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamResp = *param
			p.ch <- &msg

		//PUSH
		case protocol.MSGTYPE_PUSH:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(p.conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamPush
			if param, err = protocol.DecodeParamPush(buffer); err != nil {
				log.Println(p.conn.RemoteAddr().String(), " DecodeParamPush error: ", err)
				return
			}

			var msg protocol.Push
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamPush = *param
			msg.Msg = buffer[3:]
			fmt.Println(param.Offline, param.Flag, string(buffer[3:]))
			p.ch <- &msg
		//CALLBACK
		case protocol.MSGTYPE_CALLBACK:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(p.conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			var msg protocol.Callback
			msg.Header = *h
			msg.AddHeader = *ah
			msg.Msg = buffer
			p.ch <- &msg
		//IM
		case protocol.MSGTYPE_MESSAGE:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(p.conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			var msg protocol.Im
			msg.Header = *h
			msg.AddHeader = *ah
			msg.Msg = buffer
			p.ch <- &msg
		default:
			log.Println(p.conn.RemoteAddr().String(), " Error MsgType: ", msgType)
			return
		}
	}
}

func readTimeout(conn net.Conn, len uint32, timeout int) (buffer []byte, err error) {
	buffer = make([]byte, len)
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(timeout)))
	} else {
		var t time.Time
		conn.SetReadDeadline(t)
	}
	if _, err = conn.Read(buffer); err != nil {
		log.Println(conn.RemoteAddr().String(), " connection Read error: ", err)
	}

	//fmt.Println("buffer=", buffer)
	return
}
func (p *UA) getCometAddr() (string, error) {
	var addr string
	resp, err := http.PostForm(loadblancer,
		url.Values{"id": {p.uid}, "termtype": {fmt.Sprintf("%d", p.plat)}})
	if err != nil {
		return addr, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return addr, err
	}
	return string(body[:]), err
}
