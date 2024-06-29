/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 18:18
 */
package connect

import (
	"fmt"
	"gochat/config"
	_ "net/http/pprof"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var DefaultServer *Server

type Connect struct {
	ServerId string
}

func New() *Connect {
	return new(Connect)
}

func (c *Connect) Run() {
	// connect.toml 配置文件中的内容
	connectConfig := config.Conf.Connect

	//set the maximum number of CPUs that can be executing
	runtime.GOMAXPROCS(connectConfig.ConnectBucket.CpuNum)

	// 初始化 logic 的 rpc 客户端
	if err := c.InitLogicRpcClient(); err != nil {
		logrus.Panicf("InitLogicRpcClient err:%s", err.Error())
	}

	// 创建 buckets 切片，大小为 cpuNum = 4 个
	Buckets := make([]*Bucket, connectConfig.ConnectBucket.CpuNum)
	for i := 0; i < connectConfig.ConnectBucket.CpuNum; i++ {

		// 初始化每个*Bucket对象
		Buckets[i] = NewBucket(BucketOptions{
			ChannelSize:   connectConfig.ConnectBucket.Channel,       // 1024
			RoomSize:      connectConfig.ConnectBucket.Room,          // 1024
			RoutineAmount: connectConfig.ConnectBucket.RoutineAmount, // 32
			RoutineSize:   connectConfig.ConnectBucket.RoutineSize,   // 20
		})
	}
	operator := new(DefaultOperator)
	// 初始化 DefaultServer 对象
	DefaultServer = NewServer(Buckets, operator, ServerOptions{
		WriteWait:       10 * time.Second,
		PongWait:        60 * time.Second,
		PingPeriod:      54 * time.Second,
		MaxMessageSize:  512,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		BroadcastSize:   512, // broadcast 通道的缓冲区大小
	})
	// 定义服务的serverId
	c.ServerId = fmt.Sprintf("%s-%s", "ws", uuid.New().String())
	// 启动 rpc 服务
	if err := c.InitConnectWebsocketRpcServer(); err != nil {
		logrus.Panicf("InitConnectWebsocketRpcServer Fatal error: %s \n", err.Error())
	}

	// 启动 websocket 服务
	if err := c.InitWebsocket(); err != nil {
		logrus.Panicf("Connect layer InitWebsocket() error:  %s \n", err.Error())
	}
}

func (c *Connect) RunTcp() {
	// get Connect layer config
	connectConfig := config.Conf.Connect

	//set the maximum number of CPUs that can be executing
	runtime.GOMAXPROCS(connectConfig.ConnectBucket.CpuNum)

	//init logic layer rpc client, call logic layer rpc server
	if err := c.InitLogicRpcClient(); err != nil {
		logrus.Panicf("InitLogicRpcClient err:%s", err.Error())
	}
	//init Connect layer rpc server, logic client will call this
	Buckets := make([]*Bucket, connectConfig.ConnectBucket.CpuNum)
	for i := 0; i < connectConfig.ConnectBucket.CpuNum; i++ {
		Buckets[i] = NewBucket(BucketOptions{
			ChannelSize:   connectConfig.ConnectBucket.Channel,
			RoomSize:      connectConfig.ConnectBucket.Room,
			RoutineAmount: connectConfig.ConnectBucket.RoutineAmount,
			RoutineSize:   connectConfig.ConnectBucket.RoutineSize,
		})
	}
	operator := new(DefaultOperator)
	DefaultServer = NewServer(Buckets, operator, ServerOptions{
		WriteWait:       10 * time.Second,
		PongWait:        60 * time.Second,
		PingPeriod:      54 * time.Second,
		MaxMessageSize:  512,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		BroadcastSize:   512,
	})
	//go func() {
	//	http.ListenAndServe("0.0.0.0:9000", nil)
	//}()
	c.ServerId = fmt.Sprintf("%s-%s", "tcp", uuid.New().String())
	//init Connect layer rpc server ,task layer will call this
	if err := c.InitConnectTcpRpcServer(); err != nil {
		logrus.Panicf("InitConnectWebsocketRpcServer Fatal error: %s \n", err.Error())
	}
	//start Connect layer server handler persistent connection by tcp
	if err := c.InitTcpServer(); err != nil {
		logrus.Panicf("Connect layerInitTcpServer() error:%s\n ", err.Error())
	}
}
