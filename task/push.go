/**
 * Created by nash
 * Date: 2019-08-13
 * Time: 10:50
 */
package task

import (
	"encoding/json"
	"gochat/config"
	"gochat/proto"
	"math/rand"

	"github.com/sirupsen/logrus"
)

type PushParams struct {
	ServerId string
	UserId   int
	Msg      []byte
	RoomId   int
}

var pushChannel []chan *PushParams

func init() {
	// 2 个channel
	pushChannel = make([]chan *PushParams, config.Conf.Task.TaskBase.PushChan)
}

func (task *Task) GoPush() {

	for i := 0; i < len(pushChannel); i++ {
		// 每个channel的缓冲区大小50
		pushChannel[i] = make(chan *PushParams, config.Conf.Task.TaskBase.PushChanSize)
		go task.processSinglePush(pushChannel[i])
	}
}

func (task *Task) processSinglePush(ch chan *PushParams) {
	var arg *PushParams
	for {
		// 读通道元素
		arg = <-ch
		//@todo when arg.ServerId server is down, user could be reconnect other serverId but msg in queue no consume
		task.pushSingleToConnect(arg.ServerId, arg.UserId, arg.Msg)
	}
}

func (task *Task) Push(msg string) {

	// 解析redis中的数据包
	m := &proto.RedisMsg{}
	if err := json.Unmarshal([]byte(msg), m); err != nil {
		logrus.Infof(" json.Unmarshal err:%v ", err)
	}
	logrus.Infof("push msg info %d,op is:%d", m.RoomId, m.Op)
	switch m.Op {
	case config.OpSingleSend: // 发送到个人的数据包
		// 随机选一个 pushChannel 保存 PushParams （ 处理的逻辑在👆上面的 task.pushSingleToConnect 函数中）
		pushChannel[rand.Int()%config.Conf.Task.TaskBase.PushChan] <- &PushParams{
			ServerId: m.ServerId,
			UserId:   m.UserId,
			Msg:      m.Msg,
		}
	case config.OpRoomSend: // 发送到房间中的数据包
		task.broadcastRoomToConnect(m.RoomId, m.Msg)
	case config.OpRoomCountSend:
		task.broadcastRoomCountToConnect(m.RoomId, m.Count)
	case config.OpRoomInfoSend:
		task.broadcastRoomInfoToConnect(m.RoomId, m.RoomUserInfo)
	}
}
