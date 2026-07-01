package main

import (
	"context"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type Todo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Completed bool   `json:"completed"`
	CreatedAt int64  `json:"createdAt"`
}

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	todos  []Todo
	filePath string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	var dir string
	u, err := user.Current()
	if err != nil {
		dir = "."
	} else {
		dir = filepath.Join(u.HomeDir, ".todos-desktop")
	}
	os.MkdirAll(dir, 0755)
	a.filePath = filepath.Join(dir, "todos.json")

	a.load()
}

func (a *App) load() {
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		a.todos = []Todo{}
		return
	}
	json.Unmarshal(data, &a.todos)
}

func (a *App) save() {
	data, _ := json.MarshalIndent(a.todos, "", "  ")
	os.WriteFile(a.filePath, data, 0644)
}

func (a *App) GetAll() []Todo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.todos
}

func (a *App) Add(title string, category string) Todo {
	a.mu.Lock()
	defer a.mu.Unlock()

	todo := Todo{
		ID:        uuid.New().String(),
		Title:     title,
		Category:  category,
		Completed: false,
		CreatedAt: int64(0),
	}
	a.todos = append([]Todo{todo}, a.todos...)
	a.save()
	return todo
}

func (a *App) Toggle(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.todos {
		if a.todos[i].ID == id {
			a.todos[i].Completed = !a.todos[i].Completed
			break
		}
	}
	a.save()
}

func (a *App) Remove(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	filtered := make([]Todo, 0, len(a.todos))
	for _, t := range a.todos {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	a.todos = filtered
	a.save()
}
