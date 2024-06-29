/**
 * Created by nash
 * Date: 2019-08-12
 * Time: 23:36
 */
package connect

import (
	"context"
	"errors"
	"fmt"
	"gochat/config"
	"gochat/proto"
	"gochat/tools"
	"strings"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/rpcxio/libkv/store"
	etcdV3 "github.com/rpcxio/rpcx-etcd/client"
	"github.com/rpcxio/rpcx-etcd/serverplugin"
	"github.com/sirupsen/logrus"
	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/server"
)

var logicRpcClient client.XClient
var once sync.Once

type RpcConnect struct {
}

// 这里的逻辑和 api服务 （代码路径：api/rpc/rpc.go）中InitLogicRpcClient 函数的逻辑一模一样
func (c *Connect) InitLogicRpcClient() (err error) {

	etcdConfigOption := &store.Config{
		ClientTLS:         nil,
		TLS:               nil,
		ConnectionTimeout: time.Duration(config.Conf.Common.CommonEtcd.ConnectionTimeout) * time.Second,
		Bucket:            "",
		PersistConnection: true,
		Username:          config.Conf.Common.CommonEtcd.UserName,
		Password:          config.Conf.Common.CommonEtcd.Password,
	}
	once.Do(func() {
		d, e := etcdV3.NewEtcdV3Discovery(
			config.Conf.Common.CommonEtcd.BasePath,
			config.Conf.Common.CommonEtcd.ServerPathLogic,
			[]string{config.Conf.Common.CommonEtcd.Host},
			true,
			etcdConfigOption,
		)
		if e != nil {
			logrus.Fatalf("init connect rpc etcd discovery client fail:%s", e.Error())
		}

		logicRpcClient = client.NewXClient(config.Conf.Common.CommonEtcd.ServerPathLogic, client.Failtry, client.RandomSelect, d, client.DefaultOption)
	})
	if logicRpcClient == nil {
		return errors.New("get rpc client nil")
	}
	return
}

func (rpc *RpcConnect) Connect(connReq *proto.ConnectRequest) (uid int, err error) {

	// 调用 logic 服务的 Connect函数
	reply := &proto.ConnectReply{}
	err = logicRpcClient.Call(context.Background(), "Connect", connReq, reply)
	if err != nil {
		logrus.Fatalf("failed to call: %v", err)
	}
	uid = reply.UserId
	logrus.Infof("connect logic userId :%d", reply.UserId)
	return
}

func (rpc *RpcConnect) DisConnect(disConnReq *proto.DisConnectRequest) (err error) {
	reply := &proto.DisConnectReply{}
	if err = logicRpcClient.Call(context.Background(), "DisConnect", disConnReq, reply); err != nil {
		logrus.Fatalf("failed to call: %v", err)
	}
	return
}

// 如果是websocket服务，启动 rpc server（监听地址为 ConnectRpcAddressWebSockts.Address）
func (c *Connect) InitConnectWebsocketRpcServer() (err error) {
	var network, addr string
	connectRpcAddress := strings.Split(config.Conf.Connect.ConnectRpcAddressWebSockts.Address, ",") // tcp@0.0.0.0:6912,tcp@0.0.0.0:6913
	for _, bind := range connectRpcAddress {

		// 按照@拆分字符串
		if network, addr, err = tools.ParseNetwork(bind); err != nil {
			logrus.Panicf("InitConnectWebsocketRpcServer ParseNetwork error : %s", err)
		}
		logrus.Infof("Connect start run at-->%s:%s", network, addr)
		// 启动 rpc 服务
		go c.createConnectWebsocktsRpcServer(network, addr)
	}
	return
}

// 如果是tcp服务，启动rpc server（监听地址为 ConnectRpcAddressTcp.Address）
func (c *Connect) InitConnectTcpRpcServer() (err error) {
	var network, addr string
	connectRpcAddress := strings.Split(config.Conf.Connect.ConnectRpcAddressTcp.Address, ",") // tcp@0.0.0.0:6914,tcp@0.0.0.0:6915
	for _, bind := range connectRpcAddress {
		if network, addr, err = tools.ParseNetwork(bind); err != nil {
			logrus.Panicf("InitConnectTcpRpcServer ParseNetwork error : %s", err)
		}
		logrus.Infof("Connect start run at-->%s:%s", network, addr)
		// 启动 rpc 服务
		go c.createConnectTcpRpcServer(network, addr)
	}
	return
}

