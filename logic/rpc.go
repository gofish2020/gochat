/**
 * Created by nash
 * Date: 2019-08-12
 * Time: 15:52
 */
package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"gochat/config"
	"gochat/logic/dao"
	"gochat/proto"
	"gochat/tools"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type TestLogic struct {
}

func (rpc *TestLogic) GetUserInfoByUserId(ctx context.Context, args *proto.GetUserInfoRequest, reply *proto.GetUserInfoResponse) (err error) {

	reply.UserId = 200
	reply.UserName = "fsfsdf"
	reply.Code = config.SuccessReplyCode
	return
}

type RpcLogic struct {
}

func (rpc *RpcLogic) Register(ctx context.Context, args *proto.RegisterRequest, reply *proto.RegisterReply) (err error) {
	reply.Code = config.FailReplyCode
	u := new(dao.User)
	// 检查用户是否重复注册
	uData := u.CheckHaveUserName(args.Name)
	if uData.Id > 0 {
		return errors.New("this user name already have , please login !!!")
	}
	// 注册账号
	u.UserName = args.Name
	u.Password = args.Password
	userId, err := u.Add()
	if err != nil {
		logrus.Infof("register err:%s", err.Error())
		return err
	}
	if userId == 0 {
		return errors.New("register userId empty!")
	}
	// 生成随机 AuthToken
	randToken := tools.GetRandomToken(32)
	sessionId := tools.CreateSessionId(randToken) // sess_xxxx
	userData := make(map[string]interface{})
	userData["userId"] = userId
	userData["userName"] = args.Name

	//  可以利用 HGETALL sess_xxxxx 查看里面保存的userId和userName信息
	RedisSessClient.Do("MULTI")                //redis 事务保证原子性
	RedisSessClient.HMSet(sessionId, userData) // hmset key  field value [field value...]  保【 sess_xxxx 和 用户信息的映射关系】
	RedisSessClient.Expire(sessionId, 86400*time.Second)
	loginSessionId := tools.GetSessionIdByUserId(userId)
	RedisSessClient.Set(loginSessionId, randToken, 86400*time.Second) // 为了保存 sess_map_xxx 和 AuthToken 的反向映射
	err = RedisSessClient.Do("EXEC").Err()
	if err != nil {
		logrus.Infof("register set redis token fail!")
		return err
	}
	reply.Code = config.SuccessReplyCode
	reply.AuthToken = randToken // 返回给前端页面（下次前端请求的时候，需要带上AuthToken）
	return
}

func (rpc *RpcLogic) Login(ctx context.Context, args *proto.LoginRequest, reply *proto.LoginResponse) (err error) {
	reply.Code = config.FailReplyCode
	u := new(dao.User)
	userName := args.Name
	passWord := args.Password
	data := u.CheckHaveUserName(userName)
	// 检查账户是否存在 or 密码是否正确
	if (data.Id == 0) || (passWord != data.Password) {
		return errors.New("no this user or password error!")
	}
	loginSessionId := tools.GetSessionIdByUserId(data.Id) // sess_map_xxxxx
	// 创建 AuthToken
	randToken := tools.GetRandomToken(32)
	sessionId := tools.CreateSessionId(randToken) // 新的sess_xxxx
	userData := make(map[string]interface{})
	userData["userId"] = data.Id
	userData["userName"] = data.UserName

	token, _ := RedisSessClient.Get(loginSessionId).Result() // 先看下是否有历史的 sess_map_xxxxx
	if token != "" {                                         // 有历史的登录token，需要将之前的退出
		oldSession := tools.CreateSessionId(token)   // 旧的 sess_xxxx
		err := RedisSessClient.Del(oldSession).Err() // 直接删除（等价于退出上次的登录）
		if err != nil {
			return errors.New("logout user fail!token is:" + token)
		}
	}
	// redis事务
	RedisSessClient.Do("MULTI")
	RedisSessClient.HMSet(sessionId, userData) // 在redis中 保存 sess_xxxx 和用户信息映射
	RedisSessClient.Expire(sessionId, 86400*time.Second)
	RedisSessClient.Set(loginSessionId, randToken, 86400*time.Second) // 保存 sess_map_xxxxx 和 AuthToken 的映射
	err = RedisSessClient.Do("EXEC").Err()
	if err != nil {
		logrus.Infof("register set redis token fail!")
		return err
	}
	reply.Code = config.SuccessReplyCode
	reply.AuthToken = randToken // 返回给前端页面（下次前端请求的时候，需要带上AuthToken）
	return
}

