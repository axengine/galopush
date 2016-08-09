// gComet project main.go
package main

import (
	"galopush/internal/logs"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/judwhite/go-svc/svc"
	"github.com/widuu/goini"
)

type program struct {
	comet *Comet
}

var gsComet *Comet

func main() {
	defer func() {
		logs.Logger.Flush()
		if r := recover(); r != nil {
			logs.Logger.Error("main.recover:", r)
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logs.Logger.Error("recover ", r)
			}
		}()
		conf := goini.SetConfig("./config.ini")
		bindAddr := conf.GetValue("http", "bindAddr")
		http.ListenAndServe(bindAddr, nil)
	}()
	prg := program{
		comet: &Comet{},
	}
	gsComet = prg.comet
	if err := svc.Run(prg); err != nil {
		log.Fatal(err)
	}
}

func (p program) Init(env svc.Environment) error {
	p.comet.Init()
	return nil
}

func (p program) Start() error {
	go p.comet.Start()
	return nil
}

func (p program) Stop() error {
	if err := p.comet.Stop(); err != nil {
		return err
	}
	return nil
}
