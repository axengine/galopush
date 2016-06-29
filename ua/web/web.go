package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"galopush/protocol"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.google.com/p/go.net/websocket"
)

var (
	tid int
	ch  chan interface{}
)

const (
	userid               = "13996130360"
	token                = "000000"
	gencode              = protocol.ENCODE_DEFAULT
	loadblancer          = "http://192.168.1.63:5150/v1/gComet.addr"
	default_read_timeout = 5
)

func main() {
	cometAddr, err := getCometAddr()
	if err != nil {
		fmt.Println(err)
		return
	}
	url := fmt.Sprintf("ws://%s/ws", cometAddr)
	fmt.Println("comet addr=", url)
	origin := "http://localhost/"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP)
	go func(conn *websocket.Conn) {
		select {
		case <-signalChan:
			message(conn, "this is a im msg which send by web client")
		}
	}(ws)
	go readRuntime(ws)
	register(ws)
	select {}
}

func readRuntime(ws *websocket.Conn) {
	for {
		var buf []byte
		err := websocket.Message.Receive(ws, &buf)
		if err != nil {
			log.Fatal(err)
			return
		}

		if err = unPack(ws, buf); err != nil {
			log.Fatal(err)
			return
		}
	}
}
func unPack(conn *websocket.Conn, buffer []byte) error {
	var data protocol.JsonData
	if err := json.Unmarshal(buffer, &data); err != nil {
		return err
	}
	fmt.Println("receive msgType=", data.Cmd, " encode=", data.EnCode, " tid=", data.Tid, "data=", data.Data)
	switch data.Cmd {
	case protocol.MSGTYPE_REGRESP, protocol.MSGTYPE_HBRESP, protocol.MSGTYPE_MSGRESP:
		if data.Cmd == protocol.MSGTYPE_REGRESP {
			go heartbeat(conn)
		}
	case protocol.MSGTYPE_PUSH, protocol.MSGTYPE_CALLBACK, protocol.MSGTYPE_MESSAGE:
		var x protocol.JsonPush
		//将消息体转为字符数组
		var b []byte
		if data.EnCode > 0 {
			b, _ = base64.StdEncoding.DecodeString(data.Data)
			protocol.CodecDecode(b, len(b), data.EnCode)
		} else {
			b = []byte(data.Data)
		}
		if err := json.Unmarshal(b, &x); err != nil {
			log.Fatal("json unmarshal error ", err)
		}

		if b, err := base64.StdEncoding.DecodeString(x.Msg); err != nil {
			log.Fatal(err)
		} else {
			fmt.Println("recieve ", data.Cmd, "  msg:", string(b[:]))
		}

	default:
		return errors.New("error msg type")
	}
	return nil
}
func getCometAddr() (string, error) {
	var addr string
	resp, err := http.PostForm(loadblancer,
		url.Values{"id": {userid}, "termtype": {"Web"}})
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

func register(ws *websocket.Conn) {
	//构造参数
	var param protocol.JsonRegiter
	param.Version = 0x01
	param.TerminalType = 8
	param.Id = userid
	param.Token = token

	paramBuff, err := json.Marshal(&param)
	if err != nil {
		log.Fatal(err)
	}

	//构造全部消息
	var msg protocol.JsonData
	msg.Cmd = protocol.MSGTYPE_REGISTER
	msg.Tid = nextTid()
	msg.EnCode = gencode

	var paramStr string
	if gencode > 0 {
		protocol.CodecEncode(paramBuff, len(paramBuff), gencode)
		paramStr = base64.StdEncoding.EncodeToString(paramBuff)
	} else {
		paramStr = string(paramBuff[:])
	}
	msg.Data = paramStr
	buff, err := json.Marshal(&msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("write:", string(buff[:]))
	_, err = ws.Write(buff)
	if err != nil {
		log.Fatal(err)
	}
}
func heartbeat(ws *websocket.Conn) {
	t1 := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-t1.C:
			ping(ws)
		}
	}
}
func ping(ws *websocket.Conn) {
	//构造全部消息
	var msg protocol.JsonData
	msg.Cmd = protocol.MSGTYPE_HEARTBEAT
	msg.Tid = nextTid()
	msg.EnCode = gencode

	buff, err := json.Marshal(&msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("write:", string(buff[:]))
	_, err = ws.Write(buff)
	if err != nil {
		log.Fatal(err)
	}
}
func nextTid() int {
	tid = tid + 1
	return tid
}
func message(ws *websocket.Conn, body string) {
	//构造参数
	var param protocol.JsonPush
	param.Msg = body

	paramBuff, err := json.Marshal(&param)
	if err != nil {
		log.Fatal(err)
	}

	//构造全部消息
	var msg protocol.JsonData
	msg.Cmd = protocol.MSGTYPE_MESSAGE
	msg.Tid = nextTid()
	msg.EnCode = gencode

	var paramStr string
	if gencode > 0 {
		protocol.CodecEncode(paramBuff, len(paramBuff), gencode)
		paramStr = base64.StdEncoding.EncodeToString(paramBuff)
	} else {
		paramStr = string(paramBuff[:])
	}
	msg.Data = paramStr
	buff, err := json.Marshal(&msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("write:", string(buff[:]))
	_, err = ws.Write(buff)
	if err != nil {
		log.Fatal(err)
	}
}
