/**
 * Created by nash
 * Date: 2019-10-06
 * Time: 22:46
 */
package rpc

import (
	"context"
	"gochat/config"
	"gochat/proto"
	"sync"
	"time"

	"github.com/rpcxio/libkv/store"
	etcdV3 "github.com/rpcxio/rpcx-etcd/client"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/rpcx/client"
)

// rpc client  for logic
var LogicRpcClient client.XClient

var LogicRpcTestClient client.XClient
var once sync.Once

type RpcLogic struct {
}

var RpcLogicObj *RpcLogic

func InitLogicRpcClient() {
	once.Do(func() {

		// etcd
		etcdConfigOption := &store.Config{
			ClientTLS:         nil,
			TLS:               nil,
			ConnectionTimeout: time.Duration(config.Conf.Common.CommonEtcd.ConnectionTimeout) * time.Second, // 5s
			Bucket:            "",
			PersistConnection: true,
			Username:          config.Conf.Common.CommonEtcd.UserName, // “”
			Password:          config.Conf.Common.CommonEtcd.Password, // “”
		}

		// etcd配置（来源于 config/dev/common.toml 文件)
		d, err := etcdV3.NewEtcdV3Discovery(
			config.Conf.Common.CommonEtcd.BasePath,        // /gochat_srv
			config.Conf.Common.CommonEtcd.ServerPathLogic, // LogicRpc
			[]string{config.Conf.Common.CommonEtcd.Host},  // 127.0.0.1:2379
			true,
			etcdConfigOption,
		)
		if err != nil {
			logrus.Fatalf("init connect rpc etcd discovery client fail:%s", err.Error())
		}
		for _, connectConf := range d.GetServices() {
			logrus.Infof("key is:%s,value is:%s", connectConf.Key, connectConf.Value)
		}
		// 初始化 logic 的 rpc 客户端
		LogicRpcClient = client.NewXClient(config.Conf.Common.CommonEtcd.ServerPathLogic, client.Failtry, client.RandomSelect, d, client.DefaultOption) //在etcd中，基于servicePath 随机选择一个 rpc server，并建立连接

		// d1, err := etcdV3.NewEtcdV3Discovery(
		// 	config.Conf.Common.CommonEtcd.BasePath,       // /gochat_srv
		// 	"TestLogic",                                  // test
		// 	[]string{config.Conf.Common.CommonEtcd.Host}, // 127.0.0.1:2379
		// 	true,
		// 	etcdConfigOption,
		// )
		// for _, connectConf := range d1.GetServices() {
		// 	logrus.Infof("key is:%s,value is:%s", connectConf.Key, connectConf.Value)
		// }
		// if err != nil {
		// 	logrus.Fatalf("init connect rpc etcd discovery client fail:%s", err.Error())
		// }
		// LogicRpcTestClient = client.NewXClient("TestLogic", client.Failtry, client.RandomSelect, d1, client.DefaultOption)

		RpcLogicObj = new(RpcLogic)
	})
	if LogicRpcClient == nil {
		logrus.Fatalf("get logic rpc client nil")
	}

	// if LogicRpcTestClient == nil {
	// 	logrus.Fatalf("get logic rpc LogicRpcTestClient nil")
	// }
}

func (rpc *RpcLogic) Login(req *proto.LoginRequest) (code int, authToken string, msg string) {

	reply := &proto.LoginResponse{}
	// logic服务的 Login函数 ，代码位于 logic/rpc.go中
	err := LogicRpcClient.Call(context.Background(), "Login", req, reply)
	if err != nil {
		msg = err.Error()
	}
	code = reply.Code
	authToken = reply.AuthToken // 将 AuthToken 返回给前端
	return
}

func (rpc *RpcLogic) Register(req *proto.RegisterRequest) (code int, authToken string, msg string) {
	reply := &proto.RegisterReply{}
	err := LogicRpcClient.Call(context.Background(), "Register", req, reply)
	if err != nil {
		msg = err.Error()
	}
	code = reply.Code
	authToken = reply.AuthToken
	return
}

func (rpc *RpcLogic) GetUserNameByUserId(req *proto.GetUserInfoRequest) (code int, userName string) {
	reply := &proto.GetUserInfoResponse{}
	LogicRpcClient.Call(context.Background(), "GetUserInfoByUserId", req, reply)
	code = reply.Code
	userName = reply.UserName
	return
}

func (rpc *RpcLogic) CheckAuth(req *proto.CheckAuthRequest) (code int, userId int, userName string) {
	reply := &proto.CheckAuthResponse{}
	// logic服务的 CheckAuth 函数 ，代码位于 logic/rpc.go中
	LogicRpcClient.Call(context.Background(), "CheckAuth", req, reply)
	code = reply.Code
	userId = reply.UserId
	userName = reply.UserName
	return
}

func (rpc *RpcLogic) Logout(req *proto.LogoutRequest) (code int) {
	reply := &proto.LogoutResponse{}
	LogicRpcClient.Call(context.Background(), "Logout", req, reply)
	code = reply.Code
	return
}

func (rpc *RpcLogic) Push(req *proto.Send) (code int, msg string) {
	reply := &proto.SuccessReply{}
	LogicRpcClient.Call(context.Background(), "Push", req, reply)
	code = reply.Code
	msg = reply.Msg
	return
}

func (rpc *RpcLogic) PushRoom(req *proto.Send) (code int, msg string) {
	reply := &proto.SuccessReply{}
	// 通过rpc 调用logic服务中的PushRoom函数，代码位于logic/rpc.go
	LogicRpcClient.Call(context.Background(), "PushRoom", req, reply)
	code = reply.Code
	msg = reply.Msg
	return
}

func (rpc *RpcLogic) Count(req *proto.Send) (code int, msg string) {
	reply := &proto.SuccessReply{}
	LogicRpcClient.Call(context.Background(), "Count", req, reply)
	code = reply.Code
	msg = reply.Msg
	return
}

func (rpc *RpcLogic) GetRoomInfo(req *proto.Send) (code int, msg string) {
	reply := &proto.SuccessReply{}
	LogicRpcClient.Call(context.Background(), "GetRoomInfo", req, reply)
	code = reply.Code
	msg = reply.Msg
	return
}
