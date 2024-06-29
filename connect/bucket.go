/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 15:18
 */
package connect

import (
	"gochat/proto"
	"sync"
	"sync/atomic"
)

type Bucket struct {
	cLock         sync.RWMutex     // protect the channels for chs
	chs           map[int]*Channel // 记录用户id 和 用户的*Channel的映射
	bucketOptions BucketOptions
	rooms         map[int]*Room // 房间id 和房间对象的映射 bucket room channels
	routines      []chan *proto.PushRoomMsgRequest
	routinesNum   uint64
	broadcast     chan []byte
}

type BucketOptions struct {
	ChannelSize   int
	RoomSize      int
	RoutineAmount uint64
	RoutineSize   int
}

func NewBucket(bucketOptions BucketOptions) (b *Bucket) {
	b = new(Bucket)
	b.chs = make(map[int]*Channel, bucketOptions.ChannelSize) // 1024 Channel 对象
	b.bucketOptions = bucketOptions
	b.routines = make([]chan *proto.PushRoomMsgRequest, bucketOptions.RoutineAmount) // 32 个 channel
	b.rooms = make(map[int]*Room, bucketOptions.RoomSize)                            // 1024 Room 房间

	for i := uint64(0); i < b.bucketOptions.RoutineAmount; i++ {
		c := make(chan *proto.PushRoomMsgRequest, bucketOptions.RoutineSize) // 初始化chan，容量 20
		b.routines[i] = c
		go b.PushRoom(c) // 启动32个协程，消费32个chan
	}
	return
}

func (b *Bucket) PushRoom(ch chan *proto.PushRoomMsgRequest) {
	for {
		var (
			arg  *proto.PushRoomMsgRequest
			room *Room
		)
		arg = <-ch
		// 基于消息中的房间id， b.Room(arg.RoomId) 获取 *Room 房间对象
		if room = b.Room(arg.RoomId); room != nil { // room != nil 在该bucket下存在房间的时候，才会向房间中发送消息
			room.Push(&arg.Msg)
		}
	}
}

func (b *Bucket) Room(rid int) (room *Room) {
	b.cLock.RLock()
	room = b.rooms[rid]
	b.cLock.RUnlock()
	return
}

func (b *Bucket) Put(userId int, roomId int, ch *Channel) (err error) {
	var (
		room *Room
		ok   bool
	)
	b.cLock.Lock()
	if roomId != NoRoom {
		if room, ok = b.rooms[roomId]; !ok { // 查找roomId对应的 *Room对象
			room = NewRoom(roomId) // 不存在，创建一个新的 *Room对象
			b.rooms[roomId] = room
		}
		ch.Room = room
	}
	// 记录 userId 和 ch 之间的互相应用关系
	ch.userId = userId
	b.chs[userId] = ch
	b.cLock.Unlock()

	// 我理解：这里是不能同时加入不同的房间的，一旦加入一个房间会把ch，保存到该房间的链表中（等价于自动从其他房间的链表中退出了）
	if room != nil {
		err = room.Put(ch) // 将 *Channel保存到房间*Room的双向链表中
	}
	return
}

func (b *Bucket) DeleteChannel(ch *Channel) {
	var (
		ok   bool
		room *Room
	)
	b.cLock.RLock()
	if ch, ok = b.chs[ch.userId]; ok {
		room = b.chs[ch.userId].Room
		//delete from bucket
		delete(b.chs, ch.userId) // 解除 userId 和 ch的映射
	}
	if room != nil && room.DeleteChannel(ch) { // 从房间的 *Channel链表中 删除 ch
		// 如果房间中没有用户了，标记可以删除房间
		if room.drop {
			delete(b.rooms, room.Id) // b.rooms中删除房间
		}
	}
	b.cLock.RUnlock()
}

func (b *Bucket) Channel(userId int) (ch *Channel) {
	b.cLock.RLock()
	ch = b.chs[userId]
	b.cLock.RUnlock()
	return
}

func (b *Bucket) BroadcastRoom(pushRoomMsgReq *proto.PushRoomMsgRequest) {
	// 按照轮询的方式,选择 b.routines
	num := atomic.AddUint64(&b.routinesNum, 1) % b.bucketOptions.RoutineAmount
	b.routines[num] <- pushRoomMsgReq
}
