//模拟android客户端进行测试
package main

import (
	"fmt"
	"galopush/protocol"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	android_uid          = "8"
	token                = "d73d89472cdb13a1cdf79a55bccbbd56"
	gencode              = protocol.ENCODE_LOOP_XOR
	loadblancer          = "http://192.168.1.63:5150/v1/gComet.addr"
	default_read_timeout = 5
)

var (
	tid uint32
	ch  chan interface{}
)

func main() {
	ch = make(chan interface{})
	cometAddr, err := getCometAddr()
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := net.Dial("tcp", cometAddr)
	if err != nil {
		fmt.Println("connot connect to cometAddr ", err)
		return
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP)
	go func(conn net.Conn) {
		select {
		case <-signalChan:
			im(conn, "helloworld-------")
		}
	}(conn)
	go service(conn)
	go readRuntime(conn)
	//sendAnyThing(conn)
	register(conn, android_uid, token, protocol.PLAT_ANDROID)

	select {}
}

func service(conn net.Conn) {
	for {
		select {
		case m := <-ch:
			proc(conn, m)
		}
	}
}

func proc(conn net.Conn, msg interface{}) error {
	switch msg.(type) {
	case *protocol.Resp:
		recv := msg.(*protocol.Resp)
		msgType := protocol.GetMsgType(&recv.Header)
		code := recv.Code
		fmt.Println("receive resp with msgType=", msgType, " code=", code)
		if msgType == protocol.MSGTYPE_REGRESP && code == 0 {
			fmt.Println("register success local=", conn.LocalAddr().String())
			go pingRuntime(conn)
		}
	case *protocol.Push:
		recv := msg.(*protocol.Push)
		fmt.Println("receive Push with offline=", recv.Offline, " flag=", recv.Flag, " msg=", string(recv.Msg[:]))
		resp(conn, protocol.MSGTYPE_MSGRESP, recv.Tid, 0)
	case *protocol.Callback:
		recv := msg.(*protocol.Callback)
		fmt.Println("receive callback with  msg=", string(recv.Msg[:]))
		resp(conn, protocol.MSGTYPE_CBRESP, recv.Tid, 0)
	case *protocol.Im:
		recv := msg.(*protocol.Im)
		fmt.Println("receive IM with  msg=", string(recv.Msg[:]))
		resp(conn, protocol.MSGTYPE_MSGRESP, recv.Tid, 0)
	}
	return nil
}

