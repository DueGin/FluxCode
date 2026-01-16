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

	dbent "github.com/DueGin/FluxCode/ent"
	"github.com/DueGin/FluxCode/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	usageQueueStreamKey = "fluxcode:queue:usage:stream"
	usageQueueGroupName = "fluxcode:queue:usage:group"

	usageQueueDefaultWorkers = 4
	usageQueueBatchSize      = 50
	usageQueueBlock          = 5 * time.Second
	usageQueueClaimMinIdle   = 30 * time.Second
)

type usageQueueKind string

const (
	usageQueueKindClaude usageQueueKind = "claude"
	usageQueueKindOpenAI usageQueueKind = "openai"
)

type usageQueuePayload struct {
	ID string `json:"id"`

	Kind usageQueueKind `json:"kind"`

	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	Stream    bool   `json:"stream"`

	UserID    int64 `json:"user_id"`
	APIKeyID  int64 `json:"api_key_id"`
	AccountID int64 `json:"account_id"`

	GroupID        *int64 `json:"group_id,omitempty"`
	SubscriptionID *int64 `json:"subscription_id,omitempty"`

	RateMultiplier float64 `json:"rate_multiplier"`
	BillingType    int8    `json:"billing_type"` // 0=balance, 1=subscription

	InputTokens         int  `json:"input_tokens"`
	OutputTokens        int  `json:"output_tokens"`
	CacheCreationTokens int  `json:"cache_creation_tokens"`
	CacheReadTokens     int  `json:"cache_read_tokens"`
	DurationMs          int  `json:"duration_ms"`
	FirstTokenMs        *int `json:"first_token_ms,omitempty"`

	EnqueuedAt int64 `json:"enqueued_at"` // unix ms
}

// UsageQueueService 用于异步记账的 Redis 持久化队列（Stream + Consumer Group）
type UsageQueueService struct {
	rdb *redis.Client

	entClient *dbent.Client

	cfg                 *config.Config
	billingService      *BillingService
	usageLogRepo        UsageLogRepository
	userRepo            UserRepository
	userSubRepo         UserSubscriptionRepository
	billingCacheService *BillingCacheService
	deferredService     *DeferredService

	wg       sync.WaitGroup
	stopChan chan struct{}
	stopOnce sync.Once

	workers        int
	consumerPrefix string
}

func NewUsageQueueService(
	rdb *redis.Client,
	entClient *dbent.Client,
	cfg *config.Config,
	billingService *BillingService,
	usageLogRepo UsageLogRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	billingCacheService *BillingCacheService,
	deferredService *DeferredService,
	workers int,
) *UsageQueueService {
	if workers <= 0 {
		workers = usageQueueDefaultWorkers
	}

	s := &UsageQueueService{
		rdb:                rdb,
		entClient:          entClient,
		cfg:                cfg,
		billingService:     billingService,
		usageLogRepo:       usageLogRepo,
		userRepo:           userRepo,
		userSubRepo:        userSubRepo,
		billingCacheService: billingCacheService,
		deferredService:    deferredService,
		stopChan:           make(chan struct{}),
		workers:            workers,
	}

	host, _ := os.Hostname()
	s.consumerPrefix = fmt.Sprintf("%s-%d", host, os.Getpid())

	s.start()
	return s
}

func (s *UsageQueueService) start() {
	s.ensureConsumerGroup()

	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	log.Printf("[UsageQueue] Started %d workers", s.workers)
}

func (s *UsageQueueService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
	s.wg.Wait()
	log.Println("[UsageQueue] All workers stopped")
}

func (s *UsageQueueService) ensureConsumerGroup() {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	if err := s.rdb.XGroupCreateMkStream(ctx, usageQueueStreamKey, usageQueueGroupName, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("[UsageQueue] Failed to create consumer group: %v", err)
		}
	}
}

