package rpc

import (
	"net"
	"net/http"
	"net/rpc"
)

type RpcServer struct {
	dataChan    chan interface{}
	syncHandle  func(interface{}) int //同步接口 处理comet注册
	asyncHandle func(interface{})     //异步接口 处理业务数据
}

//NewRpcServer return a *RpcServer
func NewRpcServer(addr string, cache int, syncHandle func(interface{}) int, asyncHandle func(interface{})) *RpcServer {
	s := &RpcServer{
		syncHandle:  syncHandle,
		asyncHandle: asyncHandle,
		dataChan:    make(chan interface{}, cache),
	}

	if err := s.startRpcServer(addr); err != nil {
		panic(err)
	}
	go s.handleMessage()
	return s
}

func (p *RpcServer) handleMessage() {
	for {
		select {
		case m := <-p.dataChan:
			p.asyncHandle(m)
		}
	}
}

func (p *RpcServer) startRpcServer(addr string) error {
	var err error
	var l net.Listener

	rpc.Register(p)

	rpc.HandleHTTP()
	if l, err = net.Listen("tcp", addr); err != nil {
		return err
	}
	go http.Serve(l, nil)
	return err
}

func (p *RpcServer) Push(request *PushRequst, response *Response) error {
	p.dataChan <- request
	response.Code = 0
	return nil
}

func (p *RpcServer) Ping(request *PingRequest, response *Response) error {
	response.Code = 0
	return nil
}

func (p *RpcServer) State(request *StateRequst, response *Response) error {
	p.dataChan <- request
	response.Code = 0
	return nil
}

func (p *RpcServer) Register(request *LoadRequst, response *Response) error {
	response.Code = p.syncHandle(request)
	return nil
}