func getCometAddr() (string, error) {
	var addr string
	resp, err := http.PostForm(loadblancer,
		url.Values{"id": {android_uid}, "termtype": {"Android"}})
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

func sendAnyThing(conn net.Conn) error {
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

func register(conn net.Conn, id, token string, plat int) error {
	var msg protocol.Register
	protocol.SetMsgType(&msg.Header, protocol.MSGTYPE_REGISTER)
	protocol.SetEncode(&msg.Header, gencode)
	tid = tid + 1
	msg.Tid = tid

	msg.Len = 66

	msg.ParamReg.Version = 0x01
	msg.ParamReg.TerminalType = protocol.PLAT_ANDROID

	bufId := []byte(id)
	for i := 0; i < len(bufId) && i < 32; i++ {
		msg.ParamReg.Id[i] = bufId[i]
		//fmt.Println(bufId[i])
	}

	bufToken := []byte(token)
	for i := 0; i < len(bufToken) && i < 32; i++ {
		msg.ParamReg.Token[i] = bufToken[i]
	}
	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)
	//protocol.CodecEncode(b[protocol.HEADER_LEN:], 66, protocol.GetEncode(&msg.Header))
	fmt.Println("send ", b)
	_, err := conn.Write(b)
	if err != nil {
		fmt.Println("connot write to socket ", err)
		return err
	}
	return nil
}

func resp(conn net.Conn, msgType int, tid uint32, code int) error {
	var msg protocol.Resp
	protocol.SetMsgType(&msg.Header, msgType)
	protocol.SetEncode(&msg.Header, gencode)
	msg.Tid = tid

	msg.Len = 1

	msg.Code = byte(code)

	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)
	//protocol.CodecEncode(b[protocol.HEADER_LEN:], 1, protocol.GetEncode(&msg.Header))

	_, err := conn.Write(b)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func pingRuntime(conn net.Conn) {
	t := time.NewTicker(time.Second * 300)
	for {
		select {
		case <-t.C:
			heartbeat(conn)
		}
	}
}

func heartbeat(conn net.Conn) error {
	var msg protocol.Header
	protocol.SetMsgType(&msg, protocol.MSGTYPE_HEARTBEAT)
	protocol.SetEncode(&msg, gencode)
	tid = tid + 1
	msg.Tid = tid

	b := protocol.Pack(&msg, protocol.PROTOCOL_TYPE_BINARY)

	_, err := conn.Write(b)
	if err != nil {
		fmt.Println(err, " ", b)
		return err
	}
	return nil
}
func im(conn net.Conn, body string) error {
	var msg protocol.Im
	protocol.SetMsgType(&msg.Header, protocol.MSGTYPE_MESSAGE)
	protocol.SetEncode(&msg.Header, gencode)
	tid = tid + 1
	msg.Tid = tid
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

func readRuntime(conn net.Conn) {
	for {
		var (
			h      *protocol.Header
			ah     *protocol.AddHeader
			buffer []byte
			err    error
		)

		//读固定头
		if buffer, err = readTimeout(conn, protocol.FIX_HEADER_LEN, 0); err != nil {
			fmt.Println(err)
			return
		}

		//解析固定头
		if h, err = protocol.DecodeHeader(buffer); err != nil {
			log.Fatal(conn.RemoteAddr().String(), " DecodeHeader error: ", err)
			return
		}

		//校验消息类型
		msgType := protocol.GetMsgType(h)
		if msgType == protocol.MSGTYPE_DEFAULT || msgType >= protocol.MSGTYPE_MAX {
			log.Fatal(conn.RemoteAddr().String(), " Error msg type: ", msgType)
			return
		}

		//心跳消息无消息体，特殊处理
		//if msgType == protocol.MSGTYPE_HEARTBEAT || msgType == protocol.MSGTYPE_HBRESP {
		if msgType == protocol.MSGTYPE_HEARTBEAT {
			ch <- h
			continue
		}

		//读取附加头
		if buffer, err = readTimeout(conn, protocol.ADD_HEADER_LEN, default_read_timeout); err != nil {
			log.Fatal("read error on add header ", err)
			return
		}

		//解析附加头
		if ah, err = protocol.DecodeAddHeader(buffer); err != nil {
			log.Fatal(conn.RemoteAddr().String(), " DecodeAddHeader error: ", err)
			return
		}

		switch msgType {
		//应答
		case protocol.MSGTYPE_REGRESP, protocol.MSGTYPE_HBRESP, protocol.MSGTYPE_MSGRESP:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamResp
			if param, err = protocol.DecodeParamResp(buffer); err != nil {
				log.Fatal(conn.RemoteAddr().String(), " DecodeParamResp error: ", err)
				return
			}

			var msg protocol.Resp
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamResp = *param
			ch <- &msg

		//PUSH
		case protocol.MSGTYPE_PUSH:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			//解析body
			var param *protocol.ParamPush
			if param, err = protocol.DecodeParamPush(buffer); err != nil {
				log.Fatal(conn.RemoteAddr().String(), " DecodeParamPush error: ", err)
				return
			}

			var msg protocol.Push
			msg.Header = *h
			msg.AddHeader = *ah
			msg.ParamPush = *param
			msg.Msg = buffer[3:]
			fmt.Println(param.Offline, param.Flag, string(buffer[3:]))
			ch <- &msg
		//CALLBACK
		case protocol.MSGTYPE_CALLBACK:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			var msg protocol.Callback
			msg.Header = *h
			msg.AddHeader = *ah
			msg.Msg = buffer
			ch <- &msg
		//IM
		case protocol.MSGTYPE_MESSAGE:
			//读取body
			var buffer []byte
			if buffer, err = readTimeout(conn, ah.Len, default_read_timeout); err != nil {
				return
			}

			protocol.CodecDecode(buffer, int(ah.Len), protocol.GetEncode(h))

			var msg protocol.Im
			msg.Header = *h
			msg.AddHeader = *ah
			msg.Msg = buffer
			ch <- &msg
		default:
			log.Fatal(conn.RemoteAddr().String(), " Error MsgType: ", msgType)
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
		log.Fatal(conn.RemoteAddr().String(), " connection Read error: ", err)
	}

	fmt.Println("buffer=", buffer)
	return
}
