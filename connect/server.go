/**
 * Created by nash
 * Date: 2019-08-10
 * Time: 18:32
 */
package connect

import (
	"encoding/json"
	"fmt"
	"gochat/proto"
	"gochat/tools"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Server struct {
	Buckets   []*Bucket
	Options   ServerOptions
	bucketIdx uint32
	operator  Operator
}

type ServerOptions struct {
	WriteWait       time.Duration
	PongWait        time.Duration
	PingPeriod      time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
	BroadcastSize   int
}

func NewServer(b []*Bucket, o Operator, options ServerOptions) *Server {
	s := new(Server)
	s.Buckets = b
	s.Options = options
	s.bucketIdx = uint32(len(b))
	s.operator = o
	return s
}

// reduce lock competition, use google city hash insert to different bucket
func (s *Server) Bucket(userId int) *Bucket {
	userIdStr := fmt.Sprintf("%d", userId)
	// 基于userid计算出一个hash值，然后对s.bucketIdx求余数
	idx := tools.CityHash32([]byte(userIdStr), uint32(len(userIdStr))) % s.bucketIdx
	return s.Buckets[idx]
}

// 负责读取 ch.broadcast 中的数据，然后通过websocket发送给客户端
func (s *Server) writePump(ch *Channel, c *Connect) {
	//PingPeriod default eq 54s
	ticker := time.NewTicker(s.Options.PingPeriod) // 定时器（用于检查连接是否已经断开）
	defer func() {
		ticker.Stop()
		ch.conn.Close()
	}()

	// 死循环
	for {
		select {
		// 读取 ch.broadcast 数据
		case message, ok := <-ch.broadcast:
			//write data dead time , like http timeout , default 10s
			ch.conn.SetWriteDeadline(time.Now().Add(s.Options.WriteWait))
			if !ok { // 说明 ch.broadcast 关闭了
				logrus.Warn("SetWriteDeadline not ok")
				ch.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 写入文本
			w, err := ch.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logrus.Warnf(" ch.conn.NextWriter err :%s  ", err.Error())
				return
			}
			logrus.Infof("message write body:%s", message.Body)
			// 向前端发送数据
			w.Write(message.Body)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			ch.conn.SetWriteDeadline(time.Now().Add(s.Options.WriteWait))
			logrus.Infof("websocket.PingMessage :%v", websocket.PingMessage)
			// ping测试下 websocket连接是否已经断开
			if err := ch.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// 读取客户端发送来的数据，目的在于将 *Channel对象保存到 *Room中
func (s *Server) readPump(ch *Channel, c *Connect) {
	defer func() {
		logrus.Infof("start exec disConnect ...")
		if ch.Room == nil || ch.userId == 0 {
			logrus.Infof("roomId and userId eq 0")
			ch.conn.Close()
			return
		}
		logrus.Infof("exec disConnect ...")
		disConnectRequest := new(proto.DisConnectRequest)
		disConnectRequest.RoomId = ch.Room.Id
		disConnectRequest.UserId = ch.userId
		s.Bucket(ch.userId).DeleteChannel(ch)
		if err := s.operator.DisConnect(disConnectRequest); err != nil {
			logrus.Warnf("DisConnect err :%s", err.Error())
		}
		ch.conn.Close()
	}()

	ch.conn.SetReadLimit(s.Options.MaxMessageSize)
	ch.conn.SetReadDeadline(time.Now().Add(s.Options.PongWait))
	ch.conn.SetPongHandler(func(string) error {
		ch.conn.SetReadDeadline(time.Now().Add(s.Options.PongWait))
		return nil
	})

	// 死循环
	for {

		// 从websocket中读取客户端发送来的数据
		_, message, err := ch.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("readPump ReadMessage err:%s", err.Error())
				return
			}
		}
		if message == nil {
			return
		}
		var connReq *proto.ConnectRequest
		logrus.Infof("get a message :%s", message)

		// json解析，客户端主要传递的是 AuthToken + RoomId 这两个参数
		if err := json.Unmarshal([]byte(message), &connReq); err != nil {
			logrus.Errorf("message struct %+v", connReq)
		}
		if connReq == nil || connReq.AuthToken == "" {
			logrus.Errorf("s.operator.Connect no authToken")
			return
		}

		connReq.ServerId = c.ServerId // 当前websocket服务的serverId

		// 调用 logic服务的 Connect函数（基于AuthToken从redis中获取userId）
		userId, err := s.operator.Connect(connReq)
		if err != nil {
			logrus.Errorf("s.operator.Connect error %s", err.Error())
			return
		}
		if userId == 0 {
			logrus.Error("Invalid AuthToken ,userId empty")
			return
		}
		logrus.Infof("websocket rpc call return userId:%d,RoomId:%d", userId, connReq.RoomId)

		// 映射 userid 属于哪个 *Bucket 对象（减小锁力度）
		b := s.Bucket(userId)
		// userId 表示客户端用户 ch 表示客户端的socket（封装）  connReq.RoomId 表示当前所在的房间
		err = b.Put(userId, connReq.RoomId, ch) // b.Put 函数的目的：将 *Channel对象，保存到 *Room的链表中
		if err != nil {
			logrus.Errorf("conn close err: %s", err.Error())
			ch.conn.Close()
		}
	}
}
