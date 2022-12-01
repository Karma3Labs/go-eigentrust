package basic

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func NewRouter() http.Handler {
	r := echo.New()
	return r
}
