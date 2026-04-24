package kafka_logger_pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

const (
	dockerLogDir   = "/apps/logs"
	dockerFlagFile = "/.dockerenv"
)

type LogConsumer struct {
	brokers []string
	topic   string
	groupID string
	logDir  string
	logger  *zap.Logger
	files   map[string]*os.File
	mu      sync.Mutex
}

func NewLogConsumer(brokers []string, topic, groupID, logDir string, logger *zap.Logger) *LogConsumer {
	resolvedDir := resolveLogDir(logDir, logger)
	return &LogConsumer{
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
		logDir:  resolvedDir,
		logger:  logger,
		files:   make(map[string]*os.File),
	}
}

// resolveLogDir picks the correct log directory based on environment.
func resolveLogDir(cfgDir string, logger *zap.Logger) string {
	// ── Docker environment ────────────────────────────────────────────
	if isDocker() {
		if dirExists(dockerLogDir) {
			logger.Info("docker environment detected",
				zap.String("log_dir", dockerLogDir),
			)
			return dockerLogDir
		}
		logger.Warn("docker detected but /apps/logs missing, falling back to desktop",
			zap.String("fallback", cfgDir),
		)
	}

	// ── Local: use Desktop/logs ───────────────────────────────────────
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("could not get home dir, using ./logs", zap.Error(err))
		return "./logs"
	}

	desktopLogDir := filepath.Join(homeDir, "Desktop", "logs")

	if dirExists(desktopLogDir) {
		logger.Info("using existing desktop log dir",
			zap.String("log_dir", desktopLogDir),
		)
		return desktopLogDir
	}

	// Create Desktop/logs if it doesn't exist
	if err := os.MkdirAll(desktopLogDir, 0755); err != nil {
		logger.Warn("could not create desktop log dir, falling back to ./logs",
			zap.String("path", desktopLogDir),
			zap.Error(err),
		)
		return "./logs"
	}

	logger.Info("created desktop log dir", zap.String("log_dir", desktopLogDir))
	return desktopLogDir
}

// isDocker checks if running inside a Docker container.
func isDocker() bool {
	_, err := os.Stat(dockerFlagFile)
	return err == nil
}

// dirExists checks if a directory exists and is accessible.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (c *LogConsumer) Start(ctx context.Context) error {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(c.brokers, c.groupID, cfg)
	if err != nil {
		return fmt.Errorf("creating consumer group: %w", err)
	}
	defer group.Close()

	c.logger.Info("kafka log consumer started",
		zap.Strings("brokers", c.brokers),
		zap.String("topic", c.topic),
		zap.String("log_dir", c.logDir),
	)

	handler := &consumerHandler{consumer: c}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("kafka consumer shutting down")
			c.closeAllFiles()
			return nil
		default:
			if err := group.Consume(ctx, []string{c.topic}, handler); err != nil {
				c.logger.Error("consumer group error", zap.Error(err))
			}
		}
	}
}

// getFile returns an open file handle for the given date.
// Creates the directory if it doesn't exist.
func (c *LogConsumer) getFile(date string) (*os.File, error) {
	if f, ok := c.files[date]; ok {
		return f, nil
	}

	if !dirExists(c.logDir) {
		if err := os.MkdirAll(c.logDir, 0755); err != nil {
			return nil, fmt.Errorf("creating log dir %s: %w", c.logDir, err)
		}
		c.logger.Info("created log dir", zap.String("log_dir", c.logDir))
	}

	filename := fmt.Sprintf("log-%s.log", date)
	path := filepath.Join(c.logDir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file %s: %w", path, err)
	}

	c.logger.Info("opened log file", zap.String("file", path))
	c.files[date] = f
	return f, nil
}

func (c *LogConsumer) writeLog(date, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := c.getFile(date)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(f, message)
	return err
}

func (c *LogConsumer) closeAllFiles() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for date, f := range c.files {
		c.logger.Info("closing log file", zap.String("date", date))
		f.Close()
		delete(c.files, date)
	}
}

// ── Sarama ConsumerGroupHandler ───────────────────────────────────────

type consumerHandler struct {
	consumer *LogConsumer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		date := string(msg.Key)
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}

		if err := h.consumer.writeLog(date, string(msg.Value)); err != nil {
			h.consumer.logger.Error("writing log to file failed",
				zap.String("date", date),
				zap.Error(err),
			)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}
