package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	pb "mesh/internal/meshtastic"
)

type TCPConnector struct {
	log    *slog.Logger
	addr   string
	conn   net.Conn
	ch     chan *pb.ToRadio
	status uint32
	cb     func(msg *pb.FromRadio)
}

func NewTCP(addr string, cb func(msg *pb.FromRadio)) *TCPConnector {
	return &TCPConnector{
		addr: addr,
		log:  slog.Default(),
		ch:   make(chan *pb.ToRadio, 64),
		cb:   cb,
	}
}

func (r *TCPConnector) Start(ctx context.Context) {
	if r.log == nil {
		r.log = slog.Default().With(slog.String("logger", "tcp"))
	}

	r.log.Info("start tcp connector")

	if conn, err := net.DialTimeout("tcp", r.addr, time.Second*3); err != nil {
		r.log.Error("error dialing", slog.Any("error", err))

		return
	} else {
		r.conn = conn
	}

	r.log.Info("connected")
	fmt.Println("connected")

	r.SetConnected()
	go r.writeLoop()
	r.SendMsg(ConfigMessage())

	r.readLoop(ctx)

	r.stop()
}

func (r *TCPConnector) IsConnected() bool {
	return atomic.LoadUint32(&r.status) == 1
}

func (r *TCPConnector) SetConnected() {
	atomic.StoreUint32(&r.status, 1)
}

func (r *TCPConnector) stop() {
	if atomic.CompareAndSwapUint32(&r.status, 1, 0) {
		if r.conn != nil {
			if err := r.conn.Close(); err != nil {
				r.log.Error("error closing connection", slog.Any("error", err))
			}
		}

		close(r.ch)
	}
}

func (r *TCPConnector) SendMsg(msg *pb.ToRadio) bool {
	if !r.IsConnected() {
		return false
	}

	select {
	case r.ch <- msg:
		return true
	default:
		return false
	}
}

func (r *TCPConnector) writeLoop() {
	for msg := range r.ch {
		if !r.IsConnected() {
			return
		}

		if err := WriteMsg(r.conn, msg); err != nil {
			r.log.Error("write error", slog.Any("error", err))
			r.stop()

			return
		}
	}
}

func (r *TCPConnector) readLoop(ctx context.Context) {
	buf := bufio.NewReader(r.conn)

	for ctx.Err() == nil {
		msg, err := ReadMsg(ctx, buf)

		if err != nil {
			r.log.Error("loop error", slog.Any("error", err))

			return
		}

		r.cb(msg)
	}
}
