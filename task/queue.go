/**
 * Created by nash
 * Date: 2019-08-13
 * Time: 10:13
 */
package task

import (
	"gochat/config"
	"gochat/tools"
	"time"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

var RedisClient *redis.Client

func (task *Task) InitQueueRedisClient() (err error) {

	// redis的配置信息
	redisOpt := tools.RedisOption{
		Address:  config.Conf.Common.CommonRedis.RedisAddress,
		Password: config.Conf.Common.CommonRedis.RedisPassword,
		Db:       config.Conf.Common.CommonRedis.Db,
	}

	// 获取Redis客户端
	RedisClient = tools.GetRedisInstance(redisOpt)

	// 检测是否ping的通
	if pong, err := RedisClient.Ping().Result(); err != nil {
		logrus.Infof("RedisClient Ping Result pong: %s,  err: %s", pong, err)
	}
	go func() {
		for { // 死循环
			var result []string
			//10s timeout
			result, err = RedisClient.BRPop(time.Second*10, config.QueueName).Result()
			if err != nil {
				logrus.Infof("task queue block timeout,no msg err:%s", err.Error())
			}
			// 从 redis 链表gochat_queue中，读取元素，保存到 task.Push 中
			if len(result) >= 2 {
				task.Push(result[1])
			}
		}
	}()
	return
}
