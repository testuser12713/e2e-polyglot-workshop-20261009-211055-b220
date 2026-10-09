package auth

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"workshop/api/internal/store"
)

// SeedFirstEmployee creates the first workshop employee from configuration if
// the employees table is still empty. It runs once: when any employee already
// exists it is a no-op, so a rotating SEED_EMPLOYEE_PASSWORD never overwrites an
// account and a restart never changes stored credentials (AC-15).
//
// The password is read lazily from SEED_EMPLOYEE_PASSWORD and stored only as a
// bcrypt hash. Neither the clear-text password nor the hash is ever logged.
func SeedFirstEmployee(ctx context.Context, st *store.Store) error {
	exists, err := st.EmployeesExist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	email := strings.TrimSpace(os.Getenv("SEED_EMPLOYEE_EMAIL"))
	password := os.Getenv("SEED_EMPLOYEE_PASSWORD")
	if email == "" || password == "" {
		log.Printf("auth: first-employee seed skipped: SEED_EMPLOYEE_EMAIL and SEED_EMPLOYEE_PASSWORD must both be set (see RUN.json)")
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	if _, err := st.CreateEmployee(ctx, email, employeeDisplayName(email), string(hash)); err != nil {
		return err
	}
	log.Printf("auth: created first employee %q", email)
	return nil
}

// employeeDisplayName derives the display name of the seeded account from the
// local part of its e-mail address.
func employeeDisplayName(email string) string {
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at]
	}
	return email
}
