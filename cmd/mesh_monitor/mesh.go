package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"

	"google.golang.org/protobuf/proto"

	pb "mesh/internal/meshtastic"
)

const (
	start1 byte = 0x94
	start2 byte = 0xc3
)

func ReadMsg(ctx context.Context, r *bufio.Reader) (*pb.FromRadio, error) {
	buf := bytes.NewBuffer(nil)
	pl := 0

	for ctx.Err() == nil {
		b, err := r.ReadByte()

		if err != nil {
			return nil, err
		}

		buf.WriteByte(b)

		if buf.Len() == 1 && buf.Bytes()[0] != start1 {
			buf.Reset()
			continue
		}

		if buf.Len() == 2 && buf.Bytes()[1] != start2 {
			buf.Reset()
			continue
		}

		if buf.Len() == 4 {
			pl = int(buf.Bytes()[2])*256 + int(buf.Bytes()[3])

			if pl > 512 {
				return nil, fmt.Errorf("invalid len")
			}

			continue
		}

		if pl != 0 && buf.Len() == pl+4 {
			msg := new(pb.FromRadio)

			err = proto.Unmarshal(buf.Bytes()[4:], msg)

			return msg, err
		}
	}

	return nil, ctx.Err()
}

func WriteMsg(w io.Writer, msg *pb.ToRadio) error {
	b, err := proto.Marshal(msg)

	if err != nil {
		return err
	}

	l := len(b)

	header := []byte{start1, start2, byte(l >> 8), byte(l)}

	if _, err1 := w.Write(header); err1 != nil {
		return err1
	}

	_, err = w.Write(b)

	return err
}

func ConfigMessage() *pb.ToRadio {
	return &pb.ToRadio{PayloadVariant: &pb.ToRadio_WantConfigId{WantConfigId: uint32(rand.Intn(65535))}}
}

// TextMessage - creates text message
// to = 0xffffffff - broadcast
func TextMessage(from uint32, to uint32, ch uint32, text string) *pb.ToRadio {
	return &pb.ToRadio{PayloadVariant: &pb.ToRadio_Packet{
		Packet: &pb.MeshPacket{
			From:    from,
			To:      to,
			Channel: ch,
			WantAck: true,
			Id:      rand.Uint32(),
			PayloadVariant: &pb.MeshPacket_Decoded{Decoded: &pb.Data{
				Portnum: pb.PortNum_TEXT_MESSAGE_APP,
				Payload: []byte(text),
			}},
		}},
	}
}
