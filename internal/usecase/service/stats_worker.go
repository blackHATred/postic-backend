package service

import (
	"context"
	"postic-backend/internal/usecase"
	"time"

	"github.com/labstack/gommon/log"
)

type StatsWorker struct {
	analytics            usecase.Analytics
	workerID             string
	workerUpdateInterval time.Duration
}

func NewStatsWorker(analytics usecase.Analytics, workerID string, workerUpdateInterval time.Duration) *StatsWorker {
	return &StatsWorker{
		analytics:            analytics,
		workerID:             workerID,
		workerUpdateInterval: workerUpdateInterval,
	}
}

func (w *StatsWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.workerUpdateInterval)
	defer ticker.Stop()

	log.Infof("Запущен воркер обновления статистики: %s", w.workerID)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Остановка воркера обновления статистики: %s", w.workerID)
			return
		case <-ticker.C:
			if err := w.analytics.ProcessStatsUpdateTasks(w.workerID); err != nil {
				log.Errorf("Ошибка обработки задач обновления статистики: %v", err)
			}
		}
	}
}
