package loadtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anand/api-testing-hw/internal/model"
)

const (
	// DefaultThink is the constant pause before every request.
	DefaultThink = 200 * time.Millisecond
	// DefaultHold is how long every profile stays at full concurrency.
	DefaultHold = 15 * time.Second
)

var labelOrder = []string{
	"POST /api/v1/auth/login",
	"GET /api/v1/auth/me",
	"POST /api/v1/posts",
	"GET /api/v1/posts/{id}",
	"PATCH /api/v1/posts/{id}",
}

// Profile is one comparable load level.
type Profile struct {
	Name   string
	Users  int
	RampUp time.Duration
	// Hold is the steady window after ramp-up, while every user is active.
	Hold time.Duration
	// ThreadDuration is how long each user keeps iterating after it starts.
	// DefaultProfiles sets it to RampUp+Hold so the first user is still
	// running when the last user has started.
	ThreadDuration time.Duration
}

// DefaultProfiles returns three runs: 10/25/50 users with ramp-up 10/25/50s.
func DefaultProfiles() []Profile {
	specs := []struct {
		users int
		ramp  time.Duration
	}{
		{10, 10 * time.Second},
		{25, 25 * time.Second},
		{50, 50 * time.Second},
	}
	out := make([]Profile, 0, len(specs))
	for _, spec := range specs {
		out = append(out, Profile{
			Name:           fmt.Sprintf("%d-users", spec.users),
			Users:          spec.users,
			RampUp:         spec.ramp,
			Hold:           DefaultHold,
			ThreadDuration: spec.ramp + DefaultHold,
		})
	}
	return out
}

// Sample is one measured HTTP call.
type Sample struct {
	Start        time.Time
	Elapsed      time.Duration
	Label        string
	Status       int
	Success      bool
	Failure      string
	Bytes        int
	SentBytes    int
	Thread       string
	URL          string
	ActiveUsers  int
	PlannedUsers int
}

// Summary is the comparable result of one profile.
type Summary struct {
	Name       string        `json:"name"`
	Users      int           `json:"users"`
	RampUpSec  float64       `json:"ramp_up_sec"`
	HoldSec    float64       `json:"hold_sec"`
	ThreadSec  float64       `json:"thread_duration_sec"`
	ThinkMs    float64       `json:"think_time_ms"`
	CeilingRPS float64       `json:"think_time_ceiling_rps"`
	Started    time.Time     `json:"started"`
	Finished   time.Time     `json:"finished"`
	WallSec    float64       `json:"wall_sec"`
	Overall    Metric        `json:"overall"`
	Steady     Metric        `json:"steady"`
	Labels     []LabelMetric `json:"labels_steady"`
}

// Result is a profile summary plus the raw samples used for JTL.
type Result struct {
	Summary Summary
	Samples []Sample
}

// RunOptions configures one load profile.
type RunOptions struct {
	BaseURL string
	Rows    []Row
	Profile Profile
	Think   time.Duration
	Quiet   bool
}

// Check sends one iteration and returns an error when any assertion fails.
func Check(baseURL string, row Row) error {
	client := newClient(2)
	defer closeClient(client)
	e := &engine{
		client: client,
		base:   strings.TrimRight(baseURL, "/"),
		think:  0,
		users:  1,
		active: &atomic.Int32{},
	}
	var samples []Sample
	e.runIteration(row, "smoke", func(s Sample) { samples = append(samples, s) })
	if len(samples) != len(labelOrder) {
		return fmt.Errorf("smoke: got %d samples, want %d", len(samples), len(labelOrder))
	}
	for _, s := range samples {
		if !s.Success {
			return fmt.Errorf("smoke: %s failed: %s", s.Label, s.Failure)
		}
	}
	return nil
}

