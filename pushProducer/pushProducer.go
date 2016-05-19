package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	RUNTIME_NUM = 1
	MSG_NUM     = 1
)

func main() {
	begin := time.Now().Unix()
	var ch chan int
	ch = make(chan int, RUNTIME_NUM)
	for i := 0; i < RUNTIME_NUM; i++ {
		go pushTest(ch, i)
	}
	for i := 0; i < RUNTIME_NUM; i++ {
		<-ch
	}
	end := time.Now().Unix()
	fmt.Println("exit ", end-begin)

}

func pushTest(ch chan int, i int) {
	for i := 0; i < MSG_NUM; i++ {
		var b []byte
		var err error
		var resp *http.Response
		msg := fmt.Sprintf("i am a push msg with id=%d", i)
		if resp, err = http.Post("http://192.168.1.63:4151/put?topic=push", "text/plain", strings.NewReader(msg)); err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()
		if b, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Println(err)
		}
		fmt.Println(i, " = ", string(b[:]))
	}
	ch <- 1
	fmt.Println("pushTest ok")
}
