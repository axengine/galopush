// route project main.go
package main

import (
	"galopush/internal/logs"
	"log"
	"runtime"

	"github.com/judwhite/go-svc/svc"
)

type program struct {
	router *Router
}

func main() {
	defer func() {
		logs.Logger.Flush()
		if r := recover(); r != nil {
			logs.Logger.Error("main.recover:", r)
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	prg := program{
		router: &Router{},
	}
	if err := svc.Run(prg); err != nil {
		log.Fatal(err)
	}
}

func (p program) Init(env svc.Environment) error {
	p.router.Init()
	return nil
}

func (p program) Start() error {
	go p.router.Start()
	return nil
}

func (p program) Stop() error {
	if err := p.router.Stop(); err != nil {
		return err
	}
	return nil
}