func (rpc *RpcLogic) GetUserInfoByUserId(ctx context.Context, args *proto.GetUserInfoRequest, reply *proto.GetUserInfoResponse) (err error) {
	reply.Code = config.FailReplyCode
	userId := args.UserId
	u := new(dao.User)
	userName := u.GetUserNameByUserId(userId)
	reply.UserId = userId
	reply.UserName = userName
	reply.Code = config.SuccessReplyCode
	return
}

func (rpc *RpcLogic) CheckAuth(ctx context.Context, args *proto.CheckAuthRequest, reply *proto.CheckAuthResponse) (err error) {
	reply.Code = config.FailReplyCode
	// 之前后端返回给前端的AuthToken，前端请求的时候会带上这个参数
	authToken := args.AuthToken
	sessionName := tools.GetSessionName(authToken) // sess_xxxx
	var userDataMap = map[string]string{}
	// userDataMap中保存着用户信息
	userDataMap, err = RedisSessClient.HGetAll(sessionName).Result() // 从redis中查询 sess_xxxx
	if err != nil {
		logrus.Infof("check auth fail!,authToken is:%s", authToken)
		return err
	}

	if len(userDataMap) == 0 { // 无信息，说明tAuthToken无效
		logrus.Infof("no this user session,authToken is:%s", authToken)
		return
	}

	// 如果AuthTokne检查通过，返回 userId + userName
	intUserId, _ := strconv.Atoi(userDataMap["userId"])
	reply.UserId = intUserId
	reply.Code = config.SuccessReplyCode
	reply.UserName = userDataMap["userName"]
	return
}

// 做的事情，就是将login的时候，在redis中保存的信息，全部都del掉
func (rpc *RpcLogic) Logout(ctx context.Context, args *proto.LogoutRequest, reply *proto.LogoutResponse) (err error) {
	reply.Code = config.FailReplyCode
	authToken := args.AuthToken
	sessionName := tools.GetSessionName(authToken)

	var userDataMap = map[string]string{}
	userDataMap, err = RedisSessClient.HGetAll(sessionName).Result()
	if err != nil {
		logrus.Infof("check auth fail!,authToken is:%s", authToken)
		return err
	}
	if len(userDataMap) == 0 {
		logrus.Infof("no this user session,authToken is:%s", authToken)
		return
	}
	intUserId, _ := strconv.Atoi(userDataMap["userId"])
	sessIdMap := tools.GetSessionIdByUserId(intUserId)

	err = RedisSessClient.Del(sessIdMap).Err() // del sess_map_xxxx
	if err != nil {
		logrus.Infof("logout del sess map error:%s", err.Error())
		return err
	}
	logic := new(Logic)
	serverIdKey := logic.getUserKey(fmt.Sprintf("%d", intUserId)) // del gochat_xxx （gochat_xxx 用来记录 userid 和 serverId的映射关系）
	err = RedisSessClient.Del(serverIdKey).Err()
	if err != nil {
		logrus.Infof("logout del server id error:%s", err.Error())
		return err
	}
	err = RedisSessClient.Del(sessionName).Err() // del sess_xxxx
	if err != nil {
		logrus.Infof("logout error:%s", err.Error())
		return err
	}
	reply.Code = config.SuccessReplyCode
	return
}

