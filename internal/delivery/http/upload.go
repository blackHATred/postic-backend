package http

import (
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/usecase"
)

type Upload struct {
	uploadUseCase  usecase.Upload
	userUseCase    usecase.User
	cookiesManager *utils.CookieManager
}

func NewUpload(uploadUseCase usecase.Upload, userUseCase usecase.User, cookiesManager *utils.CookieManager) *Upload {
	return &Upload{
		uploadUseCase:  uploadUseCase,
		userUseCase:    userUseCase,
		cookiesManager: cookiesManager,
	}
}

func (u *Upload) Configure(server *echo.Group) {
	server.POST("/upload", u.Upload)
}

func (u *Upload) Upload(c echo.Context) error {
	// Извлекаем из куки айди пользователя
	userID, err := u.cookiesManager.GetUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}

	// Извлекаем файл
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Файл не найден: " + err.Error(),
		})
	}

	// Извлекаем пометку, с которой загрузили файл (photo/video/raw)
	fileType := c.FormValue("type")
	if fileType == "" || (fileType != "photo" && fileType != "video" && fileType != "raw") {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный тип файла. Допустимые типы: photo, video, raw",
		})
	}

	// Сохраняем файл в папку
	content, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка при открытии файла: " + err.Error(),
		})
	}
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка при чтении файла: " + err.Error(),
		})
	}
	filename := file.Filename
	fileID, err := u.uploadUseCase.SaveFile(filename, contentBytes, fileType, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка при сохранении файла: " + err.Error(),
		})
	}

	// Возвращаем айди файла
	return c.JSON(http.StatusOK, echo.Map{
		"file_id": fileID,
	})
}
