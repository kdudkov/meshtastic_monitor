package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	pb "mesh/internal/meshtastic"
)

const (
	maxDatagramSize = 8192
)

type MulticastListener struct {
	logger *slog.Logger
	addr   string
	cb     func(msg *pb.FromRadio)
}

func NewMulticast(cb func(msg *pb.FromRadio)) *MulticastListener {
	return &MulticastListener{
		addr:   "224.0.0.69:4403",
		logger: slog.Default(),
		cb:     cb,
	}
}

func (m *MulticastListener) Start(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp4", m.addr)
	if err != nil {
		log.Fatal(err)
	}

	ifs, err := getIface()

	if len(ifs) == 0 {
		m.logger.Error("no interfaces")

		return
	}

	wg := sync.WaitGroup{}
	wg.Add(len(ifs))

	for _, i := range ifs {
		go m.listen(ctx, i, addr, &wg)
	}

	wg.Wait()
}

func (m *MulticastListener) listen(ctx context.Context, i *net.Interface, addr *net.UDPAddr, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.ListenMulticastUDP("udp4", i, addr)
	if err != nil {
		log.Fatal(err)
	}

	_ = conn.SetReadBuffer(maxDatagramSize)

	for ctx.Err() == nil {
		buffer := make([]byte, maxDatagramSize)
		numBytes, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}

		msg := new(pb.FromRadio)

		if err = proto.Unmarshal(buffer[:numBytes], msg); err != nil {
			m.logger.Warn("error", slog.Any("error", err))

			continue
		}

		m.cb(msg)
	}
}

func getIface() ([]*net.Interface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	res := make([]*net.Interface, 0)

	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 ||
			i.Flags&net.FlagMulticast == 0 ||
			i.Flags&net.FlagLoopback != 0 ||
			i.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		addrs, _ := i.Addrs()

		var hasv4 bool

		for _, a := range addrs {
			if strings.ContainsRune(a.String(), '.') {
				hasv4 = true
			}
		}

		if hasv4 {
			res = append(res, &i)
			fmt.Println(i.Name, i.Flags)
		}
	}

	return res, nil
}