/*
*
single send msg
*/
func (rpc *RpcLogic) Push(ctx context.Context, args *proto.Send, reply *proto.SuccessReply) (err error) {
	reply.Code = config.FailReplyCode
	sendData := args
	var bodyBytes []byte

	// 发送到（消息）数据包
	bodyBytes, err = json.Marshal(sendData)
	if err != nil {
		logrus.Errorf("logic,push msg fail,err:%s", err.Error())
		return
	}
	logic := new(Logic)
	// 生成redis key
	userSidKey := logic.getUserKey(fmt.Sprintf("%d", sendData.ToUserId))
	// 获取key 对应的value
	serverIdStr := RedisSessClient.Get(userSidKey).Val()
	//var serverIdInt int
	//serverIdInt, err = strconv.Atoi(serverId)
	// if err != nil {
	// 	logrus.Errorf("logic,push parse int fail:%s", err.Error())
	// 	return
	// }
	err = logic.RedisPublishChannel(serverIdStr, sendData.ToUserId, bodyBytes)
	if err != nil {
		logrus.Errorf("logic,redis publish err: %s", err.Error())
		return
	}
	reply.Code = config.SuccessReplyCode
	return
}

/*
*
push msg to room
*/
func (rpc *RpcLogic) PushRoom(ctx context.Context, args *proto.Send, reply *proto.SuccessReply) (err error) {
	reply.Code = config.FailReplyCode
	sendData := args
	roomId := sendData.RoomId
	logic := new(Logic)

	roomUserKey := logic.getRoomUserKey(strconv.Itoa(roomId))      // gochat_room_xxxx
	roomUserInfo, err := RedisClient.HGetAll(roomUserKey).Result() // 从redis中获取房间信息
	if err != nil {
		logrus.Errorf("logic,PushRoom redis hGetAll err:%s", err.Error())
		return
	}

	var bodyBytes []byte
	sendData.RoomId = roomId                     // 房间id
	sendData.Msg = args.Msg                      // 发送的消息
	sendData.FromUserId = args.FromUserId        // 发送人id
	sendData.FromUserName = args.FromUserName    // 发送人名
	sendData.Op = config.OpRoomSend              // 向房间发送
	sendData.CreateTime = tools.GetNowDateTime() // 时间
	bodyBytes, err = json.Marshal(sendData)      // 转成json字符串

	if err != nil {
		logrus.Errorf("logic,PushRoom Marshal err:%s", err.Error())
		return
	}
	// 保存到redis链表 gochat_queue 中
	err = logic.RedisPublishRoomInfo(roomId, len(roomUserInfo), roomUserInfo, bodyBytes)
	if err != nil {
		logrus.Errorf("logic,PushRoom err:%s", err.Error())
		return
	}
	reply.Code = config.SuccessReplyCode
	return
}

/*
*
get room online person count
*/
func (rpc *RpcLogic) Count(ctx context.Context, args *proto.Send, reply *proto.SuccessReply) (err error) {
	reply.Code = config.FailReplyCode
	roomId := args.RoomId
	logic := new(Logic)
	var count int
	count, err = RedisSessClient.Get(logic.getRoomOnlineCountKey(fmt.Sprintf("%d", roomId))).Int()
	err = logic.RedisPushRoomCount(roomId, count)
	if err != nil {
		logrus.Errorf("logic,Count err:%s", err.Error())
		return
	}
	reply.Code = config.SuccessReplyCode
	return
}

/*
*
get room info
*/
func (rpc *RpcLogic) GetRoomInfo(ctx context.Context, args *proto.Send, reply *proto.SuccessReply) (err error) {
	reply.Code = config.FailReplyCode
	logic := new(Logic)
	roomId := args.RoomId
	roomUserInfo := make(map[string]string)
	roomUserKey := logic.getRoomUserKey(strconv.Itoa(roomId))
	roomUserInfo, err = RedisClient.HGetAll(roomUserKey).Result()
	if len(roomUserInfo) == 0 {
		return errors.New("getRoomInfo no this user")
	}
	err = logic.RedisPushRoomInfo(roomId, len(roomUserInfo), roomUserInfo)
	if err != nil {
		logrus.Errorf("logic,GetRoomInfo err:%s", err.Error())
		return
	}
	reply.Code = config.SuccessReplyCode
	return
}

