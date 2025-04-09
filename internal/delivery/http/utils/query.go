package utils

import "github.com/labstack/echo/v4"

func ReadQuery(c echo.Context, v any) error {
	err := c.Bind(v)
	if err != nil {
		return err
	}
	return nil
}
