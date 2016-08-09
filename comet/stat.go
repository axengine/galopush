package main

import (
	"galopush/internal/logs"
	"time"
)

func (p *Comet) stat() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	t := time.NewTicker(time.Second * 60)
	for {
		select {
		case <-t.C:
			logs.Logger.Debug(p.cnt.String())
		}
	}
}
