package main

//终端类型枚举
const (
	PLAT_DEFAULT  = 0
	PLAT_ANDROID  = 1
	PLAT_IOS      = 2
	PLAT_WINPHONE = 4
	PLAT_WEB      = 8
	PLAT_PC       = 16
	PLAT_ALL      = 31
)

type UserOnlineState struct {
	Uid         string `json:"uid"`
	Termtype    int    `json:"termtype"`
	Code        string `json:"code"`
	DeviceToken string `json:"deviceToken"`
	Login       bool   `json:"online"`
}

type SessionTimeout struct {
	BackTopic string  `json:"backTopic"`
	Key       string  `json:"key"`
	Sess      Session `json:"msg"`
}

type Session struct {
	Termtype  int    `json:"ctype"`
	Uid       string `json:"uid"`
	SessionId string `json:"sessionId"`
	Flag      int    `json:"flag"`
}

type MsgDownward struct {
	Receivers []Receiver `json:"receivers"`
	Flag      int        `json:"flag"`
	Body      string     `json:"body"`
}

type Receiver struct {
	Uid      string `json:"uid"`
	Termtype int    `json:"termtype"`
}

type MsgUpward struct {
	Uid      string `json:"uid"`
	Termtype int    `json:"termtype"`
	Body     string `json:"body"`
}
