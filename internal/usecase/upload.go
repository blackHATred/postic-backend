package usecase

type Upload interface {
	// SaveFile сохраняет файл в папку и возвращает его айди
	SaveFile(filename string, contentBytes []byte, fileType string, userID int) (int, error)
}
