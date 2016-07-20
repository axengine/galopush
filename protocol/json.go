package protocol

import (
	//	"encoding/base64"
	"encoding/json"
	"galopush/logs"
)

func packJson(v interface{}) []byte {
	var buf []byte
	var err error
	var body map[string]interface{}
	body = make(map[string]interface{})
	var data map[string]interface{}
	data = make(map[string]interface{})
	switch v.(type) {
	case *Push:
		msg := v.(*Push)
		body["cmd"] = GetMsgType(&msg.Header)
		body["tid"] = msg.Tid
		data["msg"] = string(msg.Msg)
		body["data"] = data
		buf, err = json.Marshal(&body)
		if err != nil {
			logs.Logger.Error(err)
		}
	case *Callback:
		msg := v.(*Callback)
		body["cmd"] = GetMsgType(&msg.Header)
		body["tid"] = msg.Tid
		data["msg"] = string(msg.Msg)
		body["data"] = data
		buf, err = json.Marshal(&body)
		if err != nil {
			logs.Logger.Error(err)
		}
	case *ImDown:
		msg := v.(*ImDown)
		body["cmd"] = GetMsgType(&msg.Header)
		body["tid"] = msg.Tid
		data["msg"] = string(msg.Msg)
		body["data"] = data
		buf, err = json.Marshal(&body)
		if err != nil {
			logs.Logger.Error(err)
		}
	case *Resp:
		msg := v.(*Resp)
		body["cmd"] = GetMsgType(&msg.Header)
		body["tid"] = msg.Tid
		data["code"] = int(msg.Code)
		body["data"] = data
		buf, err = json.Marshal(&body)
		if err != nil {
			logs.Logger.Error(err)
		}
	case *Kick:
		msg := v.(*Kick)
		body["cmd"] = GetMsgType(&msg.Header)
		body["tid"] = msg.Tid
		data["reson"] = int(msg.Reason)
		body["data"] = data
		buf, err = json.Marshal(&body)
		if err != nil {
			logs.Logger.Error(err)
		}
	}
	logs.Logger.Debug("packJson=", string(buf[:]))
	return buf
}

func marshal(data interface{}) ([]byte, error) {
	b, err := json.Marshal(data)
	return b, err
}
