package main

import (
	"net"
	"sync"
)

type boundedListener struct {
	net.Listener
	slots chan struct{}
}
type boundedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *boundedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
func (l *boundedListener) Accept() (net.Conn, error) {
	l.slots <- struct{}{}
	c, e := l.Listener.Accept()
	if e != nil {
		<-l.slots
		return nil, e
	}
	return &boundedConn{Conn: c, release: func() { <-l.slots }}, nil
}
