package http

import (
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"postic-backend/internal/delivery/http/utils"
	"postic-backend/internal/entity"
	"postic-backend/internal/usecase"
	"strconv"
)

type Upload struct {
	uploadUseCase usecase.Upload
	userUseCase   usecase.User
	authManager   utils.Auth
}

func NewUpload(uploadUseCase usecase.Upload, userUseCase usecase.User, authManager utils.Auth) *Upload {
	return &Upload{
		uploadUseCase: uploadUseCase,
		userUseCase:   userUseCase,
		authManager:   authManager,
	}
}

func (u *Upload) Configure(server *echo.Group) {
	server.POST("/", u.Upload)
	server.GET("/get/:id", u.GetFile)
}

func (u *Upload) Upload(c echo.Context) error {
	userID, err := u.authManager.CheckAuthFromContext(c)
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
	if fileType == "" || (fileType != "photo" && fileType != "video") {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный тип файла. Допустимые типы: photo, video",
		})
	}

	// Читаем байты из файла и сохраняем
	fileBytes, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка чтения файла: " + err.Error(),
		})
	}
	defer func() { _ = fileBytes.Close() }()

	upload := &entity.Upload{
		UserID:   userID,
		FilePath: file.Filename,
		FileType: fileType,
		RawBytes: fileBytes,
	}

	fileID, err := u.uploadUseCase.UploadFile(upload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка сохранения файла: " + err.Error(),
		})
	}

	// Возвращаем айди файла
	return c.JSON(http.StatusOK, echo.Map{
		"file_id": fileID,
	})
}

func (u *Upload) GetFile(c echo.Context) error {
	userID, err := u.authManager.CheckAuthFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "Пользователь не авторизован",
		})
	}
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Неверный формат id файла",
		})
	}

	file, err := u.uploadUseCase.GetUpload(fileID, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка получения файла: " + err.Error(),
		})
	}
	fileBytes, err := io.ReadAll(file.RawBytes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Ошибка чтения файла: " + err.Error(),
		})
	}
	return c.Blob(http.StatusOK, http.DetectContentType(fileBytes), fileBytes)
}
