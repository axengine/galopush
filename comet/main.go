// gComet project main.go
package main

import (
	"galopush/logs"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/judwhite/go-svc/svc"
	"github.com/widuu/goini"
)

var Debug = logs.Logger.Debug
var Error = logs.Logger.Error
var Critical = logs.Logger.Critical

type program struct {
	comet *Comet
}

func main() {
	defer func() {
		logs.Logger.Flush()
		if r := recover(); r != nil {
			logs.Logger.Error("main.recover:", r)
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	go func() {
		conf := goini.SetConfig("./config.ini")
		bindAddr := conf.GetValue("http", "bindAddr")
		http.ListenAndServe(bindAddr, nil)
	}()
	prg := program{
		comet: &Comet{},
	}
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
