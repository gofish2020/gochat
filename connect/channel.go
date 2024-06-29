/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 15:18
 */
package connect

import (
	"gochat/proto"
	"net"

	"github.com/gorilla/websocket"
)

// Channel 表示一个【客户端连接】的封装对象
type Channel struct {
	Room      *Room // 表示 Channel 归属的房间对象
	Next      *Channel
	Prev      *Channel
	broadcast chan *proto.Msg // 存储要发送的数据
	userId    int             // 表示Channel归属的用户id
	conn      *websocket.Conn //  websocket 句柄
	connTcp   *net.TCPConn    //  tcp 句柄
}

func NewChannel(size int) (c *Channel) {
	c = new(Channel)
	c.broadcast = make(chan *proto.Msg, size) // 512
	c.Next = nil
	c.Prev = nil
	return
}

// 这里没有直接调用 conn/connTcp 发送，而是放到 broadcast 通道中（writeDataToTcp/writePump 函数读取broadcast将数据发送出去 ）
func (ch *Channel) Push(msg *proto.Msg) (err error) {
	select {
	case ch.broadcast <- msg:
	default: // 如果消息很多，把broadcast容量打满了，就存在消息的丢弃。。。
	}
	return
}