type RpcConnectPush struct {
}

func (rpc *RpcConnectPush) PushSingleMsg(ctx context.Context, pushMsgReq *proto.PushMsgRequest, successReply *proto.SuccessReply) (err error) {
	var (
		bucket  *Bucket
		channel *Channel
	)

	logrus.Infof("rpc PushMsg :%v ", pushMsgReq)
	if pushMsgReq == nil { // 推送请求的消息不能为空
		logrus.Errorf("rpc PushSingleMsg() args:(%v)", pushMsgReq)
		return
	}

	//
	bucket = DefaultServer.Bucket(pushMsgReq.UserId)
	if channel = bucket.Channel(pushMsgReq.UserId); channel != nil {
		err = channel.Push(&pushMsgReq.Msg)
		logrus.Infof("DefaultServer Channel err nil ,args: %v", pushMsgReq)
		return
	}
	successReply.Code = config.SuccessReplyCode
	successReply.Msg = config.SuccessReplyMsg
	logrus.Infof("successReply:%v", successReply)
	return
}

func (rpc *RpcConnectPush) PushRoomMsg(ctx context.Context, pushRoomMsgReq *proto.PushRoomMsgRequest, successReply *proto.SuccessReply) (err error) {
	successReply.Code = config.SuccessReplyCode
	successReply.Msg = config.SuccessReplyMsg
	logrus.Infof("PushRoomMsg msg %+v", pushRoomMsgReq)

	// 【注意】这里对所有的buckets；原因在于 同一个roomid 可以存在于不同的 Buckets中
	for _, bucket := range DefaultServer.Buckets {
		bucket.BroadcastRoom(pushRoomMsgReq)
	}
	return
}

func (rpc *RpcConnectPush) PushRoomCount(ctx context.Context, pushRoomMsgReq *proto.PushRoomMsgRequest, successReply *proto.SuccessReply) (err error) {
	successReply.Code = config.SuccessReplyCode
	successReply.Msg = config.SuccessReplyMsg
	logrus.Infof("PushRoomCount msg %v", pushRoomMsgReq)
	for _, bucket := range DefaultServer.Buckets {
		bucket.BroadcastRoom(pushRoomMsgReq)
	}
	return
}

func (rpc *RpcConnectPush) PushRoomInfo(ctx context.Context, pushRoomMsgReq *proto.PushRoomMsgRequest, successReply *proto.SuccessReply) (err error) {
	successReply.Code = config.SuccessReplyCode
	successReply.Msg = config.SuccessReplyMsg
	logrus.Infof("connect,PushRoomInfo msg %+v", pushRoomMsgReq)
	for _, bucket := range DefaultServer.Buckets {
		bucket.BroadcastRoom(pushRoomMsgReq)
	}
	return
}

func (c *Connect) createConnectWebsocktsRpcServer(network string, addr string) {
	s := server.NewServer()
	addRegistryPlugin(s, network, addr)
	// CommonEtcd.ServerPathConnect = ConnectRpc
	s.RegisterName(config.Conf.Common.CommonEtcd.ServerPathConnect, new(RpcConnectPush), fmt.Sprintf("serverId=%s&serverType=ws", c.ServerId)) // 区别在这里  serverType=ws
	s.RegisterOnShutdown(func(s *server.Server) {
		s.UnregisterAll()
	})
	s.Serve(network, addr)
}

func (c *Connect) createConnectTcpRpcServer(network string, addr string) {
	s := server.NewServer()
	addRegistryPlugin(s, network, addr)
	// CommonEtcd.ServerPathConnect = ConnectRpc
	s.RegisterName(config.Conf.Common.CommonEtcd.ServerPathConnect, new(RpcConnectPush), fmt.Sprintf("serverId=%s&serverType=tcp", c.ServerId)) // 区别在这里  serverType=tcp
	s.RegisterOnShutdown(func(s *server.Server) {
		s.UnregisterAll()
	})
	s.Serve(network, addr)
}

func addRegistryPlugin(s *server.Server, network string, addr string) {
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
