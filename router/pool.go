package main

/**
* 用户池后期改造为redis或者mongodb持久化
 */

import (
	"galopush/internal/rpc"
	"sync"
)

type Pool struct {
	m1     sync.Mutex
	comets map[string]*comet
	//m2       sync.Mutex
	//sessions map[string]*session
}

//用户session
type session struct {
	id      string
	cometId string //用户所附着的comet
	item    []*item
}

type item struct {
	plat        int    //终端类型
	online      bool   //推送接口是否在线
	authCode    string //业务层鉴权码
	login       bool   //业务层是否已经登录
	deviceToken string //苹果设备token
}

type comet struct {
	id        string         //comet id
	rpcClient *rpc.RpcClient //router连接到本comet的RPC客户端句柄
	tcpAddr   string         //comet对外开放tcp服务地址
	wsAddr    string         //comet对外开放ws服务地址
	online    int            //comet在线统计
	ch        chan int       //comet rpc 状态通知chan
}

func (p *Pool) insertComet(id string, c *comet) {
	p.m1.Lock()
	defer p.m1.Unlock()
	p.comets[id] = c
}

func (p *Pool) findComet(id string) *comet {
	p.m1.Lock()
	defer p.m1.Unlock()
	c := p.comets[id]
	return c
}

func (p *Pool) deleteComet(id string) {
	p.m1.Lock()
	defer p.m1.Unlock()
	delete(p.comets, id)
}

func (p *Pool) cometAdd(id string) {
	p.m1.Lock()
	defer p.m1.Unlock()
	c := p.comets[id]
	if c != nil {
		c.online = c.online + 1
	}
}

func (p *Pool) cometSub(id string) {
	p.m1.Lock()
	defer p.m1.Unlock()
	c := p.comets[id]
	if c != nil {
		c.online = c.online - 1
	}
}

//选择负载最低的comet
func (p *Pool) balancer() *comet {
	p.m1.Lock()
	defer p.m1.Unlock()
	minLoad := 0
	var c *comet
	for _, v := range p.comets {
		if minLoad == 0 {
			minLoad = v.online
			c = v
		}
		if v.online < minLoad {
			minLoad = v.online
			c = v
		}
	}
	return c
}

/*
func (p *Pool) insertSessions(id string, s *session) {
	p.m2.Lock()
	defer p.m2.Unlock()
	p.sessions[id] = s
}

func (p *Pool) findSessions(id string) *session {
	p.m2.Lock()
	defer p.m2.Unlock()
	s := p.sessions[id]
	return s
}

func (p *Pool) deleteSessions(id string) {
	p.m2.Lock()
	defer p.m2.Unlock()
	delete(p.sessions, id)
}

//deleteSessionsWithCometId 指定cometid的所有用户下线
func (p *Pool) deleteSessionsWithCometId(id string) {
	p.m2.Lock()
	defer p.m2.Unlock()
	for _, v := range p.sessions {
		if v.cometId == id {
			for _, it := range v.item {
				it.online = false
			}
		}
	}
}*/
