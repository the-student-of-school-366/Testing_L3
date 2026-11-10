package loadtest

import (
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anand/api-testing-hw/internal/app"
	"github.com/anand/api-testing-hw/internal/config"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestPercentileNearestRank(t *testing.T) {
	values := make([]time.Duration, 100)
	for i := range values {
		values[i] = time.Duration(100-i) * time.Millisecond
	}
	if got := Percentile(values, 0.95); got != 95*time.Millisecond {
		t.Fatalf("p95: got %s", got)
	}
	if got := Percentile(values, 0.50); got != 50*time.Millisecond {
		t.Fatalf("p50: got %s", got)
	}
	if got := Percentile(nil, 0.95); got != 0 {
		t.Fatalf("empty: got %s", got)
	}
}

func TestParseRows(t *testing.T) {
	rows, err := ParseRows(strings.NewReader("username,password,title,body,user_id,patched_title\nBret,Password@123,Hello,Body,7,Hello updated\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UserID != 7 || rows[0].PatchedTitle != "Hello updated" {
		t.Fatalf("rows: %+v", rows)
	}

	if _, err := ParseRows(strings.NewReader("title\nonly\n")); err == nil {
		t.Fatal("expected missing column error")
	}
}

func TestFixtureCSV(t *testing.T) {
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := LoadRows(filepath.Join(root, "loadtest", "data", "posts.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 10 {
		t.Fatalf("rows: %d", len(rows))
	}
}

func TestScenarioAndShortRun(t *testing.T) {
	application := app.New(config.Config{Addr: ":0"})
	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	row := Row{
		Username:     "Bret",
		Password:     "Password@123",
		Title:        "Load test post",
		Body:         "body",
		UserID:       4,
		PatchedTitle: "Load test post updated",
	}
	if err := Check(server.URL, row); err != nil {
		t.Fatal(err)
	}

	result, err := Run(RunOptions{
		BaseURL: server.URL,
		Rows:    []Row{row},
		Profile: Profile{
			Name:           "tiny",
			Users:          2,
			RampUp:         30 * time.Millisecond,
			Hold:           200 * time.Millisecond,
			ThreadDuration: 260 * time.Millisecond,
		},
		Think: 0,
		Quiet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Overall.Samples < len(labelOrder) {
		t.Fatalf("samples: %d", result.Summary.Overall.Samples)
	}
	if result.Summary.Overall.Errors != 0 {
		t.Fatalf("errors: %d", result.Summary.Overall.Errors)
	}
	if result.Summary.Steady.Samples == 0 {
		t.Fatal("expected samples in the steady window")
	}

	dir := t.TempDir()
	err = WriteArtifacts(dir, []Result{result}, ReportMeta{
		Target:   server.URL,
		Note:     "test",
		DataFile: "posts.csv",
		Rows:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConcludeNoDegradation(t *testing.T) {
	lines := Conclude([]Summary{
		{Users: 10, HoldSec: 15, ThinkMs: 200, CeilingRPS: 50, Steady: Metric{Throughput: 48, AverageMs: 0.4, MedianMs: 0.3, P95Ms: 0.8}},
		{Users: 50, HoldSec: 15, ThinkMs: 200, CeilingRPS: 250, Steady: Metric{Throughput: 240, AverageMs: 0.5, MedianMs: 0.4, P95Ms: 1.2}},
	})
	text := strings.Join(lines, " ")
	if !strings.Contains(text, "деградации не наблюдался") {
		t.Fatalf("conclusion: %s", text)
	}
}