func (rpc *RpcLogic) Connect(ctx context.Context, args *proto.ConnectRequest, reply *proto.ConnectReply) (err error) {
	if args == nil {
		logrus.Errorf("logic,connect args empty")
		return
	}
	logic := new(Logic)
	//key := logic.getUserKey(args.AuthToken)
	logrus.Infof("logic,authToken is:%s", args.AuthToken)
	key := tools.GetSessionName(args.AuthToken) // 校验 AuthToken && 获取AuthToken对应的用户信息
	userInfo, err := RedisClient.HGetAll(key).Result()
	if err != nil {
		logrus.Infof("RedisCli HGetAll key :%s , err:%s", key, err.Error())
		return err
	}
	if len(userInfo) == 0 {
		reply.UserId = 0
		return
	}

	reply.UserId, _ = strconv.Atoi(userInfo["userId"])
	roomUserKey := logic.getRoomUserKey(strconv.Itoa(args.RoomId)) // gochat_room_xxxx
	if reply.UserId != 0 {
		userKey := logic.getUserKey(fmt.Sprintf("%d", reply.UserId)) // gochat_xxxx
		logrus.Infof("logic redis set userKey:%s, serverId : %s", userKey, args.ServerId)
		validTime := config.RedisBaseValidTime * time.Second

		// 设定 gochat_xxxx 的值：表示【用户id 和 serverId的映射关系】，也就是【用户是基于哪个server连接进来的】
		err = RedisClient.Set(userKey, args.ServerId, validTime).Err()
		if err != nil {
			logrus.Warnf("logic set err:%s", err)
		}

		if RedisClient.HGet(roomUserKey, fmt.Sprintf("%d", reply.UserId)).Val() == "" {
			// 在gochat_room_xxxx 中记录房间中的 用户id+用户的名字（前提：之前没有记录过）
			RedisClient.HSet(roomUserKey, fmt.Sprintf("%d", reply.UserId), userInfo["userName"])
			// 设定 RedisRoomOnlinePrefix_xxxxx 房间人数+1
			RedisClient.Incr(logic.getRoomOnlineCountKey(fmt.Sprintf("%d", args.RoomId)))
		}
	}
	logrus.Infof("logic rpc userId:%d", reply.UserId)
	return
}

func (rpc *RpcLogic) DisConnect(ctx context.Context, args *proto.DisConnectRequest, reply *proto.DisConnectReply) (err error) {
	logic := new(Logic)
	roomUserKey := logic.getRoomUserKey(strconv.Itoa(args.RoomId))
	// room user count --
	if args.RoomId > 0 {
		count, _ := RedisSessClient.Get(logic.getRoomOnlineCountKey(fmt.Sprintf("%d", args.RoomId))).Int()
		if count > 0 {
			RedisClient.Decr(logic.getRoomOnlineCountKey(fmt.Sprintf("%d", args.RoomId))).Result()
		}
	}
	// room login user--
	if args.UserId != 0 {
		err = RedisClient.HDel(roomUserKey, fmt.Sprintf("%d", args.UserId)).Err()
		if err != nil {
			logrus.Warnf("HDel getRoomUserKey err : %s", err)
		}
	}
	//below code can optimize send a signal to queue,another process get a signal from queue,then push event to websocket
	roomUserInfo, err := RedisClient.HGetAll(roomUserKey).Result()
	if err != nil {
		logrus.Warnf("RedisCli HGetAll roomUserInfo key:%s, err: %s", roomUserKey, err)
	}
	if err = logic.RedisPublishRoomInfo(args.RoomId, len(roomUserInfo), roomUserInfo, nil); err != nil {
		logrus.Warnf("publish RedisPublishRoomCount err: %s", err.Error())
		return
	}
	return
}
