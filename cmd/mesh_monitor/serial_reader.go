package main

import (
	"bufio"
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"go.bug.st/serial"

	pb "mesh/internal/meshtastic"
)

type SerialConnector struct {
	logger *slog.Logger
	addr   string
	port   serial.Port
	ch     chan *pb.ToRadio
	status uint32
	cb     func(msg *pb.FromRadio)
	cancel context.CancelFunc
}

func NewSerial(addr string, cb func(msg *pb.FromRadio)) *SerialConnector {
	return &SerialConnector{
		addr:   addr,
		logger: slog.Default(),
		ch:     make(chan *pb.ToRadio, 64),
		cb:     cb,
	}
}

func (r *SerialConnector) Start(ctx context.Context) {
	if r.logger == nil {
		r.logger = slog.Default().With(slog.String("logger", "reader"))
	}

	if err := r.openPort(); err != nil {
		r.logger.Error("error opening port", slog.Any("error", err))

		return
	}

	var ctx1 context.Context
	
	ctx1, r.cancel = context.WithCancel(ctx)
	
	r.SetConnected()
	go r.writeLoop()
	go r.pinger(ctx1)
	r.SendMsg(ConfigMessage())

	r.readLoop(ctx1)

	r.stop()
}

func (r *SerialConnector) IsConnected() bool {
	return atomic.LoadUint32(&r.status) == 1
}

func (r *SerialConnector) SetConnected() {
	atomic.StoreUint32(&r.status, 1)
}

func (r *SerialConnector) stop() {
	if atomic.CompareAndSwapUint32(&r.status, 1, 0) {
		if r.port != nil {
			if err := r.port.Close(); err != nil {
				r.logger.Error("error closing port", slog.Any("error", err))
			}
		}

		close(r.ch)
		r.cancel()
	}
}

func (r *SerialConnector) SendMsg(msg *pb.ToRadio) bool {
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

func (r *SerialConnector) openPort() error {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{
			RTS: true,
			DTR: true,
		},
	}

	port, err := serial.Open(r.addr, mode)

	if err != nil {
		return err
	}

	b := make([]byte, 32)

	for i := range b {
		b[i] = start2
	}

	_, err = port.Write(b)

	if err != nil {
		return err
	}

	r.port = port

	return nil
}

func (r *SerialConnector) writeLoop() {
	for msg := range r.ch {
		if !r.IsConnected() {
			return
		}

		if err := r.writeMsg(msg); err != nil {
			r.logger.Error("write error", slog.Any("error", err))
			r.stop()

			return
		}
	}
}

func (r *SerialConnector) readLoop(ctx context.Context) {
	r.port.SetReadTimeout(time.Minute)

	buf := bufio.NewReader(r.port)

	for ctx.Err() == nil {
		msg, err := ReadMsg(ctx, buf)

		if err != nil {
			r.logger.Error("loop error", slog.Any("error", err))

			return
		}

		r.cb(msg)
	}
}

func (r *SerialConnector) pinger(ctx context.Context) {
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.SendMsg(Heartbeat())
		}
	}
}

func (r *SerialConnector) writeMsg(msg *pb.ToRadio) error {
	if err := WriteMsg(r.port, msg); err != nil {
		return err
	}

	return r.port.Drain()
}
