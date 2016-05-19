package main

import (
	"galopush/logs"
	"time"
)

func (p *Comet) stat() {
	t := time.NewTicker(time.Second * 60)
	for {
		select {
		case <-t.C:
			logs.Logger.Debug(p.cnt.String())
		}
	}
}