func (s *UsageQueueService) worker(id int) {
	defer s.wg.Done()

	if s.rdb == nil {
		log.Printf("[UsageQueue] Worker %d disabled: redis client is nil", id)
		return
	}

	consumer := fmt.Sprintf("%s-%d", s.consumerPrefix, id)
	ctx := context.Background()

	for {
		select {
		case <-s.stopChan:
			log.Printf("[UsageQueue] Worker %d stopping", id)
			return
		default:
		}

		s.claimAndProcess(ctx, id, consumer)

		streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    usageQueueGroupName,
			Consumer: consumer,
			Streams:  []string{usageQueueStreamKey, ">"},
			Count:    usageQueueBatchSize,
			Block:    usageQueueBlock,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			log.Printf("[UsageQueue] Worker %d XREADGROUP error: %v", id, err)
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

func (s *UsageQueueService) claimAndProcess(ctx context.Context, workerID int, consumer string) {
	msgs, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   usageQueueStreamKey,
		Group:    usageQueueGroupName,
		Consumer: consumer,
		MinIdle:  usageQueueClaimMinIdle,
		Start:    "0-0",
		Count:    usageQueueBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		// Redis < 6.2 不支持 XAUTOCLAIM：忽略，由人工处理（或升级 Redis）
		if strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "ERR unknown command") {
			return
		}
		log.Printf("[UsageQueue] Worker %d XAUTOCLAIM error: %v", workerID, err)
		return
	}

	for _, msg := range msgs {
		s.handleMessage(ctx, workerID, consumer, msg)
	}
}

func (s *UsageQueueService) handleMessage(ctx context.Context, workerID int, consumer string, msg redis.XMessage) {
	payload, err := parseUsageQueuePayload(msg)
	if err != nil {
		log.Printf("[UsageQueue] Worker %d invalid payload (id=%s): %v", workerID, msg.ID, err)
		s.ackAndDelete(ctx, workerID, msg.ID)
		return
	}

	taskCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.processPayload(taskCtx, payload); err != nil {
		log.Printf("[UsageQueue] Worker %d process failed (msg_id=%s task_id=%s kind=%s): %v", workerID, msg.ID, payload.ID, payload.Kind, err)
		// 不 ACK：保留 pending，等待 XAUTOCLAIM 重试
		return
	}

	s.ackAndDelete(ctx, workerID, msg.ID)
	_ = consumer
}

func parseUsageQueuePayload(msg redis.XMessage) (*usageQueuePayload, error) {
	raw, ok := msg.Values["payload"]
	if !ok {
		return nil, errors.New("missing payload field")
	}

	var s string
	switch v := raw.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		s = fmt.Sprintf("%v", v)
	}

	var payload usageQueuePayload
	if err := json.Unmarshal([]byte(s), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		payload.ID = uuid.NewString()
	}
	if payload.Kind != usageQueueKindClaude && payload.Kind != usageQueueKindOpenAI {
		return nil, fmt.Errorf("unknown kind: %s", payload.Kind)
	}
	if payload.UserID <= 0 || payload.APIKeyID <= 0 || payload.AccountID <= 0 {
		return nil, fmt.Errorf("invalid ids: user=%d api_key=%d account=%d", payload.UserID, payload.APIKeyID, payload.AccountID)
	}
	if strings.TrimSpace(payload.Model) == "" {
		return nil, errors.New("model is empty")
	}
	return &payload, nil
}

func (s *UsageQueueService) ackAndDelete(ctx context.Context, workerID int, msgID string) {
	if err := s.rdb.XAck(ctx, usageQueueStreamKey, usageQueueGroupName, msgID).Err(); err != nil {
		log.Printf("[UsageQueue] Worker %d XACK failed (id=%s): %v", workerID, msgID, err)
	}
	if err := s.rdb.XDel(ctx, usageQueueStreamKey, msgID).Err(); err != nil {
		log.Printf("[UsageQueue] Worker %d XDEL failed (id=%s): %v", workerID, msgID, err)
	}
}

