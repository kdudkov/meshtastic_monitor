package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"go.bug.st/serial"
)

var colors = []color.Attribute{
	color.FgRed,
	color.FgGreen,
	color.FgYellow,
	color.FgBlue,
	color.FgMagenta,
	color.FgCyan,
	color.FgWhite,
	color.FgHiRed,
	color.FgHiGreen,
	color.FgHiYellow,
	color.FgHiBlue,
	color.FgHiMagenta,
	color.FgHiCyan,
	color.FgHiWhite,
}

type App struct {
	log        *slog.Logger
	me         uint32
	nodes      sync.Map
	network    bool
	address    string
	serialPort string
}

func NewApp(network bool, address string, port string) *App {
	return &App{
		log:        slog.Default(),
		network:    network,
		address:    address,
		serialPort: port,
		me:         0,
		nodes:      sync.Map{},
	}
}

func (app *App) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	if app.network {
		if app.address == "" {
			app.address = GetIP()

			fmt.Printf("found network node with ip %s\n", app.address)

			if app.address == "" {
				fmt.Println("No IP Address Provided")

				return
			}
		}
	}

	go app.Reconnect(ctx)

	<-sigc
	slog.Info("shutting down")
}

func (app *App) Reconnect(ctx context.Context) {
	for ctx.Err() == nil {
		if app.network {
			NewTCP(net.JoinHostPort(app.address, "4403"), app.ProcessMessage).Start(ctx)
		} else {
			if port := app.getSerialPort(); port != "" {
				fmt.Printf("connecting to serial port %s\n", port)
				
				NewSerial(port, app.ProcessMessage).Start(ctx)
			} else {
				app.log.Warn("no serial port found")
			}
		}

		time.Sleep(time.Second)
	}
}

func (app *App) getSerialPort() string {
	if app.serialPort != "" {
		return app.serialPort
	}
	ports, _ := serial.GetPortsList()

	for _, s := range ports {
		slog.Debug("found port " + s)
		if strings.Contains(s, ".usbserial") || strings.Contains(s, ".usbmodem") {
			slog.Info("found serial port " + s)
			return s
		}
	}

	return ""
}

func GetIP() string {
	ip, err := net.LookupIP("meshtastic.local")
	if err != nil {
		return ""
	}

	for _, i := range ip {
		if i.To4() != nil {
			return i.String()
		}
	}

	return ""
}

func main() {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(h))

	useNet := flag.Bool("net", false, "use network")
	host := flag.String("host", "", "host or ip")
	port := flag.String("port", "", "serial port")

	flag.Parse()

	NewApp(*useNet, *host, *port).Run()
}
