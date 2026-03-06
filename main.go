package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
	// 可選：CreatedAt time.Time `json:"created_at"`
}

type M = map[string]string

func main() {
	// 1) 讀取 DATABASE_URL
	dsn := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		panic("DATABASE_URL is empty. Please set DATABASE_URL environment variable.")
	}

	// 2) 建連線池
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// 3) 建表（如果不存在）
	if _, err := pool.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS todos (
				id SERIAL PRIMARY KEY,
				title TEXT NOT NULL,
				done BOOLEAN NOT NULL DEFAULT FALSE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)`); err != nil {
		panic(err)
	}

	// 建立 Echo 實例（網站伺服器）
	e := echo.New()

	// 路由：GET /hello
	e.GET("/hello", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, TodoList with Postgres!")
	})

	// 新增任務：POST /todos
	e.POST("/todos", func(c echo.Context) error {
		var req struct {
			Title string `json:"title"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, M{"error": "invalid request"})
		}

		var t Todo
		row := pool.QueryRow(ctx,
			`INSERT INTO todos (title) VALUES ($1) RETURNING id, title, done`,
			req.Title,
		)
		if err := row.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return c.JSON(http.StatusInternalServerError, M{"error": "db insert failed"})
		}
		return c.JSON(http.StatusOK, t)
	})

	// 列出全部：GET /todos
	e.GET("/todos", func(c echo.Context) error {
		rows, err := pool.Query(ctx, `SELECT id, title, done FROM todos ORDER BY id`)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, M{"error": "db query failed"})
		}
		defer rows.Close()

		var out []Todo
		for rows.Next() {
			var t Todo
			if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
				return c.JSON(http.StatusInternalServerError, M{"error": "scan failed"})
			}
			out = append(out, t)
		}
		return c.JSON(http.StatusOK, out)
	})

	// 查單一：GET /todos/:id
	e.GET("/todos/:id", func(c echo.Context) error {
		id, bad := parseID(c)
		if bad != nil {
			return bad
		}
		var t Todo
		row := pool.QueryRow(ctx, `SELECT id, title, done FROM todos WHERE id=$1`, id)
		if err := row.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
		}
		return c.JSON(http.StatusOK, t)
	})

	// 整筆更新：PUT /todos/:id（改 title / done）
	e.PUT("/todos/:id", func(c echo.Context) error {
		id, bad := parseID(c)
		if bad != nil {
			return bad
		}
		var req struct {
			Title string `json:"title"`
			Done  *bool  `json:"done"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, M{"error": "invalid json"})
		}
		if strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, M{"error": "title cannot be empty"})
		}

		var currentDone bool
		if err := pool.QueryRow(ctx, `SELECT done FROM todos WHERE id=$1`, id).Scan(&currentDone); err != nil {
			return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
		}
		newDone := currentDone
		if req.Done != nil {
			newDone = *req.Done
		}

		var t Todo
		row := pool.QueryRow(ctx,
			`UPDATE todos SET title=$1, done=$2 WHERE id=$3 RETURNING id, title, done`,
			req.Title, newDone, id,
		)
		if err := row.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return c.JSON(http.StatusInternalServerError, M{"error": "db update failed"})
		}
		return c.JSON(http.StatusOK, t)
	})

	// 部分更新（完成狀態）：PATCH /todos/:id/done
	e.PATCH("/todos/:id/done", func(c echo.Context) error {
		id, bad := parseID(c)
		if bad != nil {
			return bad
		}
		var req struct {
			Done *bool `json:"done"`
		}
		_ = c.Bind(&req)

		if req.Done == nil {
			var t Todo
			row := pool.QueryRow(ctx,
				`UPDATE todos SET done = NOT done WHERE id=$1 RETURNING id, title, done`,
				id,
			)
			if err := row.Scan(&t.ID, &t.Title, &t.Done); err != nil {
				return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
			}
			return c.JSON(http.StatusOK, t)
		}

		var t Todo
		row := pool.QueryRow(ctx,
			`UPDATE todos SET done=$1 WHERE id=$2 RETURNING id, title, done`,
			*req.Done, id,
		)
		if err := row.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
		}
		return c.JSON(http.StatusOK, t)
	})

	// 刪除：DELETE /todos/:id
	e.DELETE("/todos/:id", func(c echo.Context) error {
		id, bad := parseID(c)
		if bad != nil {
			return bad
		}
		ct, err := pool.Exec(ctx, `DELETE FROM todos WHERE id=$1`, id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, M{"error": "db delete failed"})
		}
		if ct.RowsAffected() == 0 {
			return c.JSON(http.StatusNotFound, M{"error": "todo not found"})
		}
		return c.NoContent(http.StatusNoContent)
	})

	// 啟動伺服器，監聽在 http://localhost:1323
	e.Start(":1323")
}

/* ----------------- 工具區 ----------------- */

func parseID(c echo.Context) (int, error) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, c.JSON(http.StatusBadRequest, M{"error": "id must be a number"})
	}
	return id, nil
}

// （可選）單純示範用：若你想拿 created_at 也行
var _ = time.Now
