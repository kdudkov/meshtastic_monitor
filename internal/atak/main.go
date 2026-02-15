package atak

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/google/uuid"
	"github.com/kdudkov/goatak/pkg/cot"
	"github.com/kdudkov/goatak/pkg/cotproto"
)

type GoatakClient struct {
	uid            string
	token          string
	logger         *slog.Logger
	host           string
	status         uint32
	ch             chan *cotproto.TakMessage
	cb             func(m *cotproto.TakMessage)
	client         *http.Client
	reconnectDelay time.Duration
	cancel         context.CancelFunc
}

func New(host, token string, cb func(m *cotproto.TakMessage)) *GoatakClient {
	return &GoatakClient{
		uid:            uuid.NewString(),
		logger:         slog.With(slog.String("logger", "atak")),
		host:           host,
		token:          token,
		reconnectDelay: time.Second,
		cb:             cb,
		status:         0,
		ch:             make(chan *cotproto.TakMessage, 128),
		client:         &http.Client{Timeout: time.Second * 5},
	}
}

func (g *GoatakClient) GetUID() string {
	return g.uid
}

func (g *GoatakClient) GetName() string {
	return "ATAK"
}

func (g *GoatakClient) Start(ctx context.Context) error {
	g.connect(ctx)

	return nil
}

func (g *GoatakClient) Stop() {
	// no-op
}

func (g *GoatakClient) Connected() bool {
	return atomic.LoadUint32(&g.status) == 1
}

func (g *GoatakClient) setConnected(connected bool) {
	if connected {
		atomic.StoreUint32(&g.status, 1)
	} else {
		atomic.StoreUint32(&g.status, 0)
	}
}

func (g *GoatakClient) SendMessage(msg *cotproto.TakMessage) {
	if !g.Connected() {
		return
	}

	select {
	case g.ch <- msg:
	default:
	}
}

func (g *GoatakClient) connect(ctx context.Context) {
	for ctx.Err() == nil {
		h := http.Header{"Authorization": []string{"Bearer " + g.token}}

		conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("wss://%s/takproto/1", g.host), h)

		if err != nil {
			g.logger.Error("error on ws dial", slog.Any("error", err))
			sleep(ctx, g.reconnectDelay)

			continue
		}

		conn.SetCloseHandler(g.closeHandler)

		g.setConnected(true)
		g.logger.Info("connected")

		ctx1, cancel := context.WithCancel(ctx)

		g.cancel = cancel

		wg := new(sync.WaitGroup)
		wg.Add(2)

		go g.reader(ctx1, conn, wg)
		go g.writer(ctx1, conn, wg)
		go g.pinger(ctx1)

		wg.Wait()
	}
}

func (g *GoatakClient) closeHandler(code int, text string) error {
	g.logger.Info(fmt.Sprintf("closed: code %d, text %s", code, text))
	g.setConnected(false)

	return nil
}

func (g *GoatakClient) stop(conn *websocket.Conn) {
	conn.Close()
	g.setConnected(false)

	if g.cancel != nil {
		g.cancel()
	}
}

func (g *GoatakClient) reader(ctx context.Context, conn *websocket.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	g.logger.Debug("reader started")
	defer g.logger.Debug("reader stopped")

	for ctx.Err() == nil {
		mt, b, err := conn.ReadMessage()

		if err != nil {
			g.logger.Error("read error", slog.Any("error", err))

			break
		}

		if mt != websocket.BinaryMessage {
			continue
		}

		if err := g.parse(b); err != nil {
			g.logger.Error("parse error", slog.Any("error", err))
		}
	}

	g.stop(conn)
}

func (g *GoatakClient) writer(ctx context.Context, conn *websocket.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	g.logger.Debug("writer started")
	defer g.logger.Debug("writer stopped")

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			g.stop(conn)

			return
		case m := <-g.ch:
			dat, err := cot.MakeProtoPacket(m)
			if err != nil {
				g.logger.Error("make proto error", slog.Any("error", err))
				continue
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, dat); err != nil {
				g.logger.Error("write error", slog.Any("error", err))
				g.stop(conn)

				return
			}
		}
	}
}

func (g *GoatakClient) pinger(ctx context.Context) {
	g.logger.Debug("pinger started")
	defer g.logger.Debug("pinger stopped")

	t := time.NewTicker(time.Second * 30)
	defer t.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			select {
			case g.ch <- cot.MakePing(g.uid):
			default:
			}
		}
	}
}

func (g *GoatakClient) parse(b []byte) error {
	msg, err := cot.ReadProto(bufio.NewReader(bytes.NewReader(b)))
	if err != nil {
		return fmt.Errorf("read error %w", err)
	}

	cotmsg, err := cot.CotFromProto(msg, "TAK", "")
	if err != nil {
		return fmt.Errorf("convert error %w", err)
	}

	if cotmsg.IsControl() || cotmsg.IsPing() {
		return nil
	}

	if g.cb != nil {
		g.cb(msg)
	}

	return nil
}

func sleep(ctx context.Context, d time.Duration) bool {
	if ctx.Err() != nil {
		return true
	}

	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}
