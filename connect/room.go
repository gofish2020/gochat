/**
 * Created by nash
 * Date: 2019-08-09
 * Time: 15:18
 */
package connect

import (
	"gochat/proto"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const NoRoom = -1

type Room struct {
	Id          int
	OnlineCount int // room online user count
	rLock       sync.RWMutex
	drop        bool     // make room is live
	next        *Channel // 表示根节点（用来指向归属于当前房间【用户socket】链表头节点）
}

func NewRoom(roomId int) *Room {
	room := new(Room)
	room.Id = roomId
	room.drop = false
	room.next = nil
	room.OnlineCount = 0
	return room
}

func (r *Room) Put(ch *Channel) (err error) {
	//doubly linked list
	r.rLock.Lock()
	defer r.rLock.Unlock()
	if !r.drop {
		if r.next != nil {
			r.next.Prev = ch
		}
		// 表示插入到链表的头节点位置
		ch.Next = r.next
		ch.Prev = nil
		r.next = ch // 更新 根节点
		r.OnlineCount++
	} else {
		err = errors.New("room drop")
	}
	return
}

func (r *Room) Push(msg *proto.Msg) {
	r.rLock.RLock()

	// 遍历房间中的所有的客户端socket（tcp or websocket）
	for ch := r.next; ch != nil; ch = ch.Next {
		// 将消息发送给客户端
		if err := ch.Push(msg); err != nil {
			logrus.Infof("push msg err:%s", err.Error())
		}
	}
	r.rLock.RUnlock()
}

func (r *Room) DeleteChannel(ch *Channel) bool {
	r.rLock.RLock()
	if ch.Next != nil {
		//if not footer
		ch.Next.Prev = ch.Prev
	}
	if ch.Prev != nil {
		// if not header
		ch.Prev.Next = ch.Next
	} else {
		r.next = ch.Next
	}
	r.OnlineCount--
	r.drop = false
	// 删除完成以后，如果房间中没人了，标记房间也可以删除
	if r.OnlineCount <= 0 {
		r.drop = true
	}
	r.rLock.RUnlock()
	return r.drop
}
