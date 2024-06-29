/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 15:19
 */
package connect

import (
	"gochat/config"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func (c *Connect) InitWebsocket() error {

	// 创建 websocket服务
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c.serveWs(DefaultServer, w, r)
	})
	err := http.ListenAndServe(config.Conf.Connect.ConnectWebsocket.Bind, nil) // 0.0.0.0:7000
	return err
}

// 调用一次serveWs函数，就建立一个新的【客户端和服务器的】socket长连接
func (c *Connect) serveWs(server *Server, w http.ResponseWriter, r *http.Request) {

	var upGrader = websocket.Upgrader{
		ReadBufferSize:  server.Options.ReadBufferSize,
		WriteBufferSize: server.Options.WriteBufferSize,
	}
	//cross origin domain support
	upGrader.CheckOrigin = func(r *http.Request) bool { return true } // 允许跨域访问

	// conn 表示一个 websocket 客户端的句柄
	conn, err := upGrader.Upgrade(w, r, nil)

	if err != nil {
		logrus.Errorf("serverWs err:%s", err.Error())
		return
	}

	// 创建一个 *Channel对象：里面保存的就是【客户端的连接句柄】
	ch := NewChannel(server.Options.BroadcastSize) // 512
	ch.conn = conn

	// 启动协程，死循环发送数据
	go server.writePump(ch, c)
	// 启动协程，死循环获取客户端数据
	go server.readPump(ch, c)
}
