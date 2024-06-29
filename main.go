/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 10:56
 */
package main

import (
	"flag"
	"fmt"
	"gochat/api"
	"gochat/connect"
	"gochat/logic"
	"gochat/site"
	"gochat/task"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var module string
	flag.StringVar(&module, "module", "", "assign run module")
	flag.Parse()
	fmt.Println("start ", module)
	switch module {

	case "site":
		site.New().Run() // 这里提供一个前端UI（目录在 ./site） 服务地址  http://127.0.0.1:8080/login

	case "api":
		api.New().Run() // 对外的api服务端口7070(供前端UI调用）内部通过 rpc（通过etcd进行的服务发现）调用logic服务的实际业务逻辑代码

	case "logic":
		// logic rpc 服务 对api提供服务，并且将消息数据保存到queue中（这里queue本质是redis链表）
		// 服务端口 tcp@127.0.0.1:6900,tcp@127.0.0.1:6901
		logic.New().Run()

	case "task":
		task.New().Run() // 负责从 queue中读取任务，然后通过 rpc 调用connect服务

	case "connect_websocket":
		connect.New().Run() // 提供websocket服务，推送数据到前端页面 && 提供rpc服务，供task任务调用
	case "connect_tcp":
		connect.New().RunTcp()

	default:
		fmt.Println("exiting,module param error!")
		return
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	fmt.Println("Server exiting")
}
