package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	// 建立 Echo 實例（網站伺服器）
	e := echo.New()

	// 路由：GET /hello
	e.GET("/hello", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, TodoList!")
	})

	// 啟動伺服器，監聽在 http://localhost:1323
	e.Start(":1323")
}