// Run executes one closed-workload profile.
func Run(opt RunOptions) (Result, error) {
	if err := validate(opt); err != nil {
		return Result{}, err
	}
	profile := opt.Profile
	client := newClient(profile.Users * 2)
	defer closeClient(client)

	e := &engine{
		client: client,
		base:   strings.TrimRight(opt.BaseURL, "/"),
		think:  opt.Think,
		users:  profile.Users,
		active: &atomic.Int32{},
	}
	seq := &sequencer{}
	samples := make(chan Sample, 4096)
	var wg sync.WaitGroup
	started := time.Now()

	for i := 0; i < profile.Users; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			delay := time.Duration(int64(profile.RampUp) * int64(id) / int64(profile.Users))
			time.Sleep(delay)
			e.active.Add(1)
			defer e.active.Add(-1)

			deadline := time.Now().Add(profile.ThreadDuration)
			thread := fmt.Sprintf("vu-%02d", id+1)
			for time.Now().Before(deadline) {
				row := seq.next(opt.Rows)
				e.runIteration(row, thread, func(s Sample) { samples <- s })
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(samples)
	}()

	progressDone := make(chan struct{})
	if !opt.Quiet {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-progressDone:
					return
				case <-ticker.C:
					fmt.Printf("  ... %s: running, active users %d\n", profile.Name, e.active.Load())
				}
			}
		}()
	}

	var all []Sample
	collectDone := make(chan struct{})
	go func() {
		for s := range samples {
			all = append(all, s)
		}
		close(collectDone)
	}()

	<-done
	close(progressDone)
	<-collectDone
	finished := time.Now()

	steady := filterSteady(all, started, profile.RampUp, profile.Hold)
	thinkSec := opt.Think.Seconds()
	var ceiling float64
	if thinkSec > 0 {
		ceiling = float64(profile.Users) / thinkSec
	}

	return Result{
		Summary: Summary{
			Name:       profile.Name,
			Users:      profile.Users,
			RampUpSec:  profile.RampUp.Seconds(),
			HoldSec:    profile.Hold.Seconds(),
			ThreadSec:  profile.ThreadDuration.Seconds(),
			ThinkMs:    ms(opt.Think),
			CeilingRPS: ceiling,
			Started:    started,
			Finished:   finished,
			WallSec:    finished.Sub(started).Seconds(),
			Overall:    summarize(all, finished.Sub(started)),
			Steady:     summarize(steady, profile.Hold),
			Labels:     summarizeByLabel(steady, profile.Hold),
		},
		Samples: all,
	}, nil
}

func validate(opt RunOptions) error {
	if strings.TrimSpace(opt.BaseURL) == "" {
		return errors.New("base url is empty")
	}
	if len(opt.Rows) == 0 {
		return errors.New("no csv rows")
	}
	if opt.Profile.Users < 1 {
		return errors.New("users must be >= 1")
	}
	if opt.Profile.ThreadDuration <= 0 {
		return errors.New("thread duration must be > 0")
	}
	if opt.Profile.Hold <= 0 {
		return errors.New("hold must be > 0")
	}
	if opt.Think < 0 {
		return errors.New("think time must be >= 0")
	}
	return nil
}

func filterSteady(samples []Sample, started time.Time, ramp, hold time.Duration) []Sample {
	from := started.Add(ramp)
	to := from.Add(hold)
	out := make([]Sample, 0, len(samples)/2)
	for _, s := range samples {
		if !s.Start.Before(from) && s.Start.Before(to) {
			out = append(out, s)
		}
	}
	return out
}

type sequencer struct {
	n atomic.Uint64
}

func (s *sequencer) next(rows []Row) Row {
	i := s.n.Add(1) - 1
	return rows[int(i%uint64(len(rows)))]
}

type engine struct {
	client *http.Client
	base   string
	think  time.Duration
	users  int
	active *atomic.Int32
}

func (e *engine) pause() {
	if e.think > 0 {
		time.Sleep(e.think)
	}
}

