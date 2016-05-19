//rpc 提供comet to router，router to comet的RPC调用
package rpc

import (
	"errors"
)

var (
	ERROR_RESPONSE = errors.New("error response code")
)

const (
	RPC_RET_SUCCESS = 0
	RPC_RET_FAILED  = -1
)

const (
	MSG_TYPE_PUSH     = 1
	MSG_TYPE_CALLBACK = 2
	MSG_TYPE_IM       = 3
)

const (
	STATE_OFFLINE = 0
	STATE_ONLINE  = 1
)

//调用方向:router->comet
//push
//callback
//im
type Push struct{}

//router向comet发起业务请求，comet的业务数据直接走nsqd
type PushRequst struct {
	Tp  int //消息类型
	Id  string
	Msg []byte
}

//调用方向:comet->router
//comet登记
//用户状态通知
//心跳检测
type Login struct{}

//comet 报告用户在线、离线状态
type StateRequst struct {
	CometId  string
	Id       string
	Termtype int
	State    int //1-online 0-offline
}

//comet启动时报告自身状态
type LoadRequst struct {
	CometId string //comet name
	TcpAddr string //comet对外开放tcp服务地址
	WsAddr  string //comet对外开放ws服务地址
	RpcAddr string //反连地址(comet rpc服务地址)
}

//公共心跳请求
type PingRequest struct {
	TimeStamp int
}

//公共应答
type Response struct {
	Code int
}
