package main

import (
	"errors"
	"fmt"
	"galopush/internal/logs"
	"strconv"

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

func apnsPush(token, title, body string, flag int) error {
	logs.Logger.Debug("[APNS] to ", token, " title=", title, " body=", body, " flag=", flag)
	notification := &apns.Notification{}
	notification.DeviceToken = token
	p := pl.NewPayload().Badge(1).AlertTitle(title).AlertBody(body).Sound(iOSSound[flag])
	notification.Payload = p
	res, err := client.Push(notification)
	if err != nil {
		logs.Logger.Error(err)
		return err
	}
	if res.StatusCode != 200 {
		errs := fmt.Sprintf("push apns with error statusCode %d reson=%s", res.StatusCode, res.Reason)
		logs.Logger.Error(errs)
		return errors.New(errs)
	}
	return nil
}
