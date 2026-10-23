package dashboard

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Dragonzz27/netkeeper/internal/history"
)

//go:embed index.html
var page string

var view = template.Must(template.New("logs").Funcs(template.FuncMap{"pretty": history.Pretty}).Parse(page))

type chartPoint struct {
	X, Y  float64
	Entry history.Entry
}

type pageData struct {
	Entries []history.Entry
	Points  []chartPoint
	Line    string
	Maximum float64
}

func dataFor(entries []history.Entry) pageData {
	data := pageData{Entries: entries, Maximum: 1}
	for _, e := range entries {
		if e.ElapsedMin != nil && *e.ElapsedMin > data.Maximum {
			data.Maximum = *e.ElapsedMin
		}
	}
	var start, span float64
	if len(entries) > 0 {
		first, _ := history.ParseTime(entries[0].Timestamp)
		last, _ := history.ParseTime(entries[len(entries)-1].Timestamp)
		start, span = float64(first.Unix()), last.Sub(first).Seconds()
	}
	var line strings.Builder
	for _, e := range entries {
		if e.ElapsedMin == nil {
			continue
		}
		at, _ := history.ParseTime(e.Timestamp)
		x := 480.0
		if span > 0 {
			x = 60 + (float64(at.Unix())-start)/span*880
		}
		y := 290 - *e.ElapsedMin/data.Maximum*250
		data.Points = append(data.Points, chartPoint{X: x, Y: y, Entry: e})
		fmt.Fprintf(&line, "%.2f,%.2f ", x, y)
	}
	data.Line = line.String()
	return data
}

func Handler(logFile string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		entries, err := history.Read(logFile)
		if err != nil {
			http.Error(w, "无法读取登录日志，请检查日志文件格式和权限", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
		if err := view.Execute(w, dataFor(entries)); err != nil {
			return
		}
	})
	return mux
}

func Serve(ctx context.Context, addr, logFile string, out io.Writer) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: Handler(logFile), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdown); err != nil {
				server.Close()
			}
		case <-done:
		}
	}()
	fmt.Fprintf(out, "登录日志页面：http://%s （Ctrl+C 停止）\n", listener.Addr())
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