func (s *UsageQueueService) processPayload(ctx context.Context, payload *usageQueuePayload) error {
	if payload == nil {
		return nil
	}
	if s.entClient == nil {
		return errors.New("ent client is nil")
	}
	if s.billingService == nil {
		return errors.New("billing service is nil")
	}

	// 计算 input_tokens（OpenAI 需要减去 cache_read_tokens）
	inputTokensForLog := payload.InputTokens
	inputTokensForPricing := payload.InputTokens
	if payload.Kind == usageQueueKindOpenAI {
		actualInput := payload.InputTokens - payload.CacheReadTokens
		if actualInput < 0 {
			actualInput = 0
		}
		inputTokensForLog = actualInput
		inputTokensForPricing = actualInput
	}

	multiplier := payload.RateMultiplier
	if multiplier <= 0 {
		if s.cfg != nil && s.cfg.Default.RateMultiplier > 0 {
			multiplier = s.cfg.Default.RateMultiplier
		} else {
			multiplier = 1
		}
	}

	tokens := UsageTokens{
		InputTokens:         inputTokensForPricing,
		OutputTokens:        payload.OutputTokens,
		CacheCreationTokens: payload.CacheCreationTokens,
		CacheReadTokens:     payload.CacheReadTokens,
	}

	cost, err := s.billingService.CalculateCost(payload.Model, tokens, multiplier)
	if err != nil {
		log.Printf("[UsageQueue] CalculateCost failed (task_id=%s model=%s): %v", payload.ID, payload.Model, err)
		cost = &CostBreakdown{ActualCost: 0}
	}

	createdAt := time.Now()
	if payload.EnqueuedAt > 0 {
		createdAt = time.UnixMilli(payload.EnqueuedAt)
	}
	durationMs := payload.DurationMs

	usageLog := &UsageLog{
		UserID:              payload.UserID,
		APIKeyID:            payload.APIKeyID,
		AccountID:           payload.AccountID,
		RequestID:           payload.RequestID,
		Model:               payload.Model,
		InputTokens:         inputTokensForLog,
		OutputTokens:        payload.OutputTokens,
		CacheCreationTokens: payload.CacheCreationTokens,
		CacheReadTokens:     payload.CacheReadTokens,
		InputCost:           cost.InputCost,
		OutputCost:          cost.OutputCost,
		CacheCreationCost:   cost.CacheCreationCost,
		CacheReadCost:       cost.CacheReadCost,
		TotalCost:           cost.TotalCost,
		ActualCost:          cost.ActualCost,
		RateMultiplier:      multiplier,
		BillingType:         payload.BillingType,
		Stream:              payload.Stream,
		DurationMs:          &durationMs,
		FirstTokenMs:        payload.FirstTokenMs,
		CreatedAt:           createdAt,
	}

	var groupIDValue int64
	if payload.GroupID != nil {
		groupIDValue = *payload.GroupID
		usageLog.GroupID = &groupIDValue
	}

	var subscriptionIDValue int64
	if payload.SubscriptionID != nil {
		subscriptionIDValue = *payload.SubscriptionID
		usageLog.SubscriptionID = &subscriptionIDValue
	}

	// 事务保证「使用日志插入」与「扣费/订阅用量更新」的原子性，避免重复扣费或漏扣风险
	tx, txErr := s.entClient.Tx(ctx)
	if txErr != nil && !errors.Is(txErr, dbent.ErrTxStarted) {
		return fmt.Errorf("begin transaction: %w", txErr)
	}

	txCtx := ctx
	if txErr == nil {
		defer func() { _ = tx.Rollback() }()
		txCtx = dbent.NewTxContext(ctx, tx)
	}

	inserted, err := s.usageLogRepo.Create(txCtx, usageLog)
	if err != nil {
		return fmt.Errorf("create usage log: %w", err)
	}

	// SIMPLE 模式：不扣费/不更新订阅用量
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		if tx != nil {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit transaction: %w", err)
			}
		}
		if s.deferredService != nil {
			s.deferredService.ScheduleLastUsedUpdate(payload.AccountID)
		}
		return nil
	}

	// 已存在 usage_log（幂等重试）：不重复扣费/不重复更新订阅用量
	if inserted {
		switch payload.BillingType {
		case BillingTypeSubscription:
			if payload.SubscriptionID == nil {
				return errors.New("subscription billing but subscription_id is nil")
			}
			if cost.TotalCost > 0 {
				if err := s.userSubRepo.IncrementUsage(txCtx, subscriptionIDValue, cost.TotalCost); err != nil {
					return fmt.Errorf("increment subscription usage: %w", err)
				}
			}
		case BillingTypeBalance:
			if cost.ActualCost > 0 {
				if err := s.userRepo.DeductBalance(txCtx, payload.UserID, cost.ActualCost); err != nil {
					// 余额不足：不重试（避免队列阻塞），只保留 usage log
					if errors.Is(err, ErrInsufficientBalance) {
						log.Printf("[UsageQueue] Insufficient balance, skip deduct (user=%d cost=%.6f request_id=%s)", payload.UserID, cost.ActualCost, payload.RequestID)
					} else {
						return fmt.Errorf("deduct balance: %w", err)
					}
				}
			}
		default:
			return fmt.Errorf("unknown billing type: %d", payload.BillingType)
		}
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
	}

	// 事务提交成功后更新缓存/异步 last_used（避免在回滚时污染缓存）
	if inserted && s.billingCacheService != nil {
		switch payload.BillingType {
		case BillingTypeSubscription:
			if payload.GroupID != nil && cost.TotalCost > 0 {
				s.billingCacheService.QueueUpdateSubscriptionUsage(payload.UserID, groupIDValue, cost.TotalCost)
			}
		case BillingTypeBalance:
			if cost.ActualCost > 0 {
				s.billingCacheService.QueueDeductBalance(payload.UserID, cost.ActualCost)
			}
		}
	}
	if s.deferredService != nil {
		s.deferredService.ScheduleLastUsedUpdate(payload.AccountID)
	}

	return nil
}

