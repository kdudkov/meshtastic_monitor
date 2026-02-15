package main

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kdudkov/goatak/pkg/cot"
	"github.com/kdudkov/goatak/pkg/cotproto"
	"google.golang.org/protobuf/proto"

	pb "mesh/internal/meshtastic"
)

func (app *App) GetName(addr uint32, useColor bool) string {
	var name string
	var col *color.Color

	switch addr {
	case 0xffffffff:
		name = "all"
		col = color.New(color.FgHiBlack)
	case app.me:
		name = "me"
		col = color.New(color.FgHiGreen)
	default:
		col = color.New(colors[int(addr)%len(colors)])
		if node := app.Load(addr); node != nil {
			name = fmt.Sprintf("%s (%8x)",
				strings.TrimSpace(cmp.Or(node.GetUser().GetLongName(), node.GetUser().GetShortName())),
				addr)
		} else {
			name = fmt.Sprintf("%8x", addr)
		}
	}

	if useColor {
		name = col.Sprint(name)
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
		if p.Packet.GetFrom() == app.me && !app.config.Bool("show_my") {
			return
		}

		from := app.GetName(p.Packet.GetFrom(), true)
		to := app.GetName(p.Packet.GetTo(), true)
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
					if app.atak != nil {
						app.atak.SendMessage(convertPosition(
							p.Packet.GetFrom(),
							app.GetName(p.Packet.GetFrom(), false),
							v,
							fmt.Sprintf("hops %d", hop),
						))
					}
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

func convertPosition(id uint32, from string, p *pb.Position, text string) *cotproto.TakMessage {
	evt := &cotproto.CotEvent{
		Uid:       fmt.Sprintf("!%8x", id),
		Type:      "a-f-G",
		SendTime:  cot.TimeToMillis(time.Now()),
		StartTime: cot.TimeToMillis(time.Now()),
		StaleTime: cot.TimeToMillis(time.Now().Add(time.Hour)),
		Lat:       float64(p.GetLatitudeI()) * 1e-7,
		Lon:       float64(p.GetLongitudeI()) * 1e-7,
		Hae:       float64(p.GetAltitudeHae()),
		Ce:        float64(p.GetPDOP()) / 100,
		Le:        0,
		Detail: &cotproto.Detail{
			Contact: &cotproto.Contact{
				Callsign: from,
			},
		},
	}

	xd := cot.NewXMLDetails()
	if text != "" {
		xd.AddChild("remarks", nil, text)
	}

	evt.Detail.XmlDetail = xd.AsXMLString()

	return &cotproto.TakMessage{
		CotEvent: evt,
	}
}
