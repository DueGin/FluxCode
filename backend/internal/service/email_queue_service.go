package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	emailQueueStreamKey  = "fluxcode:queue:email:stream"
	emailQueueGroupName  = "fluxcode:queue:email:group"
	emailQueueDelayedKey = "fluxcode:queue:email:delayed"
	emailQueueStatusKey  = "fluxcode:queue:email:status:"

	emailQueueDefaultWorkers     = 3
	emailQueueDefaultMaxAttempts = 5
	emailQueueBaseRetryDelay     = 5 * time.Second
	emailQueueMaxRetryDelay      = 2 * time.Minute
	emailQueueClaimMinIdle       = 1 * time.Minute
	emailQueueBlock              = 5 * time.Second
	emailQueueBatchSize          = 10
	emailQueueStatusTTL          = 30 * time.Minute

	emailQueueAlertEmail = "1917764907@qq.com"
)

// EmailQueueService 异步邮件队列服务
type EmailQueueService struct {
	rdb            *redis.Client
	emailService   *EmailService
	wg             sync.WaitGroup
	stopChan       chan struct{}
	stopOnce       sync.Once
	workers        int
	consumerPrefix string
}

// EnqueueEmailResult 表示入队成功后的“发送确认 ack”（Producer ACK）
type EnqueueEmailResult struct {
	TaskID         string `json:"task_id"`
	QueueMessageID string `json:"queue_message_id"`
}

// EmailTaskStatus 表示邮件任务的可观测状态（用于“消费 ack”确认）
type EmailTaskStatus struct {
	TaskID         string `json:"task_id"`
	TaskType       string `json:"task_type"`
	Email          string `json:"email"`
	SiteName       string `json:"site_name"`
	Status         string `json:"status"` // enqueued | processing | retrying | sent | failed | dropped
	QueueMessageID string `json:"queue_message_id,omitempty"`
	Attempt        int    `json:"attempt"`
	NextAttempt    int    `json:"next_attempt,omitempty"`
	MaxAttempts    int    `json:"max_attempts"`
	SendAckAt      int64  `json:"send_ack_at"`              // unix ms
	ConsumeAckAt   int64  `json:"consume_ack_at,omitempty"` // unix ms（sent/failed/dropped）
	NextRetryAt    int64  `json:"next_retry_at,omitempty"`  // unix ms
	LastError      string `json:"last_error,omitempty"`     // last error message
	Worker         string `json:"worker,omitempty"`         // consumer id
	UpdatedAt      int64  `json:"updated_at"`               // unix ms
}

type emailTaskPayload struct {
	ID          string `json:"id"`
	TaskType    string `json:"task_type"`
	Email       string `json:"email"`
	SiteName    string `json:"site_name"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	EnqueuedAt  int64  `json:"enqueued_at"`
	LastError   string `json:"last_error,omitempty"`
}

var moveDueEmailTasksScript = redis.NewScript(`
local delayed_key = KEYS[1]
local stream_key = KEYS[2]
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])

local items = redis.call('ZRANGEBYSCORE', delayed_key, '-inf', now, 'LIMIT', 0, limit)
if #items == 0 then
  return {}
end

for i, payload in ipairs(items) do
  redis.call('XADD', stream_key, '*', 'payload', payload)
end

