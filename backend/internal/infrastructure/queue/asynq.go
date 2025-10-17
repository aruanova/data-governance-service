package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alejandroruanova/data-governance-service/backend/internal/pkg/config"
	"github.com/hibiken/asynq"
)

// AsynqClient wraps the Asynq client for enqueuing tasks
type AsynqClient struct {
	client *asynq.Client
	logger *slog.Logger
}

// NewAsynqClient creates a new Asynq client
func NewAsynqClient(cfg *config.QueueConfig, logger *slog.Logger) (*AsynqClient, error) {
	redisOpt := asynq.RedisClientOpt{
		Addr:         fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	}

	client := asynq.NewClient(redisOpt)

	logger.Info("asynq client created",
		slog.String("redis_host", cfg.RedisHost),
		slog.Int("redis_port", cfg.RedisPort),
	)

	return &AsynqClient{
		client: client,
		logger: logger,
	}, nil
}

// Close closes the Asynq client
func (a *AsynqClient) Close() error {
	a.logger.Info("closing asynq client")
	return a.client.Close()
}

// Enqueue adds a task to the queue
func (a *AsynqClient) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := a.client.Enqueue(task, opts...)
	if err != nil {
		a.logger.Error("failed to enqueue task",
			slog.String("task_type", task.Type()),
			slog.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("task enqueued",
		slog.String("task_id", info.ID),
		slog.String("task_type", task.Type()),
		slog.String("queue", info.Queue),
	)

	return info, nil
}

// EnqueueContext enqueues a task with context
func (a *AsynqClient) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := a.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		a.logger.Error("failed to enqueue task",
			slog.String("task_type", task.Type()),
			slog.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("task enqueued",
		slog.String("task_id", info.ID),
		slog.String("task_type", task.Type()),
		slog.String("queue", info.Queue),
	)

	return info, nil
}

// AsynqServer wraps the Asynq server for processing tasks
type AsynqServer struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
}

// NewAsynqServer creates a new Asynq server
func NewAsynqServer(cfg *config.QueueConfig, logger *slog.Logger) (*AsynqServer, error) {
	redisOpt := asynq.RedisClientOpt{
		Addr:         fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				"critical": 6, // Highest priority
				"high":     3,
				"default":  1,
			},
			StrictPriority: cfg.StrictPriority,

			// Retry configuration
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 2s, 4s, 8s, 16s, ...
				return time.Duration(1<<uint(n)) * time.Second
			},

			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("task processing failed",
					slog.String("task_type", task.Type()),
					slog.String("payload", string(task.Payload())),
					slog.Error(err),
				)
			}),

			// Health check
			HealthCheckFunc: func(e error) {
				if e != nil {
					logger.Error("health check failed", slog.Error(e))
				}
			},
			HealthCheckInterval: 20 * time.Second,

			// Graceful shutdown
			ShutdownTimeout: 25 * time.Second,
		},
	)

	mux := asynq.NewServeMux()

	logger.Info("asynq server created",
		slog.String("redis_host", cfg.RedisHost),
		slog.Int("redis_port", cfg.RedisPort),
		slog.Int("concurrency", cfg.Concurrency),
	)

	return &AsynqServer{
		server: server,
		mux:    mux,
		logger: logger,
	}, nil
}

// HandleFunc registers a handler function for a task type
func (a *AsynqServer) HandleFunc(pattern string, handler func(context.Context, *asynq.Task) error) {
	a.mux.HandleFunc(pattern, handler)
	a.logger.Debug("handler registered", slog.String("pattern", pattern))
}

// Use adds a middleware to the mux
func (a *AsynqServer) Use(middleware func(asynq.Handler) asynq.Handler) {
	a.mux.Use(middleware)
}

// Start starts the Asynq server
func (a *AsynqServer) Start() error {
	a.logger.Info("starting asynq server")
	if err := a.server.Run(a.mux); err != nil {
		return fmt.Errorf("failed to run asynq server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (a *AsynqServer) Shutdown() {
	a.logger.Info("shutting down asynq server")
	a.server.Shutdown()
}

// Stop immediately stops the server
func (a *AsynqServer) Stop() {
	a.logger.Info("stopping asynq server")
	a.server.Stop()
}

// Task Types (constants for task identification)
const (
	TaskTypeLLMClassify = "llm:classify"
	TaskTypeBatchProcess = "batch:process"
	TaskTypeCleanData = "clean:data"
	TaskTypeGenerateSample = "sample:generate"
	TaskTypeExportResults = "export:results"
)