/**
 * Created by nash
 * Date: 2020/4/14
 */
package connect

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"gochat/api/rpc"
	"gochat/config"
	"gochat/pkg/stickpackage"
	"gochat/proto"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const maxInt = 1<<31 - 1 // 2^31-1

func init() {
	rpc.InitLogicRpcClient()
}

func (c *Connect) InitTcpServer() error {
	aTcpAddr := strings.Split(config.Conf.Connect.ConnectTcp.Bind, ",") // 0.0.0.0:7001,0.0.0.0:7002
	cpuNum := config.Conf.Connect.ConnectBucket.CpuNum                  // cpuNum = 4
	var (
		addr     *net.TCPAddr
		listener *net.TCPListener
		err      error
	)

	// 创建多个tcp服务
	for _, ipPort := range aTcpAddr {
		if addr, err = net.ResolveTCPAddr("tcp", ipPort); err != nil {
			logrus.Errorf("server_tcp ResolveTCPAddr error:%s", err.Error())
			return err
		}
		if listener, err = net.ListenTCP("tcp", addr); err != nil {
			logrus.Errorf("net.ListenTCP(tcp, %s),error(%v)", ipPort, err)
			return err
		}
		logrus.Infof("start tcp listen at:%s", ipPort)
		// 每个tcp服务，启动 4个协程，监听客户端的socket连接
		for i := 0; i < cpuNum; i++ {
			go c.acceptTcp(listener)
		}
	}
	return nil
}

func (c *Connect) acceptTcp(listener *net.TCPListener) {
	var (
		conn *net.TCPConn
		err  error
		r    int
	)
	connectTcpConfig := config.Conf.Connect.ConnectTcp

	// 死循环
	for {
		// 等待tcp socket连接
		if conn, err = listener.AcceptTCP(); err != nil {
			logrus.Errorf("listener.Accept(\"%s\") error(%v)", listener.Addr().String(), err)
			return
		}
		// set keep alive，client==server ping package check
		if err = conn.SetKeepAlive(connectTcpConfig.KeepAlive); err != nil {
			logrus.Errorf("conn.SetKeepAlive() error:%s", err.Error())
			return
		}
		//set ReceiveBuf
		if err := conn.SetReadBuffer(connectTcpConfig.ReceiveBuf); err != nil {
			logrus.Errorf("conn.SetReadBuffer() error:%s", err.Error())
			return
		}
		//set SendBuf
		if err := conn.SetWriteBuffer(connectTcpConfig.SendBuf); err != nil {
			logrus.Errorf("conn.SetWriteBuffer() error:%s", err.Error())
			return
		}

		// 每个socket客户端，单独启动一个协程进行服务
		go c.ServeTcp(DefaultServer, conn, r)
		if r++; r == maxInt {
			logrus.Infof("conn.acceptTcp num is:%d", r)
			r = 0
		}
	}
}

func (c *Connect) ServeTcp(server *Server, conn *net.TCPConn, r int) {

	// 这里的逻辑和websocket的类似
	ch := NewChannel(server.Options.BroadcastSize)
	ch.connTcp = conn
	go c.writeDataToTcp(server, ch)
	go c.readDataFromTcp(server, ch)
}

func (c *Connect) readDataFromTcp(s *Server, ch *Channel) {
	defer func() {
		logrus.Infof("start exec disConnect ...")
		if ch.Room == nil || ch.userId == 0 {
			logrus.Infof("roomId and userId eq 0")
			_ = ch.connTcp.Close()
			return
		}
		logrus.Infof("exec disConnect ...")
		disConnectRequest := new(proto.DisConnectRequest)
		disConnectRequest.RoomId = ch.Room.Id
		disConnectRequest.UserId = ch.userId
		s.Bucket(ch.userId).DeleteChannel(ch)
		if err := s.operator.DisConnect(disConnectRequest); err != nil {
			logrus.Warnf("DisConnect rpc err :%s", err.Error())
		}
		if err := ch.connTcp.Close(); err != nil {
			logrus.Warnf("DisConnect close tcp conn err :%s", err.Error())
		}
		return
	}()
	// scannerPackage 就是 ch.connTcp
	scannerPackage := bufio.NewScanner(ch.connTcp)
	scannerPackage.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if !atEOF && data[0] == 'v' {
			if len(data) > stickpackage.TcpHeaderLength { // 大于4字节
				packSumLength := int16(0)

				// 读取 2字节的数据长度，保存到packSumLength中
				_ = binary.Read(bytes.NewReader(data[stickpackage.LengthStartIndex:stickpackage.LengthStopIndex]), binary.BigEndian, &packSumLength)
				// 说明data中有足够的数据
				if int(packSumLength) <= len(data) {
					return int(packSumLength), data[:packSumLength], nil
				}
			}
		}
		return
	})
	scanTimes := 0
	for {
		scanTimes++
		if scanTimes > 3 {
			logrus.Infof("scannedPack times is:%d", scanTimes)
			break
		}
		for scannerPackage.Scan() {
			scannedPack := new(stickpackage.StickPackage)
			// 读取一个数据包
			err := scannedPack.Unpack(bytes.NewReader(scannerPackage.Bytes()))
			if err != nil {
				logrus.Errorf("scan tcp package err:%s", err.Error())
				break
			}

			var connReq proto.ConnectRequest
			logrus.Infof("get a tcp message :%s", scannedPack)
			var rawTcpMsg proto.SendTcp
			// 解析Msg值
			if err := json.Unmarshal([]byte(scannedPack.Msg), &rawTcpMsg); err != nil {
				logrus.Errorf("tcp message struct %+v", rawTcpMsg)
				break
			}
			logrus.Infof("json unmarshal,raw tcp msg is:%+v", rawTcpMsg)
			if rawTcpMsg.AuthToken == "" {
				logrus.Errorf("tcp s.operator.Connect no authToken")
				return
			}
			if rawTcpMsg.RoomId <= 0 {
				logrus.Errorf("tcp roomId not allow lgt 0")
				return
			}

			switch rawTcpMsg.Op {
			case config.OpBuildTcpConn: // 这里的逻辑和websocket一样
				connReq.AuthToken = rawTcpMsg.AuthToken
				connReq.RoomId = rawTcpMsg.RoomId
				//fix
				//connReq.ServerId = config.Conf.Connect.ConnectTcp.ServerId
				connReq.ServerId = c.ServerId
				userId, err := s.operator.Connect(&connReq)
				logrus.Infof("tcp s.operator.Connect userId is :%d", userId)
				if err != nil {
					logrus.Errorf("tcp s.operator.Connect error %s", err.Error())
					return
				}
				if userId == 0 {
					logrus.Error("tcp Invalid AuthToken ,userId empty")
					return
				}
				b := s.Bucket(userId)
				//insert into a bucket
				err = b.Put(userId, connReq.RoomId, ch) // 将 ch加入到 *room中
				if err != nil {
					logrus.Errorf("tcp conn put room err: %s", err.Error())
					_ = ch.connTcp.Close()
					return
				}

			case config.OpRoomSend:
				// 通过tcp的方式，发送消息（正常都是通过http调用api服务发送消息）（应该是作者演示怎么用tcp发送消息）
				req := &proto.Send{
					Msg:          rawTcpMsg.Msg,
					FromUserId:   rawTcpMsg.FromUserId,
					FromUserName: rawTcpMsg.FromUserName,
					RoomId:       rawTcpMsg.RoomId,
					Op:           config.OpRoomSend,
				}
				code, msg := rpc.RpcLogicObj.PushRoom(req)
				logrus.Infof("tcp conn push msg to room,err code is:%d,err msg is:%s", code, msg)
			}
		}
		if err := scannerPackage.Err(); err != nil {
			logrus.Errorf("tcp get a err package:%s", err.Error())
			return
		}
	}
}

func (c *Connect) writeDataToTcp(s *Server, ch *Channel) {
	//ping time default 54s
	ticker := time.NewTicker(DefaultServer.Options.PingPeriod)
	defer func() {
		ticker.Stop()
		_ = ch.connTcp.Close()
		return
	}()
	pack := stickpackage.StickPackage{
		Version: stickpackage.VersionContent,
	}
	for {
		select {
		case message, ok := <-ch.broadcast:
			if !ok {
				_ = ch.connTcp.Close()
				return
			}
			pack.Msg = message.Body
			pack.Length = pack.GetPackageLength()

			logrus.Infof("send tcp msg to conn:%s", pack.String())

			// tcp 发包，包的结构为 【 2字节版本号 + 2字节长度 + Msg 数据包 】
			if err := pack.Pack(ch.connTcp); err != nil {
				logrus.Errorf("connTcp.write message err:%s", err.Error())
				return
			}
		case <-ticker.C:
			logrus.Infof("connTcp.ping message,send")
			//send a ping msg ,if error , return
			pack.Msg = []byte("ping msg")
			pack.Length = pack.GetPackageLength()
			if err := pack.Pack(ch.connTcp); err != nil {
				//send ping msg to tcp conn
				return
			}
		}
	}
}