redis.call('ZREM', delayed_key, unpack(items))
return items
`)

func emailTaskStatusKey(taskID string) string {
	return emailQueueStatusKey + taskID
}

func (s *EmailQueueService) setTaskStatus(ctx context.Context, status EmailTaskStatus) {
	if ctx == nil {
		ctx = context.Background()
	}
	status.UpdatedAt = time.Now().UnixMilli()

	raw, err := json.Marshal(status)
	if err != nil {
		log.Printf("[EmailQueue] Failed to marshal task status (task_id=%s): %v", status.TaskID, err)
		return
	}

	if err := s.rdb.Set(ctx, emailTaskStatusKey(status.TaskID), raw, emailQueueStatusTTL).Err(); err != nil {
		log.Printf("[EmailQueue] Failed to persist task status (task_id=%s): %v", status.TaskID, err)
	}
}

// GetTaskStatus 查询邮件任务状态
func (s *EmailQueueService) GetTaskStatus(ctx context.Context, taskID string) (*EmailTaskStatus, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, nil
	}

	val, err := s.rdb.Get(ctx, emailTaskStatusKey(taskID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var status EmailTaskStatus
	if err := json.Unmarshal([]byte(val), &status); err != nil {
		return nil, fmt.Errorf("unmarshal task status: %w", err)
	}
	return &status, nil
}

// NewEmailQueueService 创建邮件队列服务（Redis 分布式队列）
func NewEmailQueueService(rdb *redis.Client, emailService *EmailService, workers int) *EmailQueueService {
	if workers <= 0 {
		workers = emailQueueDefaultWorkers
	}

	service := &EmailQueueService{
		rdb:          rdb,
		emailService: emailService,
		stopChan:     make(chan struct{}),
		workers:      workers,
	}

	host, _ := os.Hostname()
	service.consumerPrefix = fmt.Sprintf("%s-%d", host, os.Getpid())

	// 启动工作协程
	service.start()

	return service
}

// start 启动工作协程
func (s *EmailQueueService) start() {
	s.ensureConsumerGroup()

	// 延迟任务调度器：将到期重试任务搬运回 Stream
	s.wg.Add(1)
	go s.delayedDispatcher()

	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	log.Printf("[EmailQueue] Started %d workers", s.workers)
}

func (s *EmailQueueService) worker(id int) {
	defer s.wg.Done()

	consumer := fmt.Sprintf("%s-%d", s.consumerPrefix, id)
	ctx := context.Background()

	for {
		select {
		case <-s.stopChan:
			log.Printf("[EmailQueue] Worker %d stopping", id)
			return
		default:
		}

		// 优先回收处理超时/崩溃导致的 pending 消息，避免消息卡死
		s.claimAndProcess(ctx, id, consumer)

		streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    emailQueueGroupName,
			Consumer: consumer,
			Streams:  []string{emailQueueStreamKey, ">"},
			Count:    emailQueueBatchSize,
			Block:    emailQueueBlock,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			log.Printf("[EmailQueue] Worker %d XREADGROUP error: %v", id, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				s.handleMessage(ctx, id, consumer, msg)
			}
		}
	}
}

func (s *EmailQueueService) claimAndProcess(ctx context.Context, workerID int, consumer string) {
	messages, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   emailQueueStreamKey,
		Group:    emailQueueGroupName,
		Consumer: consumer,
		MinIdle:  emailQueueClaimMinIdle,
		Start:    "0-0",
		Count:    emailQueueBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		// Redis < 6.2 不支持 XAUTOCLAIM：退化到 XPENDING + XCLAIM
		if strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "ERR unknown command") {
			s.claimAndProcessFallback(ctx, workerID, consumer)
			return
		}
		log.Printf("[EmailQueue] Worker %d XAUTOCLAIM error: %v", workerID, err)
		return
	}
	for _, msg := range messages {
		s.handleMessage(ctx, workerID, consumer, msg)
	}
}

func (s *EmailQueueService) claimAndProcessFallback(ctx context.Context, workerID int, consumer string) {
	pending, err := s.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: emailQueueStreamKey,
		Group:  emailQueueGroupName,
		Idle:   emailQueueClaimMinIdle,
		Start:  "-",
		End:    "+",
		Count:  emailQueueBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		log.Printf("[EmailQueue] Worker %d XPENDING error: %v", workerID, err)
		return
	}
	if len(pending) == 0 {
		return
	}

	ids := make([]string, 0, len(pending))
	for _, item := range pending {
		ids = append(ids, item.ID)
	}

	messages, err := s.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   emailQueueStreamKey,
		Group:    emailQueueGroupName,
		Consumer: consumer,
		MinIdle:  emailQueueClaimMinIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		log.Printf("[EmailQueue] Worker %d XCLAIM error: %v", workerID, err)
		return
	}

	for _, msg := range messages {
		s.handleMessage(ctx, workerID, consumer, msg)
	}
}

// EnqueueVerifyCode 将验证码发送任务加入队列
func (s *EmailQueueService) EnqueueVerifyCode(ctx context.Context, email, siteName string) (*EnqueueEmailResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	payload := emailTaskPayload{
		ID:          uuid.NewString(),
		TaskType:    "verify_code",
		Email:       email,
		SiteName:    siteName,
		Attempt:     1,
		MaxAttempts: emailQueueDefaultMaxAttempts,
		EnqueuedAt:  time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal email task payload: %w", err)
	}

	queueMessageID, err := s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: emailQueueStreamKey,
		Values: map[string]any{
			"payload": string(raw),
		},
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("enqueue verify code: %w", err)
	}

	s.setTaskStatus(ctx, EmailTaskStatus{
		TaskID:         payload.ID,
		TaskType:       payload.TaskType,
		Email:          payload.Email,
		SiteName:       payload.SiteName,
		Status:         "enqueued",
		QueueMessageID: queueMessageID,
		Attempt:        payload.Attempt,
		MaxAttempts:    payload.MaxAttempts,
		SendAckAt:      payload.EnqueuedAt,
		Worker:         "",
	})

	log.Printf("[EmailQueue] Enqueued verify code task (task_id=%s, msg_id=%s) for %s", payload.ID, queueMessageID, email)
	return &EnqueueEmailResult{
		TaskID:         payload.ID,
		QueueMessageID: queueMessageID,
	}, nil
}

// Stop 停止队列服务
func (s *EmailQueueService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
	s.wg.Wait()
	log.Println("[EmailQueue] All workers stopped")
}

func (s *EmailQueueService) ensureConsumerGroup() {
	ctx := context.Background()
	if err := s.rdb.XGroupCreateMkStream(ctx, emailQueueStreamKey, emailQueueGroupName, "0").Err(); err != nil {
		// BUSYGROUP: group already exists
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("[EmailQueue] Failed to create consumer group: %v", err)
		}
	}
}

func (s *EmailQueueService) delayedDispatcher() {
	defer s.wg.Done()

	ctx := context.Background()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			log.Printf("[EmailQueue] Delayed dispatcher stopping")
			return
		case <-ticker.C:
		}

		now := time.Now().UnixMilli()
		result, err := moveDueEmailTasksScript.Run(ctx, s.rdb, []string{emailQueueDelayedKey, emailQueueStreamKey}, now, emailQueueBatchSize).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			log.Printf("[EmailQueue] Delayed dispatcher script error: %v", err)
			continue
		}

		items, ok := result.([]interface{})
		if !ok || len(items) == 0 {
			continue
		}
		log.Printf("[EmailQueue] Moved %d delayed tasks back to stream", len(items))
	}
}

func (s *EmailQueueService) handleMessage(ctx context.Context, workerID int, consumer string, msg redis.XMessage) {
	payload, err := s.parsePayload(msg)
	if err != nil {
		log.Printf("[EmailQueue] Worker %d invalid payload (id=%s): %v", workerID, msg.ID, err)
		s.ackAndDelete(ctx, workerID, msg.ID)
		return
	}

	if payload.Attempt <= 0 {
		payload.Attempt = 1
	}
	if payload.MaxAttempts <= 0 {
		payload.MaxAttempts = emailQueueDefaultMaxAttempts
	}

	s.setTaskStatus(ctx, EmailTaskStatus{
		TaskID:         payload.ID,
		TaskType:       payload.TaskType,
		Email:          payload.Email,
		SiteName:       payload.SiteName,
		Status:         "processing",
		QueueMessageID: msg.ID,
		Attempt:        payload.Attempt,
		MaxAttempts:    payload.MaxAttempts,
		SendAckAt:      payload.EnqueuedAt,
		Worker:         consumer,
	})

	taskCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	procErr := s.processTask(taskCtx, payload)
	if procErr == nil {
		s.setTaskStatus(ctx, EmailTaskStatus{
			TaskID:         payload.ID,
			TaskType:       payload.TaskType,
			Email:          payload.Email,
			SiteName:       payload.SiteName,
			Status:         "sent",
			QueueMessageID: msg.ID,
			Attempt:        payload.Attempt,
			MaxAttempts:    payload.MaxAttempts,
			SendAckAt:      payload.EnqueuedAt,
			ConsumeAckAt:   time.Now().UnixMilli(),
			Worker:         consumer,
		})
		s.ackAndDelete(ctx, workerID, msg.ID)
		return
	}

	// 冷却限制：认为是“可预期的拒绝”，不做重试（避免队列堆积/邮件轰炸）
	if errors.Is(procErr, ErrVerifyCodeTooFrequent) {
		log.Printf("[EmailQueue] Worker %d drop task(type=%s,email=%s) due to cooldown", workerID, payload.TaskType, payload.Email)
		s.setTaskStatus(ctx, EmailTaskStatus{
			TaskID:         payload.ID,
			TaskType:       payload.TaskType,
			Email:          payload.Email,
			SiteName:       payload.SiteName,
			Status:         "dropped",
			QueueMessageID: msg.ID,
			Attempt:        payload.Attempt,
			MaxAttempts:    payload.MaxAttempts,
			SendAckAt:      payload.EnqueuedAt,
			ConsumeAckAt:   time.Now().UnixMilli(),
			LastError:      procErr.Error(),
			Worker:         consumer,
		})
		s.ackAndDelete(ctx, workerID, msg.ID)
		return
	}

	// 达到最大重试次数：发送告警并丢弃任务
	if payload.Attempt >= payload.MaxAttempts {
		log.Printf("[EmailQueue] Worker %d giving up task(type=%s,email=%s) after %d attempts: %v",
			workerID, payload.TaskType, payload.Email, payload.Attempt, procErr)
		s.sendAlert(taskCtx, payload, procErr)
		s.setTaskStatus(ctx, EmailTaskStatus{
			TaskID:         payload.ID,
			TaskType:       payload.TaskType,
			Email:          payload.Email,
			SiteName:       payload.SiteName,
			Status:         "failed",
			QueueMessageID: msg.ID,
			Attempt:        payload.Attempt,
			MaxAttempts:    payload.MaxAttempts,
			SendAckAt:      payload.EnqueuedAt,
			ConsumeAckAt:   time.Now().UnixMilli(),
			LastError:      procErr.Error(),
			Worker:         consumer,
		})
		s.ackAndDelete(ctx, workerID, msg.ID)
		return
	}

	// 进入重试：先调度，再 ACK（调度失败则不 ACK，保持 pending 以便后续自动回收）
	nextPayload, nextAt, _, err := s.scheduleRetry(ctx, payload, procErr)
	if err != nil {
		log.Printf("[EmailQueue] Worker %d schedule retry failed (task id=%s): %v", workerID, payload.ID, err)
		return
	}
	s.setTaskStatus(ctx, EmailTaskStatus{
		TaskID:         payload.ID,
		TaskType:       payload.TaskType,
		Email:          payload.Email,
		SiteName:       payload.SiteName,
		Status:         "retrying",
		QueueMessageID: msg.ID,
		Attempt:        payload.Attempt,
		NextAttempt:    nextPayload.Attempt,
		MaxAttempts:    payload.MaxAttempts,
		SendAckAt:      payload.EnqueuedAt,
		NextRetryAt:    nextAt,
		LastError:      procErr.Error(),
		Worker:         consumer,
	})
	s.ackAndDelete(ctx, workerID, msg.ID)
}

func (s *EmailQueueService) parsePayload(msg redis.XMessage) (emailTaskPayload, error) {
	rawValue, ok := msg.Values["payload"]
	if !ok {
		return emailTaskPayload{}, fmt.Errorf("missing payload field")
	}

	var raw string
	switch v := rawValue.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		raw = fmt.Sprint(v)
	}

	var payload emailTaskPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return emailTaskPayload{}, fmt.Errorf("unmarshal payload: %w", err)
	}
	return payload, nil
}

func (s *EmailQueueService) processTask(ctx context.Context, payload emailTaskPayload) error {
	switch payload.TaskType {
	case "verify_code":
		if payload.Attempt <= 1 {
			return s.emailService.SendVerifyCode(ctx, payload.Email, payload.SiteName)
		}
		return s.emailService.ResendVerifyCode(ctx, payload.Email, payload.SiteName)
	default:
		log.Printf("[EmailQueue] Unknown task type: %s", payload.TaskType)
		return nil
	}
}

func (s *EmailQueueService) scheduleRetry(
	ctx context.Context,
	payload emailTaskPayload,
	cause error,
) (emailTaskPayload, int64, time.Duration, error) {
	nextPayload := payload
	nextPayload.Attempt = payload.Attempt + 1
	nextPayload.LastError = cause.Error()

	delay := emailQueueRetryDelay(nextPayload.Attempt)
	nextAt := time.Now().Add(delay).UnixMilli()

	raw, err := json.Marshal(nextPayload)
	if err != nil {
		return emailTaskPayload{}, 0, 0, fmt.Errorf("marshal retry payload: %w", err)
	}

	if err := s.rdb.ZAdd(ctx, emailQueueDelayedKey, redis.Z{
		Score:  float64(nextAt),
		Member: string(raw),
	}).Err(); err != nil {
		return emailTaskPayload{}, 0, 0, fmt.Errorf("zadd delayed task: %w", err)
	}

	log.Printf("[EmailQueue] Scheduled retry (attempt=%d) for %s in %s", nextPayload.Attempt, payload.Email, delay)
	return nextPayload, nextAt, delay, nil
}

func emailQueueRetryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return emailQueueBaseRetryDelay
	}

	exp := attempt - 2 // attempt=2 -> 0
	if exp < 0 {
		exp = 0
	}
	if exp > 10 {
		exp = 10
	}

	delay := emailQueueBaseRetryDelay * time.Duration(1<<uint(exp))
	if delay > emailQueueMaxRetryDelay {
		delay = emailQueueMaxRetryDelay
	}
	return delay
}

func (s *EmailQueueService) ackAndDelete(ctx context.Context, workerID int, messageID string) {
	if err := s.rdb.XAck(ctx, emailQueueStreamKey, emailQueueGroupName, messageID).Err(); err != nil {
		log.Printf("[EmailQueue] Worker %d XACK failed (id=%s): %v", workerID, messageID, err)
	}
	if err := s.rdb.XDel(ctx, emailQueueStreamKey, messageID).Err(); err != nil {
		log.Printf("[EmailQueue] Worker %d XDEL failed (id=%s): %v", workerID, messageID, err)
	}
}

func (s *EmailQueueService) sendAlert(ctx context.Context, payload emailTaskPayload, cause error) {
	if s.emailService == nil {
		return
	}

	alertCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	subject := fmt.Sprintf("[%s] 邮件队列发送失败告警", payload.SiteName)
	body := fmt.Sprintf(
		`<h2>邮件队列任务最终失败</h2>
<p><b>任务类型</b>：%s</p>
<p><b>收件邮箱</b>：%s</p>
<p><b>站点名称</b>：%s</p>
<p><b>尝试次数</b>：%d</p>
<p><b>最后错误</b>：%s</p>
<p><b>任务ID</b>：%s</p>
<p><b>服务实例</b>：%s</p>
<p><b>时间</b>：%s</p>`,
		payload.TaskType,
		payload.Email,
		payload.SiteName,
		payload.Attempt,
		cause.Error(),
		payload.ID,
		s.consumerPrefix,
		time.Now().Format(time.RFC3339),
	)

	if err := s.emailService.SendEmail(alertCtx, emailQueueAlertEmail, subject, body); err != nil {
		log.Printf("[EmailQueue] Failed to send alert email: %v", err)
	}
}
