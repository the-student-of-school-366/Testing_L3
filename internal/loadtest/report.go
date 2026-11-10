package loadtest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReportMeta is the static description written into the HTML report.
type ReportMeta struct {
	Target   string
	Note     string
	DataFile string
	Rows     int
}

// WriteArtifacts saves JTL files, summary.json and the HTML report.
func WriteArtifacts(dir string, results []Result, meta ReportMeta) error {
	if err := os.MkdirAll(filepath.Join(dir, "report"), 0o755); err != nil {
		return err
	}
	summaries := make([]Summary, 0, len(results))
	for _, result := range results {
		path := filepath.Join(dir, result.Summary.Name+".jtl")
		if err := writeJTL(path, result.Samples); err != nil {
			return err
		}
		summaries = append(summaries, result.Summary)
	}

	raw, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), raw, 0o644); err != nil {
		return err
	}

	html, err := renderHTML(summaries, meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report", "index.html"), html, 0o644)
}

func writeJTL(path string, samples []Sample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	header := []string{
		"timeStamp", "elapsed", "label", "responseCode", "responseMessage",
		"threadName", "dataType", "success", "failureMessage", "bytes",
		"sentBytes", "grpThreads", "allThreads", "URL", "Latency", "IdleTime", "Connect",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, s := range samples {
		code := strconv.Itoa(s.Status)
		message := "OK"
		success := "true"
		failure := ""
		if !s.Success {
			success = "false"
			message = "Assertion failed"
			failure = oneLine(s.Failure)
			if s.Status == 0 {
				code = "0"
				message = "Non HTTP response"
			}
		}
		elapsed := strconv.FormatInt(elapsedMS(s.Elapsed), 10)
		row := []string{
			strconv.FormatInt(s.Start.UnixMilli(), 10),
			elapsed,
			s.Label,
			code,
			message,
			s.Thread,
			"text",
			success,
			failure,
			strconv.Itoa(s.Bytes),
			strconv.Itoa(s.SentBytes),
			strconv.Itoa(s.ActiveUsers),
			strconv.Itoa(s.PlannedUsers),
			s.URL,
			elapsed,
			"0",
			"0",
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func elapsedMS(d time.Duration) int64 {
	ms := int64(math.Round(float64(d) / float64(time.Millisecond)))
	if ms < 0 {
		return 0
	}
	return ms
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// Conclude builds the written conclusion from steady-state metrics.
func Conclude(summaries []Summary) []string {
	if len(summaries) == 0 {
		return []string{"Запусков нет."}
	}
	first := summaries[0]
	last := summaries[len(summaries)-1]
	latency := fmt.Sprintf(
		"Среднее время ответа изменилось с %.3f до %.3f мс, медиана — с %.3f до %.3f мс, p95 — с %.3f до %.3f мс.",
		first.Steady.AverageMs, last.Steady.AverageMs,
		first.Steady.MedianMs, last.Steady.MedianMs,
		first.Steady.P95Ms, last.Steady.P95Ms,
	)
	if last.Steady.MedianMs == 0 && last.Steady.AverageMs < 2 && last.Steady.P95Ms < 5 {
		latency += " Медиана 0 мс означает, что не меньше половины ответов короче шага таймера и записаны как нулевые. Среднее и p95 остаются в субмиллисекундном диапазоне: это шум измерения на localhost, а не ускорение сервиса."
	}
	lines := []string{
		fmt.Sprintf(
			"В установившемся окне %s throughput вырос с %.1f до %.1f RPS — в %.2f раза при росте числа пользователей с %d до %d (в %.1f раза).",
			holdPhrase(first.HoldSec),
			first.Steady.Throughput,
			last.Steady.Throughput,
			safeDiv(last.Steady.Throughput, first.Steady.Throughput),
			first.Users,
			last.Users,
			safeDiv(float64(last.Users), float64(first.Users)),
		),
		latency,
	}

	maxErr := 0.0
	for _, s := range summaries {
		if s.Steady.ErrorRate > maxErr {
			maxErr = s.Steady.ErrorRate
		}
	}
	if maxErr < 0.01 {
		lines = append(lines, "Ошибок нет ни на одном уровне нагрузки: error rate в установившемся окне равен 0%.")
	} else {
		lines = append(lines, fmt.Sprintf("Максимальный error rate в установившемся окне — %.2f%%.", maxErr))
	}

	saturated := false
	for _, s := range summaries {
		if s.CeilingRPS > 0 && s.Steady.Throughput < s.CeilingRPS*0.6 {
			saturated = true
		}
	}
	latencyGrew := last.Steady.P95Ms > first.Steady.P95Ms+10 && last.Steady.P95Ms > first.Steady.P95Ms*2
	if !saturated && !latencyGrew && maxErr < 1 {
		perUser := 0.0
		if first.ThinkMs > 0 {
			perUser = 1000 / first.ThinkMs
		}
		lines = append(lines,
			fmt.Sprintf(
				"Момент деградации не наблюдался. Throughput остаётся близким к потолку think time (%.0f мс перед каждым запросом, около %.0f RPS на пользователя). Потолок по уровням: %s. p95 остаётся небольшим, доля ошибок не растёт.",
				first.ThinkMs,
				perUser,
				ceilingList(summaries),
			),
			fmt.Sprintf("Стабильным можно считать весь исследованный диапазон: %s виртуальных пользователей. До отказа сервис не доводился.", userList(summaries)),
		)
		return lines
	}

	lines = append(lines, "При росте нагрузки сервис перестал удерживать поток, заданный паузами пользователей: throughput отстал от потолка think time и/или выросли p95 либо error rate. Нижняя часть диапазона, где доля от потолка ещё высока и ошибок нет, выглядит стабильной; верхняя уже показывает насыщение.")
	return lines
}

func ceilingList(summaries []Summary) string {
	parts := make([]string, 0, len(summaries))
	for _, s := range summaries {
		parts = append(parts, fmt.Sprintf("%.0f RPS при %d пользователях", s.CeilingRPS, s.Users))
	}
	return strings.Join(parts, "; ")
}

func userList(summaries []Summary) string {
	parts := make([]string, 0, len(summaries))
	for _, s := range summaries {
		parts = append(parts, strconv.Itoa(s.Users))
	}
	return strings.Join(parts, ", ")
}

func holdPhrase(sec float64) string {
	if sec == 1 {
		return "1 с при полной конкуренции"
	}
	return fmt.Sprintf("%.0f с при полной конкуренции", sec)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func renderHTML(summaries []Summary, meta ReportMeta) ([]byte, error) {
	maxRPS := 0.0
	for _, s := range summaries {
		if s.Steady.Throughput > maxRPS {
			maxRPS = s.Steady.Throughput
		}
		if s.CeilingRPS > maxRPS {
			maxRPS = s.CeilingRPS
		}
	}
	data := struct {
		Generated  string
		Meta       ReportMeta
		Summaries  []Summary
		Conclusion []string
		MaxRPS     float64
		ThinkMs    float64
	}{
		Generated:  time.Now().Format("2006-01-02 15:04:05 MST"),
		Meta:       meta,
		Summaries:  summaries,
		Conclusion: Conclude(summaries),
		MaxRPS:     maxRPS,
		ThinkMs:    summaries[0].ThinkMs,
	}

	tpl, err := template.New("report").Funcs(template.FuncMap{
		"f1": func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) },
		"f2": func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) },
		"f3": func(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) },
		"f0": func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) },
		"bar": func(v, max float64) int {
			if max <= 0 {
				return 0
			}
			n := int(math.Round(v / max * 100))
			if n < 0 {
				return 0
			}
			if n > 100 {
				return 100
			}
			return n
		},
	}).Parse(reportTemplate)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

const reportTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <title>Нагрузочное тестирование REST API</title>
  <style>
    :root {
      --ink: #1c1915;
      --muted: #5e584f;
      --line: #e3d9cc;
      --paper: #f7f3ec;
      --card: #fffdf9;
      --accent: #0f6e56;
      --accent-soft: #d7efe6;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--paper);
      color: var(--ink);
      font: 16px/1.5 "Segoe UI", sans-serif;
    }
    main { max-width: 980px; margin: 0 auto; padding: 40px 24px 72px; }
    h1, h2 { font-family: Georgia, "Times New Roman", serif; font-weight: 500; letter-spacing: -0.02em; }
    h1 { font-size: 40px; line-height: 1.15; margin: 0 0 8px; }
    h2 { font-size: 26px; margin: 36px 0 12px; }
    p { margin: 0 0 12px; }
    .lede { color: var(--muted); max-width: 68ch; }
    .meta { color: var(--muted); font-size: 14px; margin-bottom: 28px; }
    table { width: 100%; border-collapse: collapse; background: var(--card); }
    th, td { padding: 10px 12px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; }
    th { font-size: 13px; letter-spacing: 0.02em; color: var(--muted); font-weight: 600; }
    td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
    .card { background: var(--card); border: 1px solid var(--line); padding: 16px 18px; margin: 12px 0 20px; }
    .bar-row { display: grid; grid-template-columns: 92px 1fr 72px; gap: 10px; align-items: center; margin: 8px 0; }
    .track { height: 10px; background: #efe8dc; }
    .fill { height: 10px; background: var(--accent); }
    .fill.ceiling { background: transparent; outline: 1px dashed var(--accent); height: 10px; }
    ol { margin: 8px 0 0; padding-left: 20px; }
    code { font-family: Consolas, monospace; font-size: 0.92em; }
  </style>
</head>
<body>
<main>
  <h1>Нагрузочное тестирование REST API</h1>
  <p class="lede">Закрытая нагрузка на локальный сервис этого репозитория. Виртуальные пользователи логинятся, читают профиль, создают пост, читают его и обновляют заголовок. Данные каждой итерации берутся из CSV, перед каждым запросом — постоянная пауза.</p>
  <p class="meta">Сформирован {{.Generated}} · цель: {{.Meta.Target}}</p>

  <div class="card">
    <p>{{.Meta.Note}}</p>
    <p>Параметризация: <code>{{.Meta.DataFile}}</code>, {{.Meta.Rows}} строк. В запросах используются <code>username</code>, <code>password</code>, <code>title</code>, <code>body</code>, <code>user_id</code>, <code>patched_title</code>. Пауза: {{f0 .ThinkMs}} мс перед каждым запросом (constant think time).</p>
  </div>

  <h2>Сценарий и проверки</h2>
  <ol>
    <li><code>POST /api/v1/auth/login</code> — HTTP 200, непустой <code>access_token</code>, username совпал с CSV.</li>
    <li><code>GET /api/v1/auth/me</code> — HTTP 200, Bearer-токен, username и email на месте.</li>
    <li><code>POST /api/v1/posts</code> — HTTP 201, <code>id</code> не пустой, title и user_id совпали с CSV.</li>
    <li><code>GET /api/v1/posts/{id}</code> — HTTP 200, тот же id и исходный title.</li>
    <li><code>PATCH /api/v1/posts/{id}</code> — HTTP 200, title равен <code>patched_title</code>.</li>
  </ol>

  <h2>Сравнение уровней нагрузки</h2>
  <p>Цифры ниже сняты в окне полной конкуренции: ramp-up уже закончился, первый пользователь ещё не остановился. Так три запуска сопоставимы, хотя ramp-up у них разный. Общий итог вместе с разгоном — в следующей таблице.</p>
  <table>
    <thead>
      <tr>
        <th>Нагрузка</th>
        <th class="num">Throughput / RPS</th>
        <th class="num">Average, мс</th>
        <th class="num">Median, мс</th>
        <th class="num">p95, мс</th>
        <th class="num">Error Rate</th>
      </tr>
    </thead>
    <tbody>
      {{range .Summaries}}
      <tr>
        <td>{{.Users}} users, ramp-up {{f0 .RampUpSec}} с</td>
        <td class="num">{{f1 .Steady.Throughput}}</td>
        <td class="num">{{f3 .Steady.AverageMs}}</td>
        <td class="num">{{f3 .Steady.MedianMs}}</td>
        <td class="num">{{f3 .Steady.P95Ms}}</td>
        <td class="num">{{f2 .Steady.ErrorRate}}%</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <h2>Throughput и потолок паузы</h2>
  <p>Сплошная полоса — измеренный RPS. Пунктир — потолок, если сервер отвечает мгновенно, а пользователь ждёт {{f0 .ThinkMs}} мс перед каждым запросом.</p>
  {{range .Summaries}}
  <div class="bar-row">
    <div>{{.Users}} users</div>
    <div class="track"><div class="fill" style="width: {{bar .Steady.Throughput $.MaxRPS}}%"></div></div>
    <div class="num">{{f1 .Steady.Throughput}}</div>
  </div>
  <div class="bar-row">
    <div></div>
    <div class="track"><div class="fill ceiling" style="width: {{bar .CeilingRPS $.MaxRPS}}%"></div></div>
    <div class="num">{{f0 .CeilingRPS}}</div>
  </div>
  {{end}}

  <h2>Весь запуск, включая ramp-up</h2>
  <table>
    <thead>
      <tr>
        <th>Нагрузка</th>
        <th class="num">Wall, с</th>
        <th class="num">Samples</th>
        <th class="num">RPS</th>
        <th class="num">Average, мс</th>
        <th class="num">p95, мс</th>
        <th class="num">Error Rate</th>
      </tr>
    </thead>
    <tbody>
      {{range .Summaries}}
      <tr>
        <td>{{.Users}} users</td>
        <td class="num">{{f1 .WallSec}}</td>
        <td class="num">{{.Overall.Samples}}</td>
        <td class="num">{{f1 .Overall.Throughput}}</td>
        <td class="num">{{f3 .Overall.AverageMs}}</td>
        <td class="num">{{f3 .Overall.P95Ms}}</td>
        <td class="num">{{f2 .Overall.ErrorRate}}%</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <h2>По запросам, установившееся окно</h2>
  <table>
    <thead>
      <tr>
        <th>Нагрузка</th>
        <th>Запрос</th>
        <th class="num">Samples</th>
        <th class="num">RPS</th>
        <th class="num">Average, мс</th>
        <th class="num">p95, мс</th>
        <th class="num">Errors</th>
      </tr>
    </thead>
    <tbody>
      {{range .Summaries}}
        {{$users := .Users}}
        {{range .Labels}}
        <tr>
          <td>{{$users}} users</td>
          <td><code>{{.Label}}</code></td>
          <td class="num">{{.Samples}}</td>
          <td class="num">{{f1 .Throughput}}</td>
          <td class="num">{{f3 .AverageMs}}</td>
          <td class="num">{{f3 .P95Ms}}</td>
          <td class="num">{{.Errors}}</td>
        </tr>
        {{end}}
      {{end}}
    </tbody>
  </table>

  <h2>Вывод</h2>
  {{range .Conclusion}}<p>{{.}}</p>{{end}}
</main>
</body>
</html>
`
