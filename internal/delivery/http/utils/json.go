package utils

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
)

func ReadJSON(c echo.Context, v any) error {
	err := json.NewDecoder(c.Request().Body).Decode(v)
	if err != nil {
		return err
	}
	return nil
}
