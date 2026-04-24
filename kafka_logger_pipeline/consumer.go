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
	return &LogConsumer{
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
		logDir:  logDir,
		logger:  logger,
		files:   make(map[string]*os.File),
	}
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
func (c *LogConsumer) getFile(date string) (*os.File, error) {
	if f, ok := c.files[date]; ok {
		return f, nil
	}

	if err := os.MkdirAll(c.logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log dir: %w", err)
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

type consumerHandler struct {
	consumer *LogConsumer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// Key is the date set by producer
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
