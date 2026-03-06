package main

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

var todos []Todo
var nextID = 1

func main() {
	// 建立 Echo 實例（網站伺服器）
	e := echo.New()

	// 路由：GET /hello
	e.GET("/hello", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, TodoList!")
	})

	// 新增任務：POST /todos
	e.POST("/todos", func(c echo.Context) error {
		var newTodo Todo
		if err := c.Bind(&newTodo); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request",
			})
		}

		newTodo.ID = nextID
		newTodo.Done = false
		nextID++

		todos = append(todos, newTodo)

		return c.JSON(http.StatusOK, newTodo)
	})

	// 列出全部任務：GET /todos
	e.GET("/todos", func(c echo.Context) error {
		return c.JSON(http.StatusOK, todos)
	})

	// 查單一任務：GET /todos/:id
	e.GET("/todos/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "id must be a number",
			})
		}

		for _, t := range todos {
			if t.ID == id {
				return c.JSON(http.StatusOK, t)
			}
		}
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "todo not found",
		})
	})

	// 啟動伺服器，監聽在 http://localhost:1323
	e.Start(":1323")
}
