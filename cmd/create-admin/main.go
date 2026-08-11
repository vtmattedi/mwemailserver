package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/term"

	"github.com/vtmattedi/mw-email-server/internal/admin"
	"github.com/vtmattedi/mw-email-server/internal/store/sqlite"
)

func main() {
	_ = godotenv.Load()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Admin email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		fail("read email", err)
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fail("read password", err)
	}
	defer zero(passwordBytes)

	fmt.Print("Confirm password: ")
	confirmBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fail("read password confirmation", err)
	}
	defer zero(confirmBytes)

	if string(passwordBytes) != string(confirmBytes) {
		fmt.Fprintln(os.Stderr, "ERROR: passwords do not match")
		os.Exit(1)
	}

	databasePath := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if databasePath == "" {
		databasePath = "./data/gateway.db"
	}

	db, err := sqlite.Open(databasePath)
	if err != nil {
		fail("open database", err)
	}
	defer db.Close()

	service := admin.NewService(db, admin.DefaultSessionTTL)

	user, err := service.CreateAdmin(
		context.Background(),
		strings.TrimSpace(email),
		string(passwordBytes),
	)
	if err != nil {
		fail("create admin", err)
	}

	fmt.Println()
	fmt.Println("Admin created.")
	fmt.Printf("ID: %d\n", user.ID)
	fmt.Printf("Email: %s\n", user.Email)
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", action, err)
	os.Exit(1)
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
