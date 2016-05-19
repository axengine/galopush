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
	fmt.Fprintf(w, addr)
}
