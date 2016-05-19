package protocol

import (
	"encoding/base64"
	"encoding/json"
)

type JsonData struct {
	Cmd    int `json:"cmd"`
	EnCode int `json:"enCode"`
	Tid    int `json:"tid"`
	//Data   interface{} `json:"data"`
	Data string `json:"data"`
}

type JsonRegiter struct {
	Version      int    `json:"version"`
	TerminalType int    `json:"termType"`
	Id           string `json:"id"`
	Token        string `json:"token"`
}

type JsonPush struct {
	Msg string `json:"msg"`
}

type JsonResp struct {
	Code int `json:"code"`
}

func packJson(data interface{}) []byte {
	var buf []byte
	switch data.(type) {
	case *Push:
		msg := data.(*Push)
		var data JsonData
		data.Cmd = GetMsgType(&msg.Header)
		data.EnCode = GetEncode(&msg.Header)
		data.Tid = int(msg.Tid)

		//先把消息体编码为json
		var param JsonPush
		param.Msg = base64.StdEncoding.EncodeToString(msg.Msg)
		b, _ := json.Marshal(&param)

		//把消息体按照encode进行加密
		CodecEncode(b, len(b), data.EnCode)

		//消息体再base64加密
		if data.EnCode > 0 {
			data.Data = base64.StdEncoding.EncodeToString(b)
		} else {
			data.Data = string(b[:])
		}

		//将整体消息编码为json
		buf, _ := marshal(&data)
		return buf
	case *Callback:
		msg := data.(*Callback)
		var data JsonData
		data.Cmd = GetMsgType(&msg.Header)
		data.EnCode = GetEncode(&msg.Header)
		data.Tid = int(msg.Tid)

		//先把消息体编码为json
		var param JsonPush
		param.Msg = base64.StdEncoding.EncodeToString(msg.Msg)
		b, _ := json.Marshal(&param)

		//把消息体按照encode进行加密
		CodecEncode(b, len(b), data.EnCode)

		//消息体再base64加密
		if data.EnCode > 0 {
			data.Data = base64.StdEncoding.EncodeToString(b)
		} else {
			data.Data = string(b[:])
		}

		//将整体消息编码为json
		buf, _ := marshal(&data)
		return buf
	case *Im:
		msg := data.(*Im)
		var data JsonData
		data.Cmd = GetMsgType(&msg.Header)
		data.EnCode = GetEncode(&msg.Header)
		data.Tid = int(msg.Tid)

		//先把消息体编码为json
		var param JsonPush
		param.Msg = base64.StdEncoding.EncodeToString(msg.Msg)
		b, _ := json.Marshal(&param)

		//把消息体按照encode进行加密
		CodecEncode(b, len(b), data.EnCode)

		//消息体再base64加密
		if data.EnCode > 0 {
			data.Data = base64.StdEncoding.EncodeToString(b)
		} else {
			data.Data = string(b[:])
		}

		//将整体消息编码为json
		buf, _ := marshal(&data)
		return buf
	case *Resp:
		msg := data.(*Resp)
		var data JsonData
		data.Cmd = GetMsgType(&msg.Header)
		data.EnCode = GetEncode(&msg.Header)
		data.Tid = int(msg.Tid)

		var param JsonResp
		param.Code = int(msg.Code)
		b, _ := json.Marshal(&param)
		CodecEncode(b, len(b), data.EnCode)

		//消息体再base64加密
		if data.EnCode > 0 {
			data.Data = base64.StdEncoding.EncodeToString(b)
		} else {
			data.Data = string(b[:])
		}
		buf, _ := marshal(&data)
		return buf

	default:
	}

	return buf
}

func marshal(data interface{}) ([]byte, error) {
	b, err := json.Marshal(data)
	return b, err
}
