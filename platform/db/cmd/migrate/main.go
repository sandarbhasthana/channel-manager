// Command migrate is the channel-manager migration runner.
//
// It walks ./migrations/<schema>/ for every schema declared in
// db.Schemas and applies / rolls back migrations in dependency order.
//
//	migrate up                  # apply all pending migrations
//	migrate up   -schema audit  # apply only the audit schema
//	migrate down                # roll back every schema (reverse order)
//	migrate down -schema audit  # roll back only the audit schema
//	migrate version             # print current version per schema
//	migrate force -schema audit -version 1
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/channel-manager/channel-manager/platform/config"
	"github.com/channel-manager/channel-manager/platform/db"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "up":
		runUp(args)
	case "down":
		runDown(args)
	case "version":
		runVersion(args)
	case "force":
		runForce(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "migrate: unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: migrate <command> [flags]

commands:
  up                       apply all pending migrations (or one schema)
  down                     roll back all migrations (or one schema)
  version                  print current version of each schema
  force -schema X -version N
                           force schema X to version N (recovery only)

common flags:
  -schema <name>           limit the operation to a single schema
  -migrations <dir>        migrations root (default: ./migrations)
`)
}

func loadOptions(schema string, migrationsRoot string) db.MigrateOptions {
	cfg, err := config.Load()
	if err != nil {
		die("load config: %v", err)
	}
	dbCfg := db.Config{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		DBName:   cfg.DB.DBName,
		User:     cfg.DB.User,
		Password: cfg.DB.Password,
		SSLMode:  cfg.DB.SSLMode,
	}
	if migrationsRoot == "" {
		migrationsRoot = defaultMigrationsRoot()
	}
	abs, err := filepath.Abs(migrationsRoot)
	if err != nil {
		die("resolve migrations root: %v", err)
	}
	return db.MigrateOptions{
		MigrationsRoot: abs,
		Schema:         schema,
		DatabaseURL:    dbCfg.URL(),
	}
}

func defaultMigrationsRoot() string {
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		return v
	}
	return "migrations"
}

func parseFlags(args []string) (schema, root string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	fs.StringVar(&schema, "schema", "", "limit to a single schema")
	fs.StringVar(&root, "migrations", "", "migrations root directory")
	_ = fs.Parse(args)
	return schema, root
}

func runUp(args []string) {
	schema, root := parseFlags(args)
	for _, s := range targetSchemas(schema, false) {
		opts := loadOptions(s, root)
		fmt.Printf("→ migrating up   %-12s\n", s)
		if err := db.Up(opts); err != nil {
			die("up %s: %v", s, err)
		}
	}
	fmt.Println("✓ all migrations applied")
}

func runDown(args []string) {
	schema, root := parseFlags(args)
	for _, s := range targetSchemas(schema, true) {
		opts := loadOptions(s, root)
		fmt.Printf("→ migrating down %-12s\n", s)
		if err := db.Down(opts); err != nil {
			die("down %s: %v", s, err)
		}
	}
	fmt.Println("✓ all migrations rolled back")
}

func runVersion(args []string) {
	schema, root := parseFlags(args)
	for _, s := range targetSchemas(schema, false) {
		opts := loadOptions(s, root)
		v, dirty, err := db.Version(opts)
		if err != nil {
			die("version %s: %v", s, err)
		}
		mark := ""
		if dirty {
			mark = " (dirty!)"
		}
		fmt.Printf("  %-12s %d%s\n", s, v, mark)
	}
}

func runForce(args []string) {
	var version int
	fs := flag.NewFlagSet("force", flag.ExitOnError)
	var schema, root string
	fs.StringVar(&schema, "schema", "", "schema to force (required)")
	fs.StringVar(&root, "migrations", "", "migrations root directory")
	fs.IntVar(&version, "version", -1, "version to force to (required)")
	_ = fs.Parse(args)
	if schema == "" || version < 0 {
		die("force requires -schema and -version")
	}
	opts := loadOptions(schema, root)
	if err := db.Force(opts, version); err != nil {
		die("force %s: %v", schema, err)
	}
	fmt.Printf("✓ forced %s to version %d\n", schema, version)
}

func targetSchemas(only string, reverse bool) []string {
	if only != "" {
		return []string{only}
	}
	out := append([]string(nil), db.Schemas...)
	if reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", a...)
	os.Exit(1)
}
