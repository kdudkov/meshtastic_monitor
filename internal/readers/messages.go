package readers

import (
	"math/rand"

	pb "mesh/internal/meshtastic"
)

func ConfigMessage() *pb.ToRadio {
	return &pb.ToRadio{PayloadVariant: &pb.ToRadio_WantConfigId{WantConfigId: uint32(rand.Intn(65535))}}
}

func Heartbeat() *pb.ToRadio {
	return &pb.ToRadio{PayloadVariant: &pb.ToRadio_Heartbeat{Heartbeat: &pb.Heartbeat{Nonce: uint32(rand.Intn(65535))}}}
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
