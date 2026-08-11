package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/store/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: go run ./scripts/create-project.go "Project Name"`)
		os.Exit(2)
	}

	_ = godotenv.Load()

	name := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, "project name is required")
		os.Exit(2)
	}

	databasePath := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if databasePath == "" {
		databasePath = "./data/gateway.db"
	}

	db, err := sqlite.Open(databasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open database:", err)
		os.Exit(1)
	}
	defer db.Close()

	token, err := auth.GenerateToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate token:", err)
		os.Exit(1)
	}

	id, err := db.CreateProject(
		context.Background(),
		name,
		auth.HashToken(token),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create project:", err)
		os.Exit(1)
	}

	fmt.Println("Project created.")
	fmt.Println()
	fmt.Printf("ID: %d\n", id)
	fmt.Printf("Name: %s\n", name)
	fmt.Println()
	fmt.Println("API key:")
	fmt.Println(token)
	fmt.Println()
	fmt.Println("Save this token now. Only its SHA-256 hash is stored.")
}
