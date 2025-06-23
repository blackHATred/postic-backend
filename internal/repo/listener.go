package repo

import "time"

type TelegramListener interface {
	// GetLastUpdate возвращает ID последнего обновления
	GetLastUpdate() (int, error)
	// SetLastUpdate устанавливает ID последнего обновления
	SetLastUpdate(int) error
}

type VkontakteListener interface {
	// GetUnwatchedGroups возвращает ID команд, у которых VK не отслеживался в течение последнего duration времени
	GetUnwatchedGroups(duration time.Duration) ([]int, error)
	// UpdateGroupLastUpdate обновляет время последнего обновления группы до текущего
	UpdateGroupLastUpdate(teamID int) error
	// GetLastEventTS возвращает timestamp последнего обработанного события для группы
	GetLastEventTS(teamID int) (string, error)
	// SetLastEventTS устанавливает timestamp последнего обработанного события для группы
	SetLastEventTS(teamID int, ts string) error
}
