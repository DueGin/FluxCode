package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type apiEnvelope struct {
	Code     int             `json:"code"`
	Message  string          `json:"message"`
	Reason   string          `json:"reason,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type publicSettings struct {
	RegistrationEnabled bool `json:"registration_enabled"`
	EmailVerifyEnabled  bool `json:"email_verify_enabled"`
	TurnstileEnabled    bool `json:"turnstile_enabled"`
}

type sendVerifyCodeData struct {
	Message        string `json:"message"`
	Countdown      int    `json:"countdown"`
	TaskID         string `json:"task_id"`
	QueueMessageID string `json:"queue_message_id"`
	SendAck        bool   `json:"send_ack"`
	ConsumeAck     bool   `json:"consume_ack"`
	DeliveryStatus string `json:"delivery_status"`
}

type verifyCodeTaskStatus struct {
	TaskID         string `json:"task_id"`
	TaskType       string `json:"task_type"`
	Email          string `json:"email"`
	SiteName       string `json:"site_name"`
	Status         string `json:"status"`
	QueueMessageID string `json:"queue_message_id,omitempty"`
	Attempt        int    `json:"attempt"`
	NextAttempt    int    `json:"next_attempt,omitempty"`
	MaxAttempts    int    `json:"max_attempts"`
	SendAckAt      int64  `json:"send_ack_at"`
	ConsumeAckAt   int64  `json:"consume_ack_at,omitempty"`
	NextRetryAt    int64  `json:"next_retry_at,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	Worker         string `json:"worker,omitempty"`
	UpdatedAt      int64  `json:"updated_at"`
}

type registerResponse struct {
	AccessToken string `json:"access_token"`
}

