package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gptscript-ai/cmd"
	"github.com/spf13/cobra"
)

type Migrations struct {
	DNS            string `usage:"Database URL" default:"sqlite://gptscript-threads.db"`
	Steps          int    `usage:"Number of migrations to apply, negative for down, zero means all the way up"`
	FailOnNoChange bool   `usage:"Fail if there are no migrations to apply"`
}

func (m *Migrations) Run(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return err
	}

	if err := m.run(); m.FailOnNoChange || !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func (m *Migrations) run() error {
	databaseDriver, _, _ := strings.Cut(m.DNS, ":")
	migrator, err := migrate.New(fmt.Sprintf("file://migrations/%s", databaseDriver), m.DNS)
	if err != nil {
		return err
	}

	if m.Steps != 0 {
		return migrator.Steps(m.Steps)
	}

	return migrator.Up()
}

func main() {
	cmd.MainCtx(context.Background(), cmd.Command(new(Migrations)))
}
