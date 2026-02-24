package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	fmt.Printf("Tenant: %s (ID: %s)\n", tenant1.Name, tenant1.ID)

	// Seed users for tenant1
	adminUser, err := seedUser(ctx, queries, tenant1.ID, "admin", "Admin User", "admin@test.com", "password123", "")
	if err != nil {
		fmt.Printf("Admin user: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created admin user: %s\n", adminUser.FullName)
	}

	dispatcher, err := seedUser(ctx, queries, tenant1.ID, "dispatcher", "Test Dispatcher", "dispatcher@test.com", "password123", "")
	if err != nil {
		fmt.Printf("Dispatcher user: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created dispatcher: %s\n", dispatcher.FullName)
	}

	driver1, err := seedUser(ctx, queries, tenant1.ID, "driver", "Abebe Kebede", "", "", "0912345678")
	if err != nil {
		fmt.Printf("Driver 1: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created driver: %s (0912345678)\n", driver1.FullName)
	}

	driver2, err := seedUser(ctx, queries, tenant1.ID, "driver", "Kebede Bekele", "kebede@logistics.et", "", "0978765432")
	if err != nil {
		fmt.Printf("Driver 2: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created driver: %s (0978765432)\n", driver2.FullName)
	}

	// Seed vehicles for tenant1
	vehicle1, err := seedVehicle(ctx, queries, tenant1.ID, "ET-1234-AA", "pickup")
	if err != nil {
		fmt.Printf("Vehicle 1: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created vehicle: %s\n", vehicle1.PlateNumber)
	}

	vehicle2, err := seedVehicle(ctx, queries, tenant1.ID, "ET-5678-BB", "truck")
	if err != nil {
		fmt.Printf("Vehicle 2: %v (may already exist)\n", err)
	} else {
		fmt.Printf("Created vehicle: %s\n", vehicle2.PlateNumber)
	}

	// Get user IDs for shipments (need to fetch existing users if they exist)
	users, _ := queries.ListUsersByTenant(ctx, db.ListUsersByTenantParams{
		TenantID: tenant1.ID,
		Limit:    100,
		Offset:   0,
	})

	var dispatcherID, driver1ID, driver2ID pgtype.UUID
	for _, u := range users {
		if u.Role == "dispatcher" && !dispatcherID.Valid {
			dispatcherID = u.ID
		}
		if u.Role == "driver" {
			if u.Phone != nil && *u.Phone == "0912345678" {
				driver1ID = u.ID
			}
			if u.Phone != nil && *u.Phone == "0978765432" {
				driver2ID = u.ID
			}
		}
	}

	// Get vehicle IDs
	vehicles, _ := queries.ListActiveVehiclesByTenant(ctx, tenant1.ID)
	var vehicle1ID, vehicle2ID pgtype.UUID
	for _, v := range vehicles {
		if v.PlateNumber == "ET-1234-AA" {
			vehicle1ID = v.ID
		}
		if v.PlateNumber == "ET-5678-BB" {
			vehicle2ID = v.ID
		}
	}

	// Seed shipments for driver1
	now := time.Now()

	if driver1ID.Valid && vehicle1ID.Valid && dispatcherID.Valid {
		shipment1, err := seedShipment(ctx, queries, tenant1.ID, dispatcherID, driver1ID, vehicle1ID,
			"ETH-2024-001001",
			"Bole, Addis Ababa",
			"Merkato, Addis Ababa",
			"Abebe Bikila",
			"0911000001",
			"Electronics package",
			"assigned",
			now.Add(2*time.Hour),
		)
		if err != nil {
			fmt.Printf("Shipment 1: %v (may already exist)\n", err)
		} else {
			fmt.Printf("Created shipment: %s (status: %s)\n", shipment1.TrackingNumber, shipment1.Status)
		}

		shipment2, err := seedShipment(ctx, queries, tenant1.ID, dispatcherID, driver1ID, vehicle1ID,
			"ETH-2024-001002",
			"Piazza, Addis Ababa",
			"Bole International Airport",
			"Sara Tesfaye",
			"0911000002",
			"Documents - Urgent",
			"in_transit",
			now.Add(1*time.Hour),
		)
		if err != nil {
			fmt.Printf("Shipment 2: %v (may already exist)\n", err)
		} else {
			fmt.Printf("Created shipment: %s (status: %s)\n", shipment2.TrackingNumber, shipment2.Status)
		}

		shipment5, err := seedShipment(ctx, queries, tenant1.ID, dispatcherID, driver1ID, vehicle1ID,
			"ETH-2024-001005",
			"Summit, Addis Ababa",
			"Gullele, Addis Ababa",
			"Yohannes Tadesse",
			"0911000005",
			"Food supplies",
			"delivered",
			now.Add(-2*time.Hour),
		)
		if err != nil {
			fmt.Printf("Shipment 5: %v (may already exist)\n", err)
		} else {
			fmt.Printf("Created shipment: %s (status: %s)\n", shipment5.TrackingNumber, shipment5.Status)
		}
	}

	if driver2ID.Valid && vehicle2ID.Valid && dispatcherID.Valid {
		shipment4, err := seedShipment(ctx, queries, tenant1.ID, dispatcherID, driver2ID, vehicle2ID,
			"ETH-2024-001004",
			"Eastern Industrial Zone",
			"Mekanisa, Addis Ababa",
			"Tigist Alemayehu",
			"0911000004",
			"Construction materials",
			"assigned",
			now.Add(4*time.Hour),
		)
		if err != nil {
			fmt.Printf("Shipment 4: %v (may already exist)\n", err)
		} else {
			fmt.Printf("Created shipment: %s (status: %s)\n", shipment4.TrackingNumber, shipment4.Status)
		}
	}

	fmt.Println("\nDevelopment database seeded!")
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
		h, err := hash.Password(password)
		if err != nil {
			return nil, err
		}
		passwordHash = &h
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

func seedVehicle(ctx context.Context, queries *db.Queries, tenantID pgtype.UUID, plateNumber, vehicleType string) (*db.Vehicle, error) {
	isActive := true
	vehicle, err := queries.CreateVehicle(ctx, db.CreateVehicleParams{
		TenantID:    tenantID,
		PlateNumber: plateNumber,
		VehicleType: &vehicleType,
		IsActive:    &isActive,
	})
	if err != nil {
		// Try to get existing vehicle by plate number
		vehicles, err2 := queries.ListActiveVehiclesByTenant(ctx, tenantID)
		if err2 == nil {
			for _, v := range vehicles {
				if v.PlateNumber == plateNumber {
					return &v, nil
				}
			}
		}
		return nil, err
	}
	return &vehicle, nil
}

func seedShipment(ctx context.Context, queries *db.Queries, tenantID, createdBy, driverID, vehicleID pgtype.UUID, trackingNumber, origin, destination, customerName, customerPhone, cargoDescription, status string, estimatedDelivery time.Time) (*db.Shipment, error) {
	shipment, err := queries.CreateShipment(ctx, db.CreateShipmentParams{
		TenantID:           tenantID,
		TrackingNumber:     trackingNumber,
		OriginAddress:      origin,
		DestinationAddress: destination,
		CustomerName:       customerName,
		CustomerPhone:      customerPhone,
		CargoDescription:   toText(cargoDescription),
		DriverID:           driverID,
		VehicleID:          vehicleID,
		Status:             status,
		EstimatedDelivery:  pgtype.Timestamptz{Time: estimatedDelivery, Valid: true},
		CreatedBy:          createdBy,
	})
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func toText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
