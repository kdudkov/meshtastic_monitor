package main

import (
	"cmp"
	"fmt"

	"github.com/fatih/color"
	"google.golang.org/protobuf/proto"

	pb "mesh/internal/meshtastic"
)

func (app *App) GetName(addr uint32) string {
	var name string

	if addr == app.me {
		return color.HiGreenString("%x (me)", app.me)
	}

	if addr == 0xffffffff {
		return color.HiBlackString("all")
	}

	col := color.New(colors[int(addr)%len(colors)])

	if v, ok := app.nodes.Load(addr); ok {
		if node, ok1 := v.(*pb.NodeInfo); ok1 {
			name = col.Sprintf("%x (%s)", addr, cmp.Or(node.GetUser().GetLongName(), node.String()))
		}
	}

	if name == "" {
		name = col.Sprintf("%x", addr)
	}

	return name
}

func (app *App) ProcessMessage(msg *pb.FromRadio) {
	switch p := msg.PayloadVariant.(type) {
	case *pb.FromRadio_MyInfo:
		app.me = p.MyInfo.GetMyNodeNum()
		app.log.Info(fmt.Sprintf("me: %x\n", app.me))
	case *pb.FromRadio_NodeInfo:
		if p.NodeInfo.GetNum() != app.me {
			app.nodes.Store(p.NodeInfo.GetNum(), p.NodeInfo)
		}
	case *pb.FromRadio_Packet:
		from := app.GetName(p.Packet.GetFrom())
		to := app.GetName(p.Packet.GetTo())
		ch := p.Packet.GetChannel()

		var hop uint32
		if p.Packet.GetHopStart() > 0 {
			hop = p.Packet.GetHopStart() - p.Packet.GetHopLimit() + 1
		}

		if d := p.Packet.GetDecoded(); d != nil {
			app.log.Debug(fmt.Sprintf("packet ch%d hop %d %s -> %s %s", ch, hop, from, to, d.GetPortnum()))

			prefix := fmt.Sprintf("ch%d hop %d %s %s -> %s", ch, hop, PortName(d.GetPortnum()), from, to)
			val := color.HiBlackString("%x", d.GetPayload())

			switch d.GetPortnum() {
			case pb.PortNum_TEXT_MESSAGE_APP:
				val = string(d.GetPayload())

			case pb.PortNum_TELEMETRY_APP:
				v := new(pb.Telemetry)
				if err := proto.Unmarshal(d.GetPayload(), v); err == nil {
					val = color.BlueString(v.String())
				}
			case pb.PortNum_NODEINFO_APP:
				v := new(pb.User)
				if err := proto.Unmarshal(d.GetPayload(), v); err == nil {
					val = color.CyanString(v.String())
				}
			case pb.PortNum_POSITION_APP:
				v := new(pb.Position)
				if err := proto.Unmarshal(d.GetPayload(), v); err == nil {
					val = color.HiGreenString(v.String())
				}
			case pb.PortNum_ATAK_PLUGIN:
				v := new(pb.TAKPacket)
				if err := proto.Unmarshal(d.GetPayload(), v); err == nil {
					val = color.HiRedString(v.String())
				}
			}

			fmt.Printf("%s: %s\n", prefix, val)
		} else {
			color.RedString("encoded packet ch%d hop %d %s -> %s\n", ch, hop, from, to)
		}
	}
}

func PortName(port pb.PortNum) string {
	n := int(port)

	return color.New(colors[n%len(colors)]).Sprintf("%s", port.String())
}
