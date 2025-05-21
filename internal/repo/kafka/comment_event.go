package kafka

import (
	"context"
	"errors"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"postic-backend/internal/entity"
	"postic-backend/internal/repo"
	"time"

	"github.com/segmentio/kafka-go"
	"net"
	"strconv"
)

const (
	NumPartitions = 3
)

// TopicConfig содержит настройки для создания топика
type TopicConfig struct {
	NumPartitions     int
	ReplicationFactor int
}

type CommentEventKafkaRepository struct {
	writer        *kafka.Writer
	readerFactory func(teamID int, postID int) *kafka.Reader
	brokers       []string
	topicConfig   TopicConfig
}

// createTopicIfNotExists создает топик, если он не существует
func createTopicIfNotExists(ctx context.Context, brokers []string, topic string, config TopicConfig) error {
	// Подключаемся к любому из брокеров
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	// Проверяем, существует ли уже топик
	topicExists, err := checkIfTopicExists(conn, topic)
	if err != nil {
		return err
	}

	// Если топик существует, возвращаем успешный результат
	if topicExists {
		return nil
	}

	// Создаем топик
	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer func() { _ = controllerConn.Close() }()

	return controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     config.NumPartitions,
		ReplicationFactor: config.ReplicationFactor,
	})
}

// checkIfTopicExists проверяет, существует ли топик
func checkIfTopicExists(conn *kafka.Conn, topic string) (bool, error) {
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			return false, nil
		}
		return false, err
	}
	return len(partitions) > 0, nil
}

// getMaxReplicationFactor определяет максимально возможный фактор репликации
// на основе количества доступных брокеров
func getMaxReplicationFactor(ctx context.Context, brokers []string, desiredFactor int) (int, error) {
	// Базовая проверка
	if len(brokers) == 0 {
		return 1, errors.New("пустой список брокеров")
	}

	// Подключаемся к любому из брокеров с явным таймаутом
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(dialCtx, "tcp", brokers[0])
	if err != nil {
		// В случае ошибки подключения, просто используем длину списка брокеров
		// как консервативную оценку, но логируем ошибку
		actualFactor := min(len(brokers), desiredFactor)
		return actualFactor, fmt.Errorf("не удалось подключиться к брокеру для получения метаданных, используем безопасное значение %d: %w", actualFactor, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Устанавливаем таймаут операции чтения метаданных
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		actualFactor := min(len(brokers), desiredFactor)
		return actualFactor, fmt.Errorf("ошибка установки таймаута чтения: %w", err)
	}

	// Пробуем получить информацию о всех брокерах
	brokerMetadata, err := conn.Brokers()
	if err != nil {
		// В случае ошибки используем длину списка полученных брокеров как запасной вариант
		actualFactor := min(len(brokers), desiredFactor)
		return actualFactor, fmt.Errorf("ошибка получения метаданных о брокерах, используем безопасное значение %d: %w", actualFactor, err)
	}

	// Количество доступных брокеров
	availableBrokers := len(brokerMetadata)
	if availableBrokers == 0 {
		// Если по какой-то причине метаданные пусты, используем предоставленный список
		actualFactor := min(len(brokers), desiredFactor)
		return actualFactor, fmt.Errorf("получен пустой список брокеров из метаданных, используем безопасное значение %d", actualFactor)
	}

	// Не можем реплицировать больше, чем у нас есть брокеров
	return min(availableBrokers, desiredFactor), nil
}

func NewCommentEventKafkaRepository(brokers []string) (repo.CommentEventRepository, error) {
	// Проверка подключения к Kafka
	if len(brokers) == 0 {
		return nil, errors.New("не предоставлены брокеры Kafka")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Желаемые значения для конфигурации топиков
	desiredReplicationFactor := 3 // В идеале хотим 3 для надежности

	// Определяем реально возможный фактор репликации
	actualReplicationFactor, err := getMaxReplicationFactor(ctx, brokers, desiredReplicationFactor)
	if err != nil {
		return nil, fmt.Errorf("ошибка при определении фактора репликации: %w", err)
	}

	// Значения по умолчанию для топиков с учетом реальной возможности репликации
	topicConfig := TopicConfig{
		NumPartitions:     NumPartitions,
		ReplicationFactor: actualReplicationFactor,
	}

	// Создаем базовый топик, если он не существует
	baseTopicName := "comment-events"
	if err := createTopicIfNotExists(ctx, brokers, baseTopicName, topicConfig); err != nil {
		return nil, fmt.Errorf("ошибка при создании базового топика: %w", err)
	}
	return &CommentEventKafkaRepository{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    baseTopicName, // по умолчанию, можно менять по команде
			Balancer: &kafka.LeastBytes{},
		},
		readerFactory: func(teamID int, postID int) *kafka.Reader {
			topic := fmt.Sprintf("comment-events-team-%d", teamID)

			// Всегда читаем только новые сообщения
			startOffset := kafka.LastOffset

			// Создаем GroupID с добавлением уникального идентификатора на основе времени
			// Это гарантирует, что каждое новое подключение будет получать только новые сообщения
			groupID := fmt.Sprintf("comment-listener-%d-%d-%d", teamID, postID, time.Now().UnixNano())

			return kafka.NewReader(kafka.ReaderConfig{
				Brokers:     brokers,
				Topic:       topic,
				GroupID:     groupID,
				MinBytes:    1,
				MaxBytes:    10e6,
				StartOffset: startOffset,
			})
		},
		brokers:     brokers,
		topicConfig: topicConfig,
	}, nil
}

func (r *CommentEventKafkaRepository) PublishCommentEvent(ctx context.Context, event *entity.CommentEvent) error {
	// Определяем топик по teamID
	topic := fmt.Sprintf("comment-events-team-%d", event.TeamID)

	// Проверяем и создаем топик, если он не существует
	if err := createTopicIfNotExists(ctx, r.brokers, topic, r.topicConfig); err != nil {
		return fmt.Errorf("ошибка при создании топика для команды %d: %w", event.TeamID, err)
	}

	// сериализация события
	b, err := msgpack.Marshal(event)
	if err != nil {
		return err
	}

	// устанавливаем топик для записи
	r.writer.Topic = topic

	return r.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", event.PostID)),
		Value: b,
	})
}

func (r *CommentEventKafkaRepository) SubscribeCommentEvents(ctx context.Context, teamID int, postID int) (<-chan *entity.CommentEvent, error) {
	// Определяем топик
	topic := fmt.Sprintf("comment-events-team-%d", teamID)

	// Проверяем и создаем топик, если он не существует
	if err := createTopicIfNotExists(ctx, r.brokers, topic, r.topicConfig); err != nil {
		return nil, fmt.Errorf("ошибка при создании топика для команды %d: %w", teamID, err)
	}

	reader := r.readerFactory(teamID, postID)
	ch := make(chan *entity.CommentEvent)
	go func() {
		defer close(ch)
		for {
			m, err := reader.ReadMessage(ctx)
			if err != nil {
				return
			}
			var event entity.CommentEvent
			if err := msgpack.Unmarshal(m.Value, &event); err == nil {
				if postID == 0 || event.PostID == postID {
					ch <- &event
				}
			}
		}
	}()
	return ch, nil
}
