/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 18:22
 */
package task

import (
	"gochat/config"
	"runtime"

	"github.com/sirupsen/logrus"
)

type Task struct {
}

func New() *Task {
	return new(Task)
}

func (task *Task) Run() {
	//读取配置，也就是gochat/config包中的init函数
	taskConfig := config.Conf.Task
	runtime.GOMAXPROCS(taskConfig.TaskBase.CpuNum)
	//从redis链表中读取信息
	if err := task.InitQueueRedisClient(); err != nil {
		logrus.Panicf("task init publishRedisClient fail,err:%s", err.Error())
	}
	// 初始化 connect 服务的 rpc客户端
	if err := task.InitConnectRpcClient(); err != nil {
		logrus.Panicf("task init InitConnectRpcClient fail,err:%s", err.Error())
	}
	// GoPush
	task.GoPush()
}
