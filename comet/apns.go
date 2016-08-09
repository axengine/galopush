package main

import (
	"errors"
	"fmt"
	"galopush/internal/logs"
	"strconv"

	"encoding/json"

	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	pl "github.com/sideshow/apns2/payload"
	"github.com/widuu/goini"
)

var (
	isDevelopment int
	pemFile       string
	password      string
)

var client *apns.Client
var iOSSound []string

func init() {
	conf := goini.SetConfig("./config.ini")
	sDev := conf.GetValue("apns", "development")
	isDevelopment, _ = strconv.Atoi(sDev)

	pemFile = conf.GetValue("apns", "pemFile")
	password = conf.GetValue("apns", "password")

	cert, pemErr := certificate.FromP12File(pemFile, password)
	if pemErr != nil {
		logs.Logger.Error(pemErr)
		return
	}
	var cli *apns.Client
	if isDevelopment == 1 {
		cli = apns.NewClient(cert).Development()
	} else {
		cli = apns.NewClient(cert).Production()
	}
	iOSSound = []string{"default.mp3", "chatnotify.mp3", "tasknotify.mp3"}
	client = cli
}

func apnsPush(uid, token, title, body string, flag int) error {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover from apnsPush ", r)
		}
	}()
	logs.Logger.Debug("[APNS] to ", token, " title=", title, " body=", body, " flag=", flag)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		logs.Logger.Error("[APNS] error" + err.Error())
		return err
	}

	msgId, _ := m["MSG00101"].(string)
	content, _ := m["MSG00109"].(string)
	sender, _ := m["MSG00103"].(string)
	receiver, _ := m["MSG00105"].(string)
	sType, _ := m["MSG00107"].(float64)
	sessType := int(sType)

	if sessType == 1 {
		title = sender
	} else {
		title = receiver
		content = sender + ":" + content
	}

	notification := &apns.Notification{}
	notification.DeviceToken = token
	p := pl.NewPayload().Badge(1).AlertTitle(title).AlertBody(content).Sound(iOSSound[flag]).Custom("msgId", msgId)
	notification.Payload = p
	res, err := client.Push(notification)
	if err != nil {
		logs.Logger.Error("[APNS] error "+err.Error(), *notification)
		if res, err = apnsPushOnceMore(notification); err != nil {
			return err
		}
	}
	if res.StatusCode != 200 {
		errs := fmt.Sprintf("[APNS] error statusCode %d reson=%s token=%s title=%s body=%s flag=%d uid=%s",
			res.StatusCode, res.Reason, token, title, body, flag, uid)
		logs.Logger.Error(errs)
		return errors.New(errs)
	}
	return nil
}
func apnsPushOnceMore(notification *apns.Notification) (*apns.Response, error) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover from apnsPush ", r)
		}
	}()

	return client.Push(notification)

}
