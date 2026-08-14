package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/pgstore"
)

func cmdIdentity(args []string) int {
	if len(args) == 0 || args[0] != "create" {
		fmt.Fprintln(os.Stderr, "usage: hr identity create --name <display name> --departments <a,b,c|all>")
		return 2
	}
	return cmdIdentityCreate(args[1:])
}

func cmdIdentityCreate(args []string) int {
	name, ok := flagValue(args, "name")
	if !ok || name == "" {
		fmt.Fprintln(os.Stderr, "--name is required")
		return 2
	}
	// Required, because an omitted scope used to mean "every department" -
	// forgetting the flag silently handed over the whole system. `all` is
	// still available, it just has to be said.
	raw, ok := flagValue(args, "departments")
	if !ok || strings.TrimSpace(raw) == "" {
		fmt.Fprintln(os.Stderr, "--departments is required: a comma-separated list, or `all` for an unscoped identity")
		return 2
	}
	var departments []string
	for _, d := range strings.Split(raw, ",") {
		d = strings.TrimSpace(d)
		switch {
		case d == "":
		case strings.EqualFold(d, "all"), d == pgstore.UnscopedDepartment:
			departments = []string{pgstore.UnscopedDepartment}
		default:
			departments = append(departments, d)
		}
		if len(departments) == 1 && departments[0] == pgstore.UnscopedDepartment {
			break
		}
	}
	if len(departments) == 0 {
		fmt.Fprintln(os.Stderr, "--departments listed no departments")
		return 2
	}

	dsn := os.Getenv("HR_SERVER_DB")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "HR_SERVER_DB is required (a Postgres connection string)")
		return 2
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer pool.Close()
	if err := pgstore.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
	spiffeID := "spiffe://hr.local/" + slug
	token, err := pgstore.CreateIdentity(ctx, pool, spiffeID, name, departments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	scope := strings.Join(departments, ", ")
	if len(departments) == 1 && departments[0] == pgstore.UnscopedDepartment {
		scope = "every department (unscoped)"
	}
	fmt.Printf("created identity %s\n  may act on: %s\n\n", spiffeID, scope)
	fmt.Printf("token (save this now - it cannot be shown again):\n%s\n\n", token)
	fmt.Println("Set HR_TOKEN to this value for `hr worker`/`hr watchdog`, or paste it into the web UI when prompted.")
	return 0
}