func (e *engine) runIteration(row Row, thread string, emit func(Sample)) {
	e.pause()
	var token string
	login := e.execute("POST /api/v1/auth/login", http.MethodPost, e.base+"/api/v1/auth/login", "", model.LoginRequest{
		Username: row.Username,
		Password: row.Password,
	}, thread, func(status int, raw []byte) error {
		if status != http.StatusOK {
			return fmt.Errorf("status %d: %s", status, snippet(raw))
		}
		var resp model.LoginResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return fmt.Errorf("decode login: %w", err)
		}
		if resp.AccessToken == "" {
			return errors.New("empty access_token")
		}
		if !strings.EqualFold(resp.User.Username, row.Username) {
			return fmt.Errorf("username %q", resp.User.Username)
		}
		token = resp.AccessToken
		return nil
	})
	emit(login)
	if !login.Success {
		return
	}

	e.pause()
	me := e.execute("GET /api/v1/auth/me", http.MethodGet, e.base+"/api/v1/auth/me", token, nil, thread, func(status int, raw []byte) error {
		if status != http.StatusOK {
			return fmt.Errorf("status %d: %s", status, snippet(raw))
		}
		var user model.User
		if err := json.Unmarshal(raw, &user); err != nil {
			return fmt.Errorf("decode profile: %w", err)
		}
		if !strings.EqualFold(user.Username, row.Username) {
			return fmt.Errorf("username %q", user.Username)
		}
		if user.Email == "" {
			return errors.New("empty email")
		}
		return nil
	})
	emit(me)
	if !me.Success {
		return
	}

	e.pause()
	var postID string
	create := e.execute("POST /api/v1/posts", http.MethodPost, e.base+"/api/v1/posts", token, model.CreatePostRequest{
		Title:  row.Title,
		Body:   row.Body,
		UserID: row.UserID,
	}, thread, func(status int, raw []byte) error {
		if status != http.StatusCreated {
			return fmt.Errorf("status %d: %s", status, snippet(raw))
		}
		var post model.Post
		if err := json.Unmarshal(raw, &post); err != nil {
			return fmt.Errorf("decode post: %w", err)
		}
		if post.ID == "" {
			return errors.New("empty id")
		}
		if post.Title != row.Title {
			return fmt.Errorf("title %q", post.Title)
		}
		if post.UserID != row.UserID {
			return fmt.Errorf("user_id %d", post.UserID)
		}
		postID = post.ID
		return nil
	})
	emit(create)
	if !create.Success {
		return
	}

	e.pause()
	getURL := e.base + "/api/v1/posts/" + url.PathEscape(postID)
	get := e.execute("GET /api/v1/posts/{id}", http.MethodGet, getURL, token, nil, thread, func(status int, raw []byte) error {
		if status != http.StatusOK {
			return fmt.Errorf("status %d: %s", status, snippet(raw))
		}
		var post model.Post
		if err := json.Unmarshal(raw, &post); err != nil {
			return fmt.Errorf("decode post: %w", err)
		}
		if post.ID != postID {
			return fmt.Errorf("id %q", post.ID)
		}
		if post.Title != row.Title {
			return fmt.Errorf("title %q", post.Title)
		}
		return nil
	})
	emit(get)
	if !get.Success {
		return
	}

	e.pause()
	patched := row.PatchedTitle
	patch := e.execute("PATCH /api/v1/posts/{id}", http.MethodPatch, getURL, token, model.PatchPostRequest{
		Title: &patched,
	}, thread, func(status int, raw []byte) error {
		if status != http.StatusOK {
			return fmt.Errorf("status %d: %s", status, snippet(raw))
		}
		var post model.Post
		if err := json.Unmarshal(raw, &post); err != nil {
			return fmt.Errorf("decode post: %w", err)
		}
		if post.ID != postID {
			return fmt.Errorf("id %q", post.ID)
		}
		if post.Title != row.PatchedTitle {
			return fmt.Errorf("title %q", post.Title)
		}
		return nil
	})
	emit(patch)
}

func (e *engine) execute(label, method, rawURL, token string, body any, thread string, assert func(status int, raw []byte) error) Sample {
	sample := Sample{
		Label:        label,
		Thread:       thread,
		URL:          rawURL,
		ActiveUsers:  int(e.active.Load()),
		PlannedUsers: e.users,
		Success:      false,
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			sample.Start = time.Now()
			sample.Failure = fmt.Sprintf("marshal body: %v", err)
			return sample
		}
		sample.SentBytes = len(raw)
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		sample.Start = time.Now()
		sample.Failure = fmt.Sprintf("new request: %v", err)
		return sample
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := e.client.Do(req)
	if err != nil {
		sample.Start = start
		sample.Elapsed = time.Since(start)
		sample.Failure = err.Error()
		return sample
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	sample.Start = start
	sample.Elapsed = time.Since(start)
	sample.Status = resp.StatusCode
	sample.Bytes = len(raw)
	if err != nil {
		sample.Failure = fmt.Sprintf("read body: %v", err)
		return sample
	}
	if err := assert(resp.StatusCode, raw); err != nil {
		sample.Failure = err.Error()
		return sample
	}
	sample.Success = true
	sample.Failure = ""
	return sample
}

func newClient(conns int) *http.Client {
	if conns < 2 {
		conns = 2
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        conns,
		MaxIdleConnsPerHost: conns,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 2 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

func closeClient(client *http.Client) {
	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func snippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) > 180 {
		s = s[:180]
	}
	return s
}
