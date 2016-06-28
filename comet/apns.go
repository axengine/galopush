package main

import (
	"errors"
	"fmt"
	"galopush/logs"
	"strconv"

	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/widuu/goini"
)

var (
	isDevelopment int
	pemFile       string
	password      string
)

var client *apns.Client

func init() {
	conf := goini.SetConfig("./config.ini")
	sDev := conf.GetValue("apns", "Development")
	isDevelopment, _ = strconv.Atoi(sDev)

	pemFile = conf.GetValue("apns", "pemFile")
	password = conf.GetValue("apns", "password")

	cert, pemErr := certificate.FromPemFile(pemFile, password)
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

	client = cli
}

func apnsPush(token, topic, msg string) error {
	notification := &apns.Notification{}
	notification.DeviceToken = token
	notification.Topic = topic
	notification.Payload = []byte(msg)
	res, err := client.Push(notification)
	if err != nil {
		logs.Logger.Error(err)
		return err
	}
	if res.StatusCode != 200 {
		errs := fmt.Sprintf("push apns with error statusCode %d", res.StatusCode)
		logs.Logger.Error(errs)
		return errors.New(errs)
	}
	return nil
}
