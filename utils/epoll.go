// +build linux

/*
 * @Author: calmwu
 * @Date: 2019-02-22 14:38:29
 * @Last Modified by: calmwu
 * @Last Modified time: 2019-02-22 20:06:27
 */

package utils

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"golang.org/x/sys/unix"
)

type EpollConnType int

const (
	EPOLLConnTypeTCPCONN EpollConnType = iota
	EPOLLConnTypeTCPLISTENER
	EPOLLConnTypeUDP
	EPOLLConnTypeWEBSOCKET
)

/*
const (
	POLLIN    = 0x1
	POLLPRI   = 0x2
	POLLOUT   = 0x4
	POLLRDHUP = 0x2000
	POLLERR   = 0x8
	POLLHUP   = 0x10
	POLLNVAL  = 0x20
)
*/
type EpollConn struct {
	ConnHolder    interface{}   // golang各种连接对象
	ConnArg       interface{}   // 附加参数
	ConnType      EpollConnType // 连接类型
	TriggerEvents uint32        // EpollEvent返回的事件类型
	SocketFD      int32
}

type Epoll struct {
	fd          int32
	connections map[int32]*EpollConn
	lock        *sync.RWMutex
}

func NewEpoll() (*Epoll, error) {
	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		connections: make(map[int32]*EpollConn),
	}, nil
}

func (ep *Epoll) Add(conn, connArg interface{}) (int32, error) {
	//ConnType := reflect.Indirect(reflect.ValueOf(conn)).Type()
	econn := &EpollConn{
		ConnHolder: conn,
		ConnArg:    connArg,
	}

	switch realConn := conn.(type) {
	case *net.TCPConn:
		econn.SocketFD = TcpConnSocketFD(realConn)
		econn.ConnType = EPOLLConnTypeTCPCONN
	case *net.TCPListener:
		econn.SocketFD = TcpListenerSocketFD(realConn)
		econn.ConnType = EPOLLConnTypeTCPLISTENER
	case *net.UDPConn:
		econn.SocketFD = UdpConnSocketFD(realConn)
		econn.ConnType = EPOLLConnTypeUDP
	case *websocket.Conn:
		econn.SocketFD = GorillaConnSocketFD(realConn)
		econn.ConnType = EPOLLConnTypeWEBSOCKET
	default:
		return -1, errors.New(fmt.Sprintf("conn type:%s is not support\n", reflect.Indirect(reflect.ValueOf(conn)).Type().Name()))
	}

	/*
		EPOLLIN:表示关联的fd可以进行读操作了。
		EPOLLOUT:表示关联的fd可以进行写操作了。
		EPOLLRDHUP(since Linux 2.6.17):表示套接字关闭了连接，或者关闭了正写一半的连接。
		EPOLLPRI:表示关联的fd有紧急优先事件可以进行读操作了。
		EPOLLERR:表示关联的fd发生了错误，epoll_wait会一直等待这个事件，所以一般没必要设置这个属性。
		EPOLLHUP:表示关联的fd挂起了，epoll_wait会一直等待这个事件，所以一般没必要设置这个属性。
		EPOLLET:设置关联的fd为ET的工作方式，epoll的默认工作方式是LT。
		EPOLLONESHOT (since Linux 2.6.2):设置关联的fd为one-shot的工作方式。表示只监听一次事件，如果要再次监听，需要把socket放入到epoll队列中。
	*/
	err := unix.EpollCtl(ep.fd, syscall.EPOLL_CTL_ADD, socketFD,
		&unix.EpollEvent{
			Events: unix.POLLIN | unix.POLLHUP | unix.EPOLLRDHUP | unix.EPOLLERR,
			Fd:     int32(socketFD),
		})
	if err != nil {
		return -1, err
	}
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.connections[socketFD] = econn
	return econn.SocketFD, nil
}

func (ep *Epoll) Remove(socketFD int32) error {
	err := unix.EpollCtl(ep.fd, syscall.EPOLL_CTL_DEL, socketFD, nil)
	if err != nil {
		return err
	}

	ep.lock.Lock()
	defer ep.lock.Unlock()
	delete(ep.connections, socketFD)
	return nil
}

func (ep *Epoll) Wait(milliseconds int) ([]*EpollConn, error) {
	events := make([]unix.EpollEvent, 1024)

	var n int
	var err error

	for {
		n, err = unix.EpollWait(ep.fd, events, milliseconds)
		if err != nil {
			if err == syscall.EINTR {
				continue
			} else {
				return nil, err
			}
		} else {
			break
		}
	}

	ep.lock.RLock()
	defer ep.lock.RUnlock()
	var connections []*EpollConn
	for i := 0; i < n; i++ {
		if conn, exist := ep.connections[int(events[i].Fd)]; exist {
			conn.TriggerEvents = events[i].Events
			connections = append(connections, conn)
		}
	}
	// 返回可读的网络连接
	return connections, nil
}

func TcpConnSocketFD(conn *net.TCPConn) int {
	// 就算是私有成员通过反射还是可以获取
	tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int32(pfdVal.FieldByName("Sysfd").Int())
}

func UdpConnSocketFD(conn *net.UDPConn) int32 {
	tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int32(pfdVal.FieldByName("Sysfd").Int())
}

func TcpListenerSocketFD(listener *net.TCPListener) int32 {
	fdVal := reflect.Indirect(reflect.ValueOf(listener)).FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int32(pfdVal.FieldByName("Sysfd").Int())
}

func GorillaConnSocketFD(conn *websocket.Conn) int32 {
	// Elem()从返回的interface中获取真实的对象
	connVal := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn").Elem()
	tcpConn := reflect.Indirect(connVal).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int32(pfdVal.FieldByName("Sysfd").Int())
}
