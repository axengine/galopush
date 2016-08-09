package main

import (
	"encoding/json"
	"galopush/internal/logs"
	"galopush/internal/protocol"
	"galopush/internal/rds"
)

//NsqHandler NSQ CHANNEL句柄
//根据不同的主题和数据进行分发
func (p *Router) NsqHandler(topic string, i interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error("recover ", r)
		}
	}()
	b := i.([]byte)
	logs.Logger.Info("[nsq] topic=", topic, " msg=", string(b[:]))

	switch topic {
	case p.topics[4]: //sessionTimeout
		var msg SessionTimeout
		if err := json.Unmarshal(b, &msg); err != nil {
			logs.Logger.Error(err, string(b[:]))
			return
		}

		if online := p.store.SessionOnline(msg.Sess.Uid, msg.Sess.Termtype); online {
			msg.Sess.Flag = 1
		}
		buff, _ := json.Marshal(&msg)
		logs.Logger.Info("[session time out resp]", string(buff[:]))
		if err := p.producer.Publish(msg.BackTopic, buff); err != nil {
			logs.Logger.Error("sessionTimeout push to nsq Failed ", err, " msg=", string(buff[:]))
		}
	case p.topics[0]: //userOnlineState
		{
			var msg UserOnlineState
			if err := json.Unmarshal(b, &msg); err != nil {
				logs.Logger.Error(err, string(b[:]))
				return
			}

			//离线状态不处理
			if msg.Login == false {
				break
			}

			{
				var find bool
				sess := p.store.FindSessions(msg.Uid)
				if sess != nil {
					for i, v := range sess.Sess {
						if v.Plat == msg.Termtype {
							logs.Logger.Debug("UserState Find id=", v.Id, " plat=", v.Plat, " online=", v.Online, " login=", v.Login, " token=", v.AuthCode)

							//socket在线 用户在线
							if v.Online == true && v.Login == true && msg.Login == true {
								//踢人
								c := p.pool.findComet(sess.CometId)
								if c != nil {
									logs.Logger.Info("UserState Kick Because repeat login id=", v.Id, " palt=", v.Plat)
									c.rpcClient.Kick(v.Id, v.Plat, v.AuthCode, protocol.KICK_REASON_REPEAT)
									if v.Plat == protocol.PLAT_IOS {
										c.rpcClient.Push(1, v.Id, v.Plat, v.IOSToken, 1, "你的帐号已在另一台手机上登录，若非本人操作，请登录后修改密码")
									}
								}
							}

							//socket在线 用户离线 增加对鉴权码的校验 防止web踢人后又下线的BUG
							//							if msg.Login == false && v.Online == true && v.AuthCode == msg.Code {
							//								//踢人
							//								c := p.pool.findComet(sess.CometId)
							//								if c != nil {
							//									logs.Logger.Info("UserState Kick Because unlogin id=", v.Id, " palt=", v.Plat)
							//									c.rpcClient.Kick(v.Id, v.Plat, v.AuthCode, protocol.KICK_REASON_REPEAT)
							//									if v.Plat == protocol.PLAT_IOS {
							//										c.rpcClient.Push(1, v.Id, v.Plat, v.IOSToken, 1, "你的帐号已在另一台手机上登录，若非本人操作，请登录后修改密码")
							//									}
							//								}
							//							}

							find = true
							v.AuthCode = msg.Code
							v.IOSToken = msg.DeviceToken
							v.Login = msg.Login
						} else if v.Plat|msg.Termtype >= 1 && v.Plat|msg.Termtype <= 3 { //类型互斥
							logs.Logger.Debug("UserState Find Mutex id=", v.Id, " plat=", v.Plat, " online=", v.Online, " login=", v.Login, " token=", v.AuthCode)

							//socket在线 用户在线
							if v.Online == true && v.Login == true && msg.Login == true {
								//踢人
								c := p.pool.findComet(sess.CometId)
								if c != nil {
									logs.Logger.Info("UserState Kick Because mutex login id=", v.Id, " palt=", v.Plat)
									c.rpcClient.Kick(v.Id, v.Plat, v.AuthCode, protocol.KICK_REASON_MUTEX)
									if v.Plat == protocol.PLAT_IOS {
										c.rpcClient.Push(1, v.Id, v.Plat, v.IOSToken, 1, "你的帐号已在另一台手机上登录，若非本人操作，请登录后修改密码")
									}
								}
							}
							//直接清除互斥端信息
							sess.Sess = append(sess.Sess[:i], sess.Sess[i+1:]...)
						}
					}
					//有session但无对应终端类型
					if find == false {
						var it rds.Session
						it.Id = msg.Uid
						it.Plat = msg.Termtype
						it.Online = false
						it.Login = msg.Login
						it.AuthCode = msg.Code
						it.IOSToken = msg.DeviceToken
						sess.Sess = append(sess.Sess, &it)
						logs.Logger.Debug("UserState New item=", it)
					}
					p.store.SaveSessions(sess)
				} else {
					//没有找到session
					sess = new(rds.Sessions)
					sess.Id = msg.Uid

					var it rds.Session
					it.Id = msg.Uid
					it.Plat = msg.Termtype
					it.Online = false
					it.Login = msg.Login
					it.AuthCode = msg.Code
					it.IOSToken = msg.DeviceToken
					sess.Sess = append(sess.Sess, &it)
					p.store.SaveSessions(sess)

					logs.Logger.Debug("UserState New session ", it)
				}
			}
		}
	case p.topics[1], p.topics[2], p.topics[3]: //push callback message
		{
			var msg MsgDownward
			if err := json.Unmarshal(b, &msg); err != nil {
				logs.Logger.Error(err, string(b[:]))
				return
			}

			msgType := 0
			if topic == "push" {
				msgType = protocol.MSGTYPE_PUSH
			} else if topic == "callback" {
				msgType = protocol.MSGTYPE_CALLBACK
			} else if topic == "msgDownward" {
				msgType = protocol.MSGTYPE_MESSAGE
			}

			for _, receiver := range msg.Receivers {
				sess := p.store.FindSessions(receiver.Uid)
				if sess == nil {
					logs.Logger.Error("OnPush canot find session uid=", receiver.Uid /*, " plat=", receiver.Termtype*/)
					continue
				}

				//comet 离线了
				comet := p.pool.findComet(sess.CometId)
				if comet == nil {
					logs.Logger.Error("OnPush comet offline ", sess.CometId)
					for _, it := range sess.Sess {
						if it.Plat&receiver.Termtype > 0 {
							p.SaveOfflineMsg(msgType, it.Id, it.Plat, msg.Body)
						}
					}
					continue
				}

				for _, it := range sess.Sess {
					if it.Plat&receiver.Termtype > 0 {
						logs.Logger.Info("[OnPush>>>] topic=", topic, " to id=", it.Id, " plat=", it.Plat)
						err := comet.rpcClient.Push(msgType, it.Id, it.Plat, it.IOSToken, msg.Flag, msg.Body)
						if err != nil {
							logs.Logger.Error(err)
							if it.Plat != PLAT_WEB { //web不存离线
								p.SaveOfflineMsg(msgType, it.Id, it.Plat, msg.Body)
							}
						}
					}
				}
			}
		}
	}
}

func (p *Router) SaveOfflineMsg(msgType int, id string, termtype int, msg string) {
	switch msgType {
	case protocol.MSGTYPE_PUSH:
		p.store.SavePushMsg(id, termtype, []byte(msg))
	case protocol.MSGTYPE_CALLBACK:
		p.store.SaveCallbackMsg(id, termtype, []byte(msg))
	case protocol.MSGTYPE_MESSAGE:
		p.store.SaveImMsg(id, termtype, []byte(msg))
	}
}
