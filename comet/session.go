package main

import (
	"sync"
	"time"
)

type Pool struct {
	//id为识别符 由用户id+plat唯一指定
	mutex1   sync.Mutex
	ids      map[string]string //<addr-id>
	mutex2   sync.Mutex
	sessions map[string]*session //<id-session>
}

func (p *Pool) insertIds(addr, id string) {
	p.mutex1.Lock()
	defer p.mutex1.Unlock()
	p.ids[addr] = id
}

func (p *Pool) deleteIds(addr string) {
	p.mutex1.Lock()
	defer p.mutex1.Unlock()
	delete(p.ids, addr)
}

func (p *Pool) findId(addr string) string {
	p.mutex1.Lock()
	defer p.mutex1.Unlock()
	v, _ := p.ids[addr]
	return v
}

func (p *Pool) insertSessions(id string, sess *session) {
	p.mutex2.Lock()
	defer p.mutex2.Unlock()
	p.sessions[id] = sess
}

func (p *Pool) deleteSessions(id string) {
	p.mutex2.Lock()
	defer p.mutex2.Unlock()
	delete(p.sessions, id)
}

func (p *Pool) findSessions(id string) *session {
	p.mutex2.Lock()
	defer p.mutex2.Unlock()
	v, _ := p.sessions[id]
	return v
}

type session struct {
	id     string
	plat   int
	conn   interface{}
	encode int

	tid   int
	mutex sync.Mutex
	trans []*transaction
}

type transaction struct {
	tid     int
	msgType int
	timer   *time.Timer
	exit    chan int
	msg     []byte
}

func newTransaction() *transaction {
	var t transaction          //mem leak
	t.exit = make(chan int, 1) //mem leak
	t.msg = make([]byte, 0)
	t.timer = time.NewTimer(time.Second * 5) //mem leak
	return &t
}

var (
	MaxUint32 = 1<<32 - 1
)

func (p *session) nextTid() int {
	p.tid = p.tid + 1
	if p.tid >= MaxUint32 {
		p.tid = 1
	}
	return p.tid
}

func (p *session) destroy() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, t := range p.trans {
		t.timer.Stop()
	}
}

func (p *session) checkTrans(t *transaction) {
	var tm *time.Timer
	tm = t.timer
	go func() {
		for {
			select {
			case <-t.exit: //事务退出
				p.delTrans(t.tid)
				return
			case <-tm.C:
				//time out to save message
				//store.Store(this.Uid, t.Msg)
				t.exit <- 1
			}
		}
	}()
}

func (p *session) saveTransaction(t *transaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.trans = append(p.trans, t)
}

func (p *session) getTrans(tid int) *transaction {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	var trans *transaction
	for _, t := range p.trans {
		if t.tid == tid {
			trans = t
			return trans
		}
	}
	return trans
}

func (p *session) delTrans(tid int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for i, t := range p.trans {
		if t.tid == tid {
			t.timer.Stop()
			p.trans = append(p.trans[:i], p.trans[i+1:]...)
			break
		}
	}
}
