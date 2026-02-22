package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
)

func main() {
	// Handle graceful shutdown on Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, shutting down gracefully...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run scripts/seed.go <database_url>")
		fmt.Println("Example: go run scripts/seed.go postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable")
		os.Exit(1)
	}

	databaseURL := os.Args[1]

	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)

	fmt.Println("Seeding development database...")

	// Seed tenants
	tenant1, err := seedTenant(ctx, queries, "Test Logistics Company", "test-logistics", "starter", 5)
	if err != nil {
		fmt.Printf("Failed to seed tenant: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created tenant: %s (ID: %s)\n", tenant1.Name, tenant1.ID)

	tenant2, err := seedTenant(ctx, queries, "Demo Transport Ltd", "demo-transport", "business", 25)
	if err != nil {
		fmt.Printf("Failed to seed tenant: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created tenant: %s (ID: %s)\n", tenant2.Name, tenant2.ID)

	// Seed users for tenant1
	adminUser, err := seedUser(ctx, queries, tenant1.ID, "admin", "Admin User", "admin@test.com", "password123", "")
	if err != nil {
		fmt.Printf("Failed to seed admin: %v\n", err)
		os.Exit(1)
	}
	email := ""
	if adminUser.Email != nil {
		email = *adminUser.Email
	}
	fmt.Printf("Created admin user: %s (%s)\n", adminUser.FullName, email)

	dispatcher, err := seedUser(ctx, queries, tenant1.ID, "dispatcher", "Test Dispatcher", "dispatcher@test.com", "password123", "")
	if err != nil {
		fmt.Printf("Failed to seed dispatcher: %v\n", err)
		os.Exit(1)
	}
	email = ""
	if dispatcher.Email != nil {
		email = *dispatcher.Email
	}
	fmt.Printf("Created dispatcher: %s (%s)\n", dispatcher.FullName, email)

	driver1, err := seedUser(ctx, queries, tenant1.ID, "driver", "Abebe Kebede", "", "", "0912345678")
	if err != nil {
		fmt.Printf("Failed to seed driver: %v\n", err)
		os.Exit(1)
	}
	phone := ""
	if driver1.Phone != nil {
		phone = *driver1.Phone
	}
	fmt.Printf("Created driver: %s (%s)\n", driver1.FullName, phone)

	driver2, err := seedUser(ctx, queries, tenant1.ID, "driver", "Kebede Bekele", "kebede@logistics.et", "", "0978765432")
	if err != nil {
		fmt.Printf("Failed to seed driver: %v\n", err)
		os.Exit(1)
	}
	phone = ""
	if driver2.Phone != nil {
		phone = *driver2.Phone
	}
	fmt.Printf("Created driver: %s (%s)\n", driver2.FullName, phone)

	fmt.Println("\nDevelopment database seeded successfully!")
	fmt.Println("\nTest credentials:")
	fmt.Println("  Dispatcher: dispatcher@test.com / password123")
	fmt.Println("  Driver 1:   0912345678 / 1234")
	fmt.Println("  Driver 2:   0978765432 / 1234")

	return nil
}

func seedTenant(ctx context.Context, queries *db.Queries, name, slug, plan string, maxDrivers int32) (*db.Tenant, error) {
	tenant, err := queries.CreateTenant(ctx, db.CreateTenantParams{
		Name:       name,
		Slug:       slug,
		Plan:       plan,
		MaxDrivers: &maxDrivers,
	})
	if err != nil {
		// If already exists, try to get it
		t, err := queries.GetTenantBySlug(ctx, slug)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
	return &tenant, nil
}

func seedUser(ctx context.Context, queries *db.Queries, tenantID pgtype.UUID, role, fullName, email, password, phone string) (*db.User, error) {
	var passwordHash, pinHash *string

	if password != "" {
		hash, err := hash.Password(password)
		if err != nil {
			return nil, err
		}
		passwordHash = &hash
	}

	if phone != "" {
		pin, err := hash.PIN("1234")
		if err != nil {
			return nil, err
		}
		pinHash = &pin
	}

	isActive := true
	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     tenantID,
		Role:         role,
		FullName:     fullName,
		Email:        toText(email),
		Phone:        toText(phone),
		PasswordHash: passwordHash,
		PinHash:      pinHash,
		IsActive:     &isActive,
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func toText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
