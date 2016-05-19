package main

import (
	"fmt"
	"galopush/mgostore"
	"galopush/redisstore"
	"time"
)

func main() {
	testMgoStore()
	testRedisStore()
}

func testMgoStore() {
	db := mgostore.NewStorager("115.28.128.9:27017", "gservice", "offmsg")
	if db == nil {
		panic("connect to mstore error")
	}
	msg := []byte("hello world,I am a PushMsg")
	begin := time.Now().UnixNano()
	for i := 0; i < 10000; i++ {
		if err := db.SavePushMsg("testid", msg); err != nil {
			fmt.Println(err)
		}
		db.GetPushMsg("testid")
		//n, m := db.GetPushMsg("testid")
		//fmt.Println(n, string(m[:]))
	}
	end := time.Now().UnixNano()
	fmt.Println("testMgoStore use ", (end-begin)/1000/1000, " ms")
}

func testRedisStore() {
	db := redisstore.NewStorager("115.28.128.9:6379", "12345678", 0)
	if db == nil {
		panic("connect to redisstore error")
	}
	msg := []byte("hello world,I am a PushMsg")
	begin := time.Now().UnixNano()
	for i := 0; i < 10000; i++ {
		if err := db.SavePushMsg("testid", msg); err != nil {
			fmt.Println(err)
		}
		db.GetPushMsg("testid")
		//n, m := db.GetPushMsg("testid")
		//fmt.Println(n, string(m[:]))
	}
	end := time.Now().UnixNano()
	fmt.Println("testRedisStore use ", (end-begin)/1000/1000, " ms")
}