func (s *UsageQueueService) enqueue(ctx context.Context, payload *usageQueuePayload) (string, error) {
	if s.rdb == nil {
		return "", errors.New("redis client is nil")
	}
	if payload == nil {
		return "", errors.New("payload is nil")
	}
	if strings.TrimSpace(payload.ID) == "" {
		payload.ID = uuid.NewString()
	}
	payload.EnqueuedAt = time.Now().UnixMilli()

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	msgID, err := s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: usageQueueStreamKey,
		Values: map[string]any{
			"kind":    string(payload.Kind),
			"payload": string(b),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("xadd: %w", err)
	}
	return msgID, nil
}

func (s *UsageQueueService) EnqueueClaudeUsage(ctx context.Context, result *ForwardResult, apiKey *APIKey, account *Account, subscription *UserSubscription) (string, error) {
	if result == nil || apiKey == nil || apiKey.User == nil || account == nil {
		return "", errors.New("nil input")
	}

	multiplier := 0.0
	if apiKey.GroupID != nil && apiKey.Group != nil {
		multiplier = apiKey.Group.RateMultiplier
	}

	billingType := BillingTypeBalance
	if subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		billingType = BillingTypeSubscription
	}

	var subscriptionID *int64
	if subscription != nil {
		id := subscription.ID
		subscriptionID = &id
	}

	durationMs := int(result.Duration.Milliseconds())

	payload := &usageQueuePayload{
		ID:          uuid.NewString(),
		Kind:        usageQueueKindClaude,
		RequestID:    result.RequestID,
		Model:        result.Model,
		Stream:       result.Stream,
		UserID:       apiKey.User.ID,
		APIKeyID:     apiKey.ID,
		AccountID:    account.ID,
		GroupID:      apiKey.GroupID,
		SubscriptionID: subscriptionID,
		RateMultiplier: multiplier,
		BillingType:    billingType,
		InputTokens:    result.Usage.InputTokens,
		OutputTokens:   result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		DurationMs:          durationMs,
		FirstTokenMs:        result.FirstTokenMs,
	}

	return s.enqueue(ctx, payload)
}

func (s *UsageQueueService) EnqueueOpenAIUsage(ctx context.Context, result *OpenAIForwardResult, apiKey *APIKey, account *Account, subscription *UserSubscription) (string, error) {
	if result == nil || apiKey == nil || apiKey.User == nil || account == nil {
		return "", errors.New("nil input")
	}

	multiplier := 0.0
	if apiKey.GroupID != nil && apiKey.Group != nil {
		multiplier = apiKey.Group.RateMultiplier
	}

	billingType := BillingTypeBalance
	if subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		billingType = BillingTypeSubscription
	}

	var subscriptionID *int64
	if subscription != nil {
		id := subscription.ID
		subscriptionID = &id
	}

	durationMs := int(result.Duration.Milliseconds())

	payload := &usageQueuePayload{
		ID:          uuid.NewString(),
		Kind:        usageQueueKindOpenAI,
		RequestID:    result.RequestID,
		Model:        result.Model,
		Stream:       result.Stream,
		UserID:       apiKey.User.ID,
		APIKeyID:     apiKey.ID,
		AccountID:    account.ID,
		GroupID:      apiKey.GroupID,
		SubscriptionID: subscriptionID,
		RateMultiplier: multiplier,
		BillingType:    billingType,
		InputTokens:    result.Usage.InputTokens,
		OutputTokens:   result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		DurationMs:          durationMs,
		FirstTokenMs:        result.FirstTokenMs,
	}

	return s.enqueue(ctx, payload)
}

