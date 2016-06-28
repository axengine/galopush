package main

import (
	"fmt"
	"galopush/logs"
	"galopush/protocol"
	"net/http"
	_ "net/http/pprof"
)

func (p *Router) startHttpServer() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("startHttpServer.recover:", r)
			go p.startHttpServer()
		}
	}()
	go func() {
		http.HandleFunc("/v1/gComet.addr", func(w http.ResponseWriter, r *http.Request) {
			p.loadDispatcher(w, r)
		})
		err := http.ListenAndServe(p.httpBindAddr, nil)
		if err != nil {
			panic(err)
		}
	}()
}

func (p *Router) loadDispatcher(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method != "POST" {
		return
	}

	id := r.FormValue("id")
	platS := r.FormValue("termtype")
	var plat int
	if platS == "Android" {
		plat = protocol.PLAT_ANDROID
	} else if platS == "iOS" {
		plat = protocol.PLAT_IOS
	} else if platS == "Web" {
		plat = protocol.PLAT_WEB
	} else if platS == "WinPhone" {
		plat = protocol.PLAT_WINPHONE
	} else if platS == "PC" {
		plat = protocol.PLAT_PC
	}
	addr := p.balancer(id, plat)
	logs.Logger.Debug("[http] load dispatcher id=", id, " plat=", platS, " addr=", addr)

	//for ajax cross domain
	Origin := r.Header.Get("Origin")
	if Origin != "" {
		w.Header().Add("Access-Control-Allow-Origin", Origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,GET,OPTIONS,DELETE")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,content-type")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
	}

	fmt.Fprintf(w, addr)
}

//balancer HTTP接口 根据id和plat返回socket地址
func (p *Router) balancer(id string, plat int) string {
	s := p.pool.findSessions(id)
	if s != nil {
		c := p.pool.findComet(s.cometId)
		if c != nil {
			if plat == protocol.PLAT_WEB {
				return c.wsAddr
			} else {
				return c.tcpAddr
			}
		}
	}
	//系统指配
	c := p.pool.balancer()
	if c != nil {
		if plat == 8 {
			return c.wsAddr
		} else {
			return c.tcpAddr
		}
	}
	return ""
}
