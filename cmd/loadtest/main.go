package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anand/api-testing-hw/internal/app"
	"github.com/anand/api-testing-hw/internal/config"
	"github.com/anand/api-testing-hw/internal/loadtest"
)

func main() {
	baseURL := flag.String("base-url", "", "already running API; empty starts the project server in-process")
	flag.Parse()

	root, err := loadtest.ModuleRoot()
	if err != nil {
		log.Fatal(err)
	}
	dataPath := filepath.Join(root, "loadtest", "data", "posts.csv")
	outDir := filepath.Join(root, "loadtest", "results")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	rows, err := loadtest.LoadRows(dataPath)
	if err != nil {
		log.Fatal(err)
	}

	target := *baseURL
	note := "Запросы отправлены на уже запущенный сервер " + target + "."
	var shutdown func()
	stop := func() {
		if shutdown != nil {
			shutdown()
			shutdown = nil
		}
	}
	defer stop()
	if target == "" {
		target, shutdown, err = startServer(filepath.Join(outDir, "server.log"))
		if err != nil {
			log.Fatal(err)
		}
		note = "Поднят in-process сервер с тем же router, что и cmd/server, включая middleware логирования. Журнал сервера: loadtest/results/server.log."
	}

	fmt.Printf("target: %s\n", target)
	fmt.Printf("data:   %s (%d rows)\n", dataPath, len(rows))
	fmt.Printf("think:  %s before each request\n", loadtest.DefaultThink)
	fmt.Println("smoke:  one iteration")
	if err := loadtest.Check(target, rows[0]); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		stop()
		os.Exit(1)
	}
	fmt.Println("smoke:  ok")

	profiles := loadtest.DefaultProfiles()
	results := make([]loadtest.Result, 0, len(profiles))
	for i, profile := range profiles {
		fmt.Printf("\nrun %d/%d: %d users, ramp-up %s, each user %s, steady window %s\n",
			i+1, len(profiles), profile.Users, profile.RampUp, profile.ThreadDuration, profile.Hold)
		started := time.Now()
		result, err := loadtest.Run(loadtest.RunOptions{
			BaseURL: target,
			Rows:    rows,
			Profile: profile,
			Think:   loadtest.DefaultThink,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "run %s: %v\n", profile.Name, err)
			stop()
			os.Exit(1)
		}
		results = append(results, result)
		s := result.Summary
		fmt.Printf("done %s in %.1fs\n", profile.Name, time.Since(started).Seconds())
		fmt.Printf("  steady: samples=%d rps=%.1f avg=%.3fms median=%.3fms p95=%.3fms errors=%.2f%% ceiling=%.0f\n",
			s.Steady.Samples, s.Steady.Throughput, s.Steady.AverageMs, s.Steady.MedianMs, s.Steady.P95Ms, s.Steady.ErrorRate, s.CeilingRPS)
		fmt.Printf("  overall: samples=%d rps=%.1f avg=%.3fms p95=%.3fms errors=%.2f%%\n",
			s.Overall.Samples, s.Overall.Throughput, s.Overall.AverageMs, s.Overall.P95Ms, s.Overall.ErrorRate)
	}

	if err := loadtest.WriteArtifacts(outDir, results, loadtest.ReportMeta{
		Target:   target,
		Note:     note,
		DataFile: "loadtest/data/posts.csv",
		Rows:     len(rows),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write results: %v\n", err)
		stop()
		os.Exit(1)
	}

	fmt.Println("\nВывод:")
	for _, line := range loadtest.Conclude(summariesOf(results)) {
		fmt.Printf("- %s\n", line)
	}
	fmt.Printf("\nreport: %s\n", filepath.Join(outDir, "report", "index.html"))
}

func summariesOf(results []loadtest.Result) []loadtest.Summary {
	out := make([]loadtest.Summary, len(results))
	for i, result := range results {
		out[i] = result.Summary
	}
	return out
}

func startServer(logPath string) (string, func(), error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", nil, err
	}
	buffered := bufio.NewWriterSize(logFile, 256*1024)
	log.SetOutput(buffered)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	application := app.New(config.Config{Addr: ln.Addr().String()})
	server := &http.Server{
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server: %v\n", err)
		}
	}()

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = buffered.Flush()
		_ = logFile.Close()
	}
	return "http://" + ln.Addr().String(), shutdown, nil
}
