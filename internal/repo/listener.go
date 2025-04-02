package repo

type TelegramListener interface {
	// GetLastUpdate возвращает ID последнего обновления
	GetLastUpdate() (int, error)
	// SetLastUpdate устанавливает ID последнего обновления
	SetLastUpdate(int) error
}
