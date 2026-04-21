package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
	"github.com/CBookShu/kd48/tools/config-loader/internal/gogen"
	"github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
	"github.com/CBookShu/kd48/tools/config-loader/internal/mysqlwriter"
	"github.com/CBookShu/kd48/tools/config-loader/internal/redisnotify"
	"github.com/CBookShu/kd48/tools/config-loader/internal/validator"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

func main() {
	inputFile := flag.String("input", "", "CSV input file path (required)")
	outputFile := flag.String("output", "", "JSON output file path (default: stdout)")
	mysqlDSN := flag.String("mysql-dsn", "", "MySQL DSN (optional, enables DB write)")
	redisAddr := flag.String("redis-addr", "", "Redis address (optional, enables notification)")
	scope := flag.String("scope", "", "Business scope (checkin/reward/rank/task)")
	title := flag.String("title", "", "Config title")
	revisionFlag := flag.Int64("revision", 0, "Explicit revision (default: unix_millis)")
	genGo := flag.Bool("gen-go", false, "Enable Go code generation")
	goOut := flag.String("go-out", "", "Go output file path")
	goPkg := flag.String("go-package", "lobbyconfig", "Go package name")
	dryRun := flag.Bool("dry-run", false, "Validate only, do not write")
	verbose := flag.Bool("verbose", false, "Verbose logging")
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "error: -input is required")
		os.Exit(1)
	}

	if *verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	// Parse CSV
	f, err := os.Open(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open file: %v\n", err)
		os.Exit(2)
	}

	parser := csvparser.NewParser()
	sheet, err := parser.Parse(f, *inputFile)
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse CSV: %v\n", err)
		os.Exit(2)
	}
	slog.Info("parsed CSV", "config_name", sheet.ConfigName, "rows", len(sheet.Rows))

	// Validate
	v := validator.NewValidator()
	if err := v.Validate(sheet); err != nil {
		fmt.Fprintf(os.Stderr, "error: validate: %v\n", err)
		os.Exit(3)
	}
	slog.Info("validation passed")

	if *dryRun {
		fmt.Println("dry-run: validation passed")
		os.Exit(0)
	}

	// Generate JSON
	revision := *revisionFlag
	if revision == 0 {
		revision = time.Now().UnixMilli()
	}

	gen := jsongen.NewGenerator()
	payload, err := gen.Generate(sheet, revision)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generate JSON: %v\n", err)
		os.Exit(3)
	}

	// Output JSON
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal JSON: %v\n", err)
		os.Exit(3)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, jsonBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
			os.Exit(3)
		}
		slog.Info("wrote JSON", "path", *outputFile)
	} else {
		fmt.Println(string(jsonBytes))
	}

	// MySQL write
	if *mysqlDSN != "" {
		f, err = os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reopen file: %v\n", err)
			os.Exit(2)
		}
		csvText, _ := io.ReadAll(f)
		f.Close()

		db, err := sql.Open("mysql", *mysqlDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: connect MySQL: %v\n", err)
			os.Exit(4)
		}
		defer db.Close()

		w := mysqlwriter.NewWriter(db)
		if err := w.Write(payload, mysqlwriter.WriteOptions{
			Scope:   *scope,
			Title:   *title,
			CSVText: string(csvText),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error: write MySQL: %v\n", err)
			os.Exit(4)
		}
		slog.Info("wrote to MySQL", "config_name", payload.ConfigName, "revision", payload.Revision)

		// Redis notify
		if *redisAddr != "" {
			rdb := redis.NewClient(&redis.Options{Addr: *redisAddr})
			p := redisnotify.NewPublisher(rdb, "kd48:lobby:config:notify")
			if err := p.Publish(context.Background(), payload.ConfigName, payload.Revision); err != nil {
				slog.Error("Redis publish failed", "error", err)
			} else {
				slog.Info("published to Redis")
			}
		}
	}

	// Go code generation
	if *genGo {
		goGen := gogen.NewGenerator()
		code, err := goGen.Generate(sheet, *goPkg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: generate Go: %v\n", err)
			os.Exit(5)
		}

		goOutPath := *goOut
		if goOutPath == "" {
			goOutPath = fmt.Sprintf("generated/%s.go", sheet.ConfigName)
		}
		if err := os.WriteFile(goOutPath, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write Go file: %v\n", err)
			os.Exit(5)
		}
		slog.Info("generated Go code", "path", goOutPath)
	}

	slog.Info("done", "config_name", payload.ConfigName, "revision", payload.Revision)
}
