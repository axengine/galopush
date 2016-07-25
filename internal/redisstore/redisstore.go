// redisstore 实现离线消息的redis存储与获取
package redisstore

import (
	"encoding/json"
	"galopush/internal/logs"

	"github.com/hoisie/redis"
)

type offMsg struct {
	Id   string `json:"id"`
	Body body   `json:"body"`
}

type body struct {
	Push     push   `json:"push"`
	Callback []item `json:"callback"`
	Im       []item `json:"im"`
}

type push struct {
	Offline uint16 `json:"offline"`
	Msg     []byte `json:"msg"`
}

type item struct {
	Plat int    `json:"plat"`
	Msg  []byte `json:"msg"`
}

type Storager struct {
	cli *redis.Client
}

func NewStorager(dial, pswd string, db int) *Storager {
	var store Storager
	var client redis.Client
	client.Addr = dial
	client.Password = pswd
	client.Db = db
	client.MaxPoolSize = 10
	store.cli = &client
	return &store
}

func (p *Storager) SavePushMsg(id string, msg []byte) error {
	logs.Logger.Debug("[Storager] SavePushMsg id=", id, " msg=", string(msg[:]))
	b, err := p.cli.Exists(id)
	if err != nil {
		logs.Logger.Error(err)
		return err
	}
	var omsg offMsg
	switch b {
	case true:
		body, err := p.cli.Get(id)
		if err != nil {
			logs.Logger.Error(err)
			return err
		}
		json.Unmarshal(body, &omsg)
		omsg.Body.Push.Offline = omsg.Body.Push.Offline + 1
		omsg.Body.Push.Msg = make([]byte, 0)
		omsg.Body.Push.Msg = msg
	case false:
		omsg.Body.Push.Offline = omsg.Body.Push.Offline + 1
		omsg.Body.Push.Msg = make([]byte, 0)
		omsg.Body.Push.Msg = msg
	}
	by, _ := json.Marshal(&omsg)
	err = p.cli.Set(id, by)
	return err
}

func (p *Storager) SaveCallbackMsg(id string, plat int, msg []byte) error {
	logs.Logger.Debug("[Storager] SaveCallbackMsg id=", id, " plat=", plat, " msg=", string(msg[:]))
	b, err := p.cli.Exists(id)
	if err != nil {
		logs.Logger.Error(err)
		return err
	}
	var omsg offMsg
	switch b {
	case true:
		body, err := p.cli.Get(id)
		if err != nil {
			logs.Logger.Error(err)
			return err
		}
		json.Unmarshal(body, &omsg)
		var it item
		it.Msg = append(it.Msg, msg...)
		it.Plat = plat
		omsg.Body.Callback = append(omsg.Body.Callback, it)
	case false:
		var it item
		it.Msg = append(it.Msg, msg...)
		it.Plat = plat
		omsg.Body.Callback = append(omsg.Body.Callback, it)
	}
	by, _ := json.Marshal(&omsg)
	if err = p.cli.Set(id, by); err != nil {
		logs.Logger.Error(err)
	}
	return err
}

func (p *Storager) SaveImMsg(id string, plat int, msg []byte) error {
	logs.Logger.Debug("[Storager] SaveImMsg id=", id, " plat=", plat, " msg=", string(msg[:]))
	b, err := p.cli.Exists(id)
	if err != nil {
		logs.Logger.Error(err)
		return err
	}
	var omsg offMsg
	switch b {
	case true:
		body, err := p.cli.Get(id)
		if err != nil {
			logs.Logger.Error(err)
			return err
		}
		json.Unmarshal(body, &omsg)
		var it item
		it.Msg = append(it.Msg, msg...)
		it.Plat = plat
		omsg.Body.Im = append(omsg.Body.Im, it)
	case false:
		var it item
		it.Msg = append(it.Msg, msg...)
		it.Plat = plat
		omsg.Body.Im = append(omsg.Body.Im, it)
	}
	by, _ := json.Marshal(&omsg)
	if err = p.cli.Set(id, by); err != nil {
		logs.Logger.Error(err)
	}
	return err
}

func (p *Storager) GetPushMsg(id string) (uint16, []byte) {
	var offCount uint16
	var buff []byte

	var omsg offMsg
	body, err := p.cli.Get(id)
	if err != nil {
		logs.Logger.Error(err)
		return offCount, buff
	}
	json.Unmarshal(body, &omsg)
	offCount = omsg.Body.Push.Offline
	buff = append(buff, omsg.Body.Push.Msg...)

	//重置消息
	omsg.Body.Push.Offline = 0
	omsg.Body.Push.Msg = make([]byte, 0)
	by, _ := json.Marshal(&omsg)
	if err = p.cli.Set(id, by); err != nil {
		logs.Logger.Error(err)
	}
	return offCount, buff
}

func (p *Storager) GetCallbackMsg(id string, plat int) [][]byte {
	var buff [][]byte

	var omsg offMsg
	body, err := p.cli.Get(id)
	if err != nil {
		logs.Logger.Error(err)
		return buff
	}
	json.Unmarshal(body, &omsg)
	var newItem []item
	for _, v := range omsg.Body.Callback {
		var it item
		it = v
		if it.Plat == plat {
			buff = append(buff, it.Msg)
		} else {
			newItem = append(newItem, it)
		}
	}

	//重置消息
	omsg.Body.Callback = newItem
	by, _ := json.Marshal(&omsg)
	if err = p.cli.Set(id, by); err != nil {
		logs.Logger.Error(err)
	}
	return buff
}

func (p *Storager) GetImMsg(id string, plat int) [][]byte {
	var buff [][]byte
	var omsg offMsg
	body, err := p.cli.Get(id)
	if err != nil {
		logs.Logger.Error(err)
		return buff
	}
	json.Unmarshal(body, &omsg)
	var newItem []item
	for _, v := range omsg.Body.Im {
		var it item
		it = v
		if it.Plat == plat {
			buff = append(buff, it.Msg)
		} else {
			newItem = append(newItem, it)
		}
	}

	//重置消息
	omsg.Body.Im = newItem
	by, _ := json.Marshal(&omsg)
	if err = p.cli.Set(id, by); err != nil {
		logs.Logger.Error(err)
	}
	return buff
}
