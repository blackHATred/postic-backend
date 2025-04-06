package retry

import (
	"github.com/labstack/gommon/log"
	"time"
)

const (
	maxRetries        = 6
	retryMultiplier   = 2
	retryInitialDelay = time.Millisecond * 100
	// При maxRetries = 6, retryMultiplier = 2, retryInitialDelay = 100ms:
	// 0-ая попытка: 0ms
	// 1-ая попытка: 100ms
	// 2-ая попытка: 200ms
	// 3-я попытка: 400ms
	// 4-ая попытка: 800ms
	// 5-ая попытка: 1600ms
	// 6-ая попытка: 3200ms, потом завершение
)

// Retry выполняет операцию с экспоненциальной задержкой между попытками.
// Возвращает nil, если операция успешна, или последнюю ошибку, если все попытки завершились неудачей.
func Retry(operation func() error) error {
	retryCounter := 0
	for {
		err := operation()
		if err == nil {
			return nil
		}
		if retryCounter >= maxRetries {
			return err
		}
		log.Errorf("error during retry %d: %v", retryCounter, err)
		time.Sleep(retryInitialDelay * time.Duration(retryCounter*retryMultiplier))
		retryCounter++
	}
}
