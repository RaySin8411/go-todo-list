package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type M = map[string]string

var (
	todos    []Todo
	nextID   = 1
	dataFile = "todos.json"
)

func main() {
	// 1) 啟動時載入 JSON（若不存在就忽略錯誤）
	if err := loadFromJSON(dataFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
	recomputeNextID()

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

	// 整筆更新：PUT /todos/:id
	e.PUT("/todos/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "id must be a number"})
		}
		var req struct {
			Title string `json:"title"`
			Done  *bool  `json:"done"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
		}
		if strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "title cannot be empty"})
		}
		idx := findIndexByID(id)
		if idx == -1 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "todo not found"})
		}
		todos[idx].Title = req.Title
		if req.Done != nil {
			todos[idx].Done = *req.Done
		}
		return c.JSON(http.StatusOK, todos[idx])
	})

	// 部分更新（完成狀態）：PATCH /todos/:id/done
	e.PATCH("/todos/:id/done", func(c echo.Context) error {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "id must be a number"})
		}
		idx := findIndexByID(id)
		if idx == -1 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "todo not found"})
		}
		var req struct {
			Done *bool `json:"done"`
		}
		_ = c.Bind(&req)
		if req.Done == nil {
			// 切換模式
			todos[idx].Done = !todos[idx].Done
		} else {
			todos[idx].Done = *req.Done
		}
		return c.JSON(http.StatusOK, todos[idx])
	})

	// 刪除任務：DELETE /todos/:id
	e.DELETE("/todos/:id", func(c echo.Context) error {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, M{"error": "id must be a number"})
		}
		idx := findIndexByID(id)
		if idx == -1 {
			return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
		}
		// 移除 idx
		todos = append(todos[:idx], todos[idx+1:]...)
		if err := saveToJSON(dataFile); err != nil {
			return c.JSON(http.StatusInternalServerError, M{"error": "save failed"})
		}
		return c.NoContent(http.StatusNoContent) // 204，純刪除不用回 body
	})

	// 啟動伺服器，監聽在 http://localhost:1323
	e.Start(":1323")
}

/* ----------------- 工具區 ----------------- */

// 找到 id 的索引；找不到回 -1
func findIndexByID(id int) int {
	for i, t := range todos {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// 啟動後根據載入資料重算 nextID
func recomputeNextID() {
	maxID := 0
	for _, t := range todos {
		if t.ID > maxID {
			maxID = t.ID
		}
	}
	nextID = maxID + 1
}

// 讀檔：把 todos.json 載回 todos
func loadFromJSON(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	// 空檔案處理
	if len(strings.TrimSpace(string(b))) == 0 {
		todos = nil
		return nil
	}
	return json.Unmarshal(b, &todos)
}

// 寫檔（安全寫入）：先寫暫存檔，再 rename 成正式檔
func saveToJSON(path string) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, ".todos.tmp.json")

	b, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
