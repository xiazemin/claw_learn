package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

const (
	defaultConnStr = "postgres://pgvec:pgvec123@localhost:5434/pgvector_demo?sslmode=disable"
)

func getConnStr() string {
	if s := os.Getenv("PGVECTOR_CONN"); s != "" {
		return s
	}
	return defaultConnStr
}

func connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, getConnStr())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	// 先创建 pgvector 扩展，再注册类型
	_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("create extension: %w", err)
	}
	if err := pgxvec.RegisterTypes(ctx, conn); err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("register types: %w", err)
	}
	return conn, nil
}

func divider(title string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("=", 60))
}

func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
