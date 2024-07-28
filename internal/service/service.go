package service

import (
	"ProjectMessageService/config"
	"ProjectMessageService/internal/repository"
	"ProjectMessageService/internal/utils"
	"context"
	"strings"

	"github.com/segmentio/kafka-go"
)

type MessageService struct {
	repo        *repository.Repository
	kafkaWriter *kafka.Writer
	kafkaReader *kafka.Reader
	app         *config.Application
}

func NewMessageService(repo *repository.Repository, kafkaWriter *kafka.Writer, app *config.Application) *MessageService {
	return &MessageService{repo: repo, kafkaWriter: kafkaWriter, app: app}
}

func (s *MessageService) SaveMessage(ctx context.Context, message utils.Message) error {
	if err := s.repo.SaveMessage(ctx, message); err != nil {
		s.app.Log.Error("Ошибка сохранения сообщения:", err)
		return err
	}

	topic, _ := getTopicAndGroup(message.Topic)

	err := s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(message.Topic),
		Value: []byte(message.Message),
	})

	return err
}

func (s *MessageService) GetStats(ctx context.Context, topic utils.Messages) (int, error) {
	return s.repo.GetProcessedMessagesCount(ctx, topic)
}

func NewKafkaWriter(cfg config.Config) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaURL),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll, // Подтверждение от всех реплик
	}
}

func (s *MessageService) ConsumeMessages(ctx context.Context, cfg config.Config, messageTypes []string) {
	var message utils.Message
	for _, messageType := range messageTypes {
		go func(messageType string) {
			reader := NewKafkaReader(cfg, messageType)
			defer func(reader *kafka.Reader) {
				_ = reader.Close()
			}(reader)

			for {
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					s.app.Log.Errorf("Не удалось прочитать сообщение: %v", err)
					continue
				}
				parts := strings.Split(msg.Topic, "-")
				message.Topic = parts[0]
				message.Message = string(msg.Value)

				s.app.Log.Infof("msg.Topic %v, msg.Headers %v, msg.Partition %v, msg.Offset %v\n", msg.Topic, msg.Headers, msg.Partition, msg.Offset)

				// Обработка сообщения
				err = s.processMessage(ctx, message)
				if err != nil {
					s.app.Log.Errorf("Ошибка: %v", err)
					return
				}
			}
		}(messageType)
	}
}

// getTopicAndGroup для определения топика и группы.
func getTopicAndGroup(messageType string) (string, string) {
	return messageType + "-topic", messageType + "-group"
}

func NewKafkaReader(cfg config.Config, messageType string) *kafka.Reader {
	topic, group := getTopicAndGroup(messageType)

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.KafkaURL},
		Topic:   topic,
		GroupID: group,
	})
}

func (s *MessageService) processMessage(ctx context.Context, msg utils.Message) error {
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		s.app.Log.Errorf("Ошибка сохранения сообщения: %v", err)
		return err
	}

	// Обработка сообщения
	key, err := s.repo.ContentMessagesKey(context.Background(), msg)
	if err != nil {
		s.app.Log.Errorf("Failed to select messages key: %v", err)
		return err
	}

	// Обновление состояния сообщения в базе данных
	err = s.repo.MarkMessageAsProcessed(context.Background(), msg, key)
	if err != nil {
		s.app.Log.Errorf("Failed to mark message as processed: %v", err)
		return err
	}

	s.app.Log.Infof("Processing message: %s Message written to topic %s", msg.Message, msg.Topic)
	// Дополнительная логика обработки
	return nil
}
