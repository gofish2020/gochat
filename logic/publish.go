/**
 * Created by nash
 * Date: 2019-08-12
 * Time: 15:44
 */
package logic

import (
	"bytes"
	"encoding/json"
	"gochat/config"
	"gochat/proto"
	"gochat/tools"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/rcrowley/go-metrics"
	"github.com/rpcxio/rpcx-etcd/serverplugin"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/rpcx/server"
)

var RedisClient *redis.Client
var RedisSessClient *redis.Client

func (logic *Logic) InitPublishRedisClient() (err error) {

	// redis 的配置
	redisOpt := tools.RedisOption{
		Address:  config.Conf.Common.CommonRedis.RedisAddress,
		Password: config.Conf.Common.CommonRedis.RedisPassword,
		Db:       config.Conf.Common.CommonRedis.Db,
	}
	// redis 客户端
	RedisClient = tools.GetRedisInstance(redisOpt)
	if pong, err := RedisClient.Ping().Result(); err != nil {
		logrus.Infof("RedisCli Ping Result pong: %s,  err: %s", pong, err)
	}
	//this can change use another redis save session data
	RedisSessClient = RedisClient
	return err
}

func (logic *Logic) InitRpcServer() (err error) {
	var network, addr string
	// rpc server 的地址+端口
	rpcAddressList := strings.Split(config.Conf.Logic.LogicBase.RpcAddress, ",") //tcp@127.0.0.1:6900,tcp@127.0.0.1:6901
	for _, bind := range rpcAddressList {

		// tpc + ip:port
		if network, addr, err = tools.ParseNetwork(bind); err != nil {
			logrus.Panicf("InitLogicRpc ParseNetwork error : %s", err.Error())
		}
		logrus.Infof("logic start run at-->%s:%s", network, addr)
		// 启动 rpc server
		go logic.createRpcServer(network, addr)
	}
	return
}

func (logic *Logic) createRpcServer(network string, addr string) {
	s := server.NewServer()
	logic.addRegistryPlugin(s, network, addr)

	//   /gochat_srv/LogicRpc/tcp@127.0.0.1:6900  对应new(RpcLogic)对象 (注册到 etcd中)
	err := s.RegisterName(config.Conf.Common.CommonEtcd.ServerPathLogic, new(RpcLogic), logic.ServerId)

	if err != nil {
		logrus.Errorf("register error:%s", err.Error())
	}

	// 表示注册 /gochat_srv/TestLogic/tcp@127.0.0.1:6900  对应  new(TestLogic)
	// err = s.Register(new(TestLogic), logic.ServerId) // 在同一个服务中，不同的对象，需要不同的name进行对应
	// if err != nil {
	// 	logrus.Errorf("register error:%s", err.Error())
	// }

	//关闭了，从etcd中取消注册
	s.RegisterOnShutdown(func(s *server.Server) {
		s.UnregisterAll()
	})

	// 启动rpc
	s.Serve(network, addr)
}

func (logic *Logic) addRegistryPlugin(s *server.Server, network string, addr string) {
	r := &serverplugin.EtcdV3RegisterPlugin{
		ServiceAddress: network + "@" + addr,
		EtcdServers:    []string{config.Conf.Common.CommonEtcd.Host},
		BasePath:       config.Conf.Common.CommonEtcd.BasePath,
		Metrics:        metrics.NewRegistry(),
		UpdateInterval: time.Minute,
	}
	err := r.Start()
	if err != nil {
		logrus.Fatal(err)
	}
	s.Plugins.Add(r)
}

func (logic *Logic) RedisPublishChannel(serverId string, toUserId int, msg []byte) (err error) {
	redisMsg := proto.RedisMsg{
		Op:       config.OpSingleSend,
		ServerId: serverId,
		UserId:   toUserId,
		Msg:      msg,
	}
	redisMsgStr, err := json.Marshal(redisMsg)
	if err != nil {
		logrus.Errorf("logic,RedisPublishChannel Marshal err:%s", err.Error())
		return err
	}

	// 保存到 redis 的链表 gochat_queue 中
	if err := RedisClient.LPush(config.QueueName, redisMsgStr).Err(); err != nil {
		logrus.Errorf("logic,lpush err:%s", err.Error())
		return err
	}
	return
}

func (logic *Logic) RedisPublishRoomInfo(roomId int, count int, RoomUserInfo map[string]string, msg []byte) (err error) {
	var redisMsg = &proto.RedisMsg{ // 这里又包装了一层信息
		Op:           config.OpRoomSend, // 向房间发送
		RoomId:       roomId,            // 房间id
		Count:        count,             // 房间人数
		Msg:          msg,               // 这个就是外部的json字符串后的消息
		RoomUserInfo: RoomUserInfo,      // 房间信息
	}

	redisMsgByte, err := json.Marshal(redisMsg) // 有进行了一次 json 字符串
	if err != nil {
		logrus.Errorf("logic,RedisPublishRoomInfo redisMsg error : %s", err.Error())
		return
	}

	// 保存到 redis 链表 gochat_queue 中 ， 通过 LRANGE gochat_queue 0 -1 命令行在redis中，可以查看其中的消息
	err = RedisClient.LPush(config.QueueName, redisMsgByte).Err()

	if err != nil {
		logrus.Errorf("logic,RedisPublishRoomInfo redisMsg error : %s", err.Error())
		return
	}
	return
}

func (logic *Logic) RedisPushRoomCount(roomId int, count int) (err error) {
	var redisMsg = &proto.RedisMsg{
		Op:     config.OpRoomCountSend,
		RoomId: roomId,
		Count:  count,
	}
	redisMsgByte, err := json.Marshal(redisMsg)
	if err != nil {
		logrus.Errorf("logic,RedisPushRoomCount redisMsg error : %s", err.Error())
		return
	}
	err = RedisClient.LPush(config.QueueName, redisMsgByte).Err()
	if err != nil {
		logrus.Errorf("logic,RedisPushRoomCount redisMsg error : %s", err.Error())
		return
	}
	return
}

func (logic *Logic) RedisPushRoomInfo(roomId int, count int, roomUserInfo map[string]string) (err error) {
	var redisMsg = &proto.RedisMsg{
		Op:           config.OpRoomInfoSend,
		RoomId:       roomId,
		Count:        count,
		RoomUserInfo: roomUserInfo,
	}
	redisMsgByte, err := json.Marshal(redisMsg)
	if err != nil {
		logrus.Errorf("logic,RedisPushRoomInfo redisMsg error : %s", err.Error())
		return
	}
	err = RedisClient.LPush(config.QueueName, redisMsgByte).Err()
	if err != nil {
		logrus.Errorf("logic,RedisPushRoomInfo redisMsg error : %s", err.Error())
		return
	}
	return
}

func (logic *Logic) getRoomUserKey(authKey string) string {
	var returnKey bytes.Buffer
	returnKey.WriteString(config.RedisRoomPrefix)
	returnKey.WriteString(authKey)
	return returnKey.String() // gochat_room_xxxx
}

func (logic *Logic) getRoomOnlineCountKey(authKey string) string {
	var returnKey bytes.Buffer
	returnKey.WriteString(config.RedisRoomOnlinePrefix)
	returnKey.WriteString(authKey)
	return returnKey.String()
}

func (logic *Logic) getUserKey(authKey string) string {
	var returnKey bytes.Buffer
	returnKey.WriteString(config.RedisPrefix)
	returnKey.WriteString(authKey)
	return returnKey.String() // gochat_xxxx
}
