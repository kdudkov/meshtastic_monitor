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

	"mesh/internal/atak"
	"mesh/internal/readers"
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
	log     *slog.Logger
	config  *AppConfig
	address string
	me      uint32
	nodes   sync.Map
	atak    *atak.GoatakClient
}

func NewApp(config *AppConfig) *App {
	app := &App{
		log:    slog.Default(),
		config: config,
		me:     0,
		nodes:  sync.Map{},
	}

	if app.config.String("atak.host") != "" {
		app.atak = atak.New(
			app.config.String("atak.host"),
			app.config.String("atak.user"),
			app.config.String("atak.password"),
			nil)
	}

	return app
}

func (app *App) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	if app.config.Bool("network") {
		app.address = app.config.String("host")
		if app.address == "" {
			app.address = GetIP()
		}

		if app.address != "" {
			fmt.Printf("found network node with ip %s\n", app.address)
		} else {
			fmt.Println("No IP Address Provided")

			return
		}
	}

	if app.atak != nil {
		go app.atak.Start(ctx)
	}

	go app.Reconnect(ctx)

	<-sigc
	slog.Info("shutting down")
}

func (app *App) Reconnect(ctx context.Context) {
	for ctx.Err() == nil {
		if app.config.Bool("network") {
			readers.NewTCP(net.JoinHostPort(app.address, "4403"), app.ProcessMessage).Start(ctx)
		} else {
			if port := app.getSerialPort(); port != "" {
				fmt.Printf("connecting to serial port %s\n", port)

				readers.NewSerial(port, app.ProcessMessage).Start(ctx)
			} else {
				app.log.Warn("no serial port found")
			}
		}

		time.Sleep(time.Second)
	}
}

func (app *App) getSerialPort() string {
	if port := app.config.String("serial_port"); port != "" {
		return port
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

	config := NewAppConfig()
	config.Load("config.yml", "config_local.yml")

	if *useNet {
		config.Set("network", "true")
	}

	if *host != "" {
		config.Set("host", *host)
	}

	if *port != "" {
		config.Set("serial_port", *port)
	}

	NewApp(config).Run()
}