type verificationCodeCacheValue struct {
	Code      string    `json:"Code"`
	Attempts  int       `json:"Attempts"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type client struct {
	baseURL        string
	httpClient     *http.Client
	turnstileToken string
}

func (c *client) getPublicSettings(ctx context.Context) (*publicSettings, error) {
	endpoint := strings.TrimRight(c.baseURL, "/") + "/settings/public"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("api error: code=%d message=%q reason=%q", env.Code, env.Message, env.Reason)
	}

	var settings publicSettings
	if err := json.Unmarshal(env.Data, &settings); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}
	return &settings, nil
}

func (c *client) sendVerifyCode(ctx context.Context, email string) (*sendVerifyCodeData, time.Duration, int, string, error) {
	payload := map[string]any{
		"email": email,
	}
	if c.turnstileToken != "" {
		payload["turnstile_token"] = c.turnstileToken
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, 0, "", err
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/auth/send-verify-code"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	rawStr := strings.TrimSpace(string(raw))
	if resp.StatusCode != http.StatusOK {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("http %d", resp.StatusCode)
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Code != 0 {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("api error: code=%d message=%q reason=%q", env.Code, env.Message, env.Reason)
	}

	var data sendVerifyCodeData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("decode data: %w", err)
	}
	return &data, latency, resp.StatusCode, rawStr, nil
}

func (c *client) getVerifyCodeStatus(ctx context.Context, taskID string) (*verifyCodeTaskStatus, time.Duration, int, string, error) {
	u, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/auth/verify-code-status")
	if err != nil {
		return nil, 0, 0, "", err
	}
	q := u.Query()
	q.Set("task_id", taskID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, 0, "", err
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	rawStr := strings.TrimSpace(string(raw))
	if resp.StatusCode != http.StatusOK {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("http %d", resp.StatusCode)
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Code != 0 {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("api error: code=%d message=%q reason=%q", env.Code, env.Message, env.Reason)
	}

	var status verifyCodeTaskStatus
	if err := json.Unmarshal(env.Data, &status); err != nil {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("decode data: %w", err)
	}
	return &status, latency, resp.StatusCode, rawStr, nil
}

func (c *client) register(ctx context.Context, email, password, verifyCode string) (*registerResponse, time.Duration, int, string, error) {
	payload := map[string]any{
		"email":       email,
		"password":    password,
		"verify_code": verifyCode,
	}
	// 注意：当 verify_code 非空时，后端会跳过 Turnstile 校验
	if verifyCode == "" && c.turnstileToken != "" {
		payload["turnstile_token"] = c.turnstileToken
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, 0, "", err
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/auth/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	rawStr := strings.TrimSpace(string(raw))
	if resp.StatusCode != http.StatusOK {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("http %d", resp.StatusCode)
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Code != 0 {
		return nil, latency, resp.StatusCode, rawStr, fmt.Errorf("api error: code=%d message=%q reason=%q", env.Code, env.Message, env.Reason)
	}

	var data registerResponse
	if err := json.Unmarshal(env.Data, &data); err != nil {
		// 不强依赖 token 内容，只要 code==0 即认为注册成功
		return &registerResponse{}, latency, resp.StatusCode, rawStr, nil
	}
	return &data, latency, resp.StatusCode, rawStr, nil
}

type sendResult struct {
	Email      string
	OK         bool
	Latency    time.Duration
	HTTPStatus int
	Err        string

	TaskID     string
	MsgID      string
	Countdown  int
	SendAck    bool
	ConsumeAck bool
	Status     string
}

type consumeResult struct {
	TaskID        string
	Email         string
	FinalStatus   string
	LastError     string
	TimeToFinal   time.Duration
	Polls         int
	HTTPStatus    int
	HTTPErrorBody string
}

type registerResult struct {
	Email      string
	OK         bool
	Latency    time.Duration
	HTTPStatus int
	Err        string
}

func main() {
	var (
		baseURL       = flag.String("base-url", "http://localhost:8080/api/v1", "API base url, e.g. https://your.domain/api/v1")
		mode          = flag.String("mode", "register", "send-code | register (auto adapts to email_verify_enabled)")
		domain        = flag.String("domain", "duegin.online", "email domain, e.g. duegin.online")
		emailPrefix   = flag.String("email-prefix", "loadtest", "email local-part prefix")
		total         = flag.Int("total", 200, "total users")
		concurrency   = flag.Int("concurrency", 20, "concurrent workers")
		password      = flag.String("password", "Test123456!", "password for all users (min length 6)")
		httpTimeout   = flag.Duration("http-timeout", 15*time.Second, "single http request timeout")
		turnstile     = flag.String("turnstile-token", "", "turnstile token (required if turnstile_enabled and calling /send-verify-code)")
		checkConsume  = flag.Bool("check-consume", true, "poll /auth/verify-code-status for consume ack")
		consumeWait   = flag.Duration("consume-timeout", 2*time.Minute, "max time to wait for consume ack per task")
		pollInterval  = flag.Duration("poll-interval", 1*time.Second, "poll interval for task status")
		printErrors   = flag.Int("print-errors", 10, "print first N errors")
		redisAddr     = flag.String("redis-addr", "", "redis address for fetching verify code, e.g. 127.0.0.1:6379 (optional)")
		redisPassword = flag.String("redis-password", "", "redis password (optional)")
		redisDB       = flag.Int("redis-db", 0, "redis db (optional)")
	)
	flag.Parse()

	if *total <= 0 || *concurrency <= 0 {
		fatalf("total/concurrency must be positive")
	}
	if strings.TrimSpace(*domain) == "" {
		fatalf("domain is required")
	}
	if strings.TrimSpace(*baseURL) == "" {
		fatalf("base-url is required")
	}

	httpClient := &http.Client{
		Timeout: *httpTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        2048,
			MaxIdleConnsPerHost: 2048,
			IdleConnTimeout:     60 * time.Second,
		},
	}
	c := &client{
		baseURL:        strings.TrimRight(*baseURL, "/"),
		httpClient:     httpClient,
		turnstileToken: strings.TrimSpace(*turnstile),
	}

	ctx := context.Background()
	settings, err := c.getPublicSettings(ctx)
	if err != nil {
		fatalf("get public settings failed: %v", err)
	}

	fmt.Printf("BaseURL=%s total=%d concurrency=%d mode=%s\n", c.baseURL, *total, *concurrency, *mode)
	fmt.Printf("Settings: registration_enabled=%v email_verify_enabled=%v turnstile_enabled=%v\n",
		settings.RegistrationEnabled, settings.EmailVerifyEnabled, settings.TurnstileEnabled)

	if !settings.RegistrationEnabled {
		fatalf("registration is disabled by server settings")
	}

	if settings.TurnstileEnabled && c.turnstileToken == "" && (*mode == "send-code" || settings.EmailVerifyEnabled) {
		fatalf("turnstile_enabled=true but -turnstile-token is empty; cannot call /auth/send-verify-code")
	}

	var rdb *redis.Client
	if strings.TrimSpace(*redisAddr) != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     strings.TrimSpace(*redisAddr),
			Password: *redisPassword,
			DB:       *redisDB,
		})
		if err := rdb.Ping(ctx).Err(); err != nil {
			fatalf("redis ping failed: %v", err)
		}
	}

	emails := make([]string, *total)
	for i := 0; i < *total; i++ {
		emails[i] = fmt.Sprintf("%s_%s_%d@%s", *emailPrefix, randomString(10), i, *domain)
	}

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "send-code":
		runSendCodeOnly(ctx, c, emails, *concurrency, *checkConsume, *consumeWait, *pollInterval, *printErrors)
	case "register":
		if settings.EmailVerifyEnabled {
			if rdb == nil {
				fatalf("email_verify_enabled=true; need -redis-addr to fetch verify_code for registration")
			}
			runRegisterWithEmailVerify(ctx, c, rdb, emails, *password, *concurrency, *consumeWait, *pollInterval, *printErrors)
		} else {
			runRegisterDirect(ctx, c, emails, *password, *concurrency, *printErrors)
		}
	default:
		fatalf("unknown mode: %s", *mode)
	}
}

func runSendCodeOnly(
	ctx context.Context,
	c *client,
	emails []string,
	concurrency int,
	checkConsume bool,
	consumeTimeout time.Duration,
	pollInterval time.Duration,
	printErrors int,
) {
	fmt.Printf("\n== Stage 1: send verify code ==\n")
	sendResults, elapsed := stageSendVerifyCode(ctx, c, emails, concurrency)
	printSendSummary(sendResults, elapsed, printErrors)

	if !checkConsume {
		return
	}

	fmt.Printf("\n== Stage 2: poll consume ack ==\n")
	consumeResults, elapsed2 := stagePollConsumeAck(ctx, c, sendResults, concurrency, consumeTimeout, pollInterval)
	printConsumeSummary(consumeResults, elapsed2, printErrors)
}

func runRegisterDirect(ctx context.Context, c *client, emails []string, password string, concurrency int, printErrors int) {
	fmt.Printf("\n== Stage 1: register (no email verify) ==\n")
	results, elapsed := stageRegister(ctx, c, emails, password, "", concurrency)
	printRegisterSummary(results, elapsed, printErrors)
}

func runRegisterWithEmailVerify(
	ctx context.Context,
	c *client,
	rdb *redis.Client,
	emails []string,
	password string,
	concurrency int,
	consumeTimeout time.Duration,
	pollInterval time.Duration,
	printErrors int,
) {
	fmt.Printf("\n== Stage 1: send verify code ==\n")
	sendResults, elapsed := stageSendVerifyCode(ctx, c, emails, concurrency)
	printSendSummary(sendResults, elapsed, printErrors)

	fmt.Printf("\n== Stage 2: wait consume ack ==\n")
	consumeResults, elapsed2 := stagePollConsumeAck(ctx, c, sendResults, concurrency, consumeTimeout, pollInterval)
	printConsumeSummary(consumeResults, elapsed2, printErrors)

	fmt.Printf("\n== Stage 3: register with verify_code (from redis) ==\n")
	verifyCodes := make(map[string]string, len(emails))
	for _, r := range consumeResults {
		if r.FinalStatus != "sent" {
			continue
		}
		code, err := fetchVerifyCodeFromRedis(ctx, rdb, r.Email)
		if err != nil || code == "" {
			continue
		}
		verifyCodes[r.Email] = code
	}

	results, elapsed3 := stageRegisterWithCode(ctx, c, emails, password, verifyCodes, concurrency)
	printRegisterSummary(results, elapsed3, printErrors)
}

func stageSendVerifyCode(ctx context.Context, c *client, emails []string, concurrency int) ([]sendResult, time.Duration) {
	start := time.Now()
	results := make([]sendResult, len(emails))

	var okCount int64
	taskCh := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range taskCh {
			email := emails[idx]
			r := sendResult{Email: email}

			data, latency, httpStatus, rawBody, err := c.sendVerifyCode(ctx, email)
			r.Latency = latency
			r.HTTPStatus = httpStatus
			if err != nil {
				r.OK = false
				r.Err = fmt.Sprintf("%v; body=%s", err, compactBody(rawBody))
			} else if data == nil {
				r.OK = false
				r.Err = "nil response"
			} else {
				r.OK = true
				r.TaskID = data.TaskID
				r.MsgID = data.QueueMessageID
				r.Countdown = data.Countdown
				r.SendAck = data.SendAck
				r.ConsumeAck = data.ConsumeAck
				r.Status = data.DeliveryStatus
				atomic.AddInt64(&okCount, 1)
			}
			results[idx] = r
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	for i := range emails {
		taskCh <- i
	}
	close(taskCh)
	wg.Wait()

	_ = okCount
	return results, time.Since(start)
}

func stagePollConsumeAck(
	ctx context.Context,
	c *client,
	sendResults []sendResult,
	concurrency int,
	consumeTimeout time.Duration,
	pollInterval time.Duration,
) ([]consumeResult, time.Duration) {
	start := time.Now()

	type pollTask struct {
		TaskID string
		Email  string
	}
	tasks := make([]pollTask, 0, len(sendResults))
	for _, r := range sendResults {
		if !r.OK || r.TaskID == "" {
			continue
		}
		tasks = append(tasks, pollTask{TaskID: r.TaskID, Email: r.Email})
	}

	results := make([]consumeResult, len(tasks))
	taskCh := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range taskCh {
			task := tasks[idx]
			res := consumeResult{TaskID: task.TaskID, Email: task.Email}

			deadline := time.Now().Add(consumeTimeout)
			polls := 0
			for time.Now().Before(deadline) {
				polls++
				status, _, httpStatus, rawBody, err := c.getVerifyCodeStatus(ctx, task.TaskID)
				if err != nil {
					res.HTTPStatus = httpStatus
					res.HTTPErrorBody = compactBody(rawBody)
					time.Sleep(pollInterval)
					continue
				}

				if status == nil || status.Status == "" {
					time.Sleep(pollInterval)
					continue
				}
				res.FinalStatus = status.Status
				res.LastError = status.LastError
				res.Polls = polls

				if status.Status == "sent" || status.Status == "failed" || status.Status == "dropped" {
					res.TimeToFinal = time.Since(deadline.Add(-consumeTimeout))
					break
				}
				time.Sleep(pollInterval)
			}

			if res.FinalStatus == "" {
				res.FinalStatus = "timeout"
				res.Polls = polls
				res.TimeToFinal = consumeTimeout
			}

			results[idx] = res
		}
	}

	if concurrency > 50 {
		concurrency = 50 // 避免轮询压垮API
	}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	for i := range tasks {
		taskCh <- i
	}
	close(taskCh)
	wg.Wait()

	return results, time.Since(start)
}

func stageRegisterWithCode(
	ctx context.Context,
	c *client,
	emails []string,
	password string,
	verifyCodes map[string]string,
	concurrency int,
) ([]registerResult, time.Duration) {
	codes := make([]string, len(emails))
	for i, email := range emails {
		codes[i] = verifyCodes[email]
	}
	return stageRegister(ctx, c, emails, password, codes, concurrency)
}

func stageRegister(
	ctx context.Context,
	c *client,
	emails []string,
	password string,
	verifyCodes any,
	concurrency int,
) ([]registerResult, time.Duration) {
	start := time.Now()
	results := make([]registerResult, len(emails))

	taskCh := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for idx := range taskCh {
			email := emails[idx]

			verifyCode := ""
			switch v := verifyCodes.(type) {
			case string:
				verifyCode = v
			case []string:
				verifyCode = v[idx]
			}

			r := registerResult{Email: email}
			_, latency, httpStatus, rawBody, err := c.register(ctx, email, password, verifyCode)
			r.Latency = latency
			r.HTTPStatus = httpStatus
			if err != nil {
				r.OK = false
				r.Err = fmt.Sprintf("%v; body=%s", err, compactBody(rawBody))
			} else {
				r.OK = true
			}
			results[idx] = r
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	for i := range emails {
		taskCh <- i
	}
	close(taskCh)
	wg.Wait()

	return results, time.Since(start)
}

func fetchVerifyCodeFromRedis(ctx context.Context, rdb *redis.Client, email string) (string, error) {
	key := "verify_code:" + email
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	var v verificationCodeCacheValue
	if err := json.Unmarshal([]byte(val), &v); err != nil {
		return "", err
	}
	return strings.TrimSpace(v.Code), nil
}

func printSendSummary(results []sendResult, elapsed time.Duration, printErrors int) {
	var ok int
	var latencies []time.Duration
	var errSamples []string
	statusCount := map[string]int{}
	for _, r := range results {
		if r.OK {
			ok++
			latencies = append(latencies, r.Latency)
			if r.Status != "" {
				statusCount[r.Status]++
			}
		} else if r.Err != "" && len(errSamples) < printErrors {
			errSamples = append(errSamples, r.Err)
		}
	}
	fmt.Printf("send_code: ok=%d fail=%d elapsed=%s rps=%.2f p50=%s p95=%s p99=%s\n",
		ok, len(results)-ok, elapsed, float64(len(results))/elapsed.Seconds(),
		percentile(latencies, 0.50), percentile(latencies, 0.95), percentile(latencies, 0.99),
	)
	if len(statusCount) > 0 {
		fmt.Printf("delivery_status: %s\n", formatCountMap(statusCount))
	}
	if len(errSamples) > 0 {
		fmt.Printf("sample_errors:\n")
		for _, s := range errSamples {
			fmt.Printf("  - %s\n", s)
		}
	}
}

func printConsumeSummary(results []consumeResult, elapsed time.Duration, printErrors int) {
	statusCount := map[string]int{}
	var timeToSent []time.Duration
	var errSamples []string
	for _, r := range results {
		statusCount[r.FinalStatus]++
		if r.FinalStatus == "sent" {
			timeToSent = append(timeToSent, r.TimeToFinal)
		} else if (r.FinalStatus == "failed" || r.FinalStatus == "dropped" || r.FinalStatus == "timeout") && len(errSamples) < printErrors {
			msg := r.FinalStatus
			if r.LastError != "" {
				msg += ": " + r.LastError
			} else if r.HTTPErrorBody != "" {
				msg += ": " + r.HTTPErrorBody
			}
			errSamples = append(errSamples, msg)
		}
	}
	fmt.Printf("consume_ack: tasks=%d elapsed=%s sent=%d p50_tts=%s p95_tts=%s p99_tts=%s\n",
		len(results), elapsed, statusCount["sent"],
		percentile(timeToSent, 0.50), percentile(timeToSent, 0.95), percentile(timeToSent, 0.99),
	)
	fmt.Printf("final_status: %s\n", formatCountMap(statusCount))
	if len(errSamples) > 0 {
		fmt.Printf("sample_non_sent:\n")
		for _, s := range errSamples {
			fmt.Printf("  - %s\n", s)
		}
	}
}

func printRegisterSummary(results []registerResult, elapsed time.Duration, printErrors int) {
	var ok int
	var latencies []time.Duration
	var errSamples []string
	for _, r := range results {
		if r.OK {
			ok++
			latencies = append(latencies, r.Latency)
		} else if r.Err != "" && len(errSamples) < printErrors {
			errSamples = append(errSamples, r.Err)
		}
	}
	fmt.Printf("register: ok=%d fail=%d elapsed=%s rps=%.2f p50=%s p95=%s p99=%s\n",
		ok, len(results)-ok, elapsed, float64(len(results))/elapsed.Seconds(),
		percentile(latencies, 0.50), percentile(latencies, 0.95), percentile(latencies, 0.99),
	)
	if len(errSamples) > 0 {
		fmt.Printf("sample_errors:\n")
		for _, s := range errSamples {
			fmt.Printf("  - %s\n", s)
		}
	}
}

func percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	if p <= 0 {
		p = 0
	}
	if p >= 1 {
		p = 1
	}
	cp := make([]time.Duration, len(d))
	copy(cp, d)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	// Nearest-rank
	rank := int(math.Ceil(float64(len(cp))*p)) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(cp) {
		rank = len(cp) - 1
	}
	return cp[rank]
}

func formatCountMap(m map[string]int) string {
	type kv struct {
		k string
		v int
	}
	var arr []kv
	for k, v := range m {
		arr = append(arr, kv{k: k, v: v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	var parts []string
	for _, it := range arr {
		parts = append(parts, fmt.Sprintf("%s=%d", it.k, it.v))
	}
	return strings.Join(parts, " ")
}

func randomString(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// fallback
		now := time.Now().UnixNano()
		return fmt.Sprintf("%x", now)[:n]
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func compactBody(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

func fatalf(format string, a ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
