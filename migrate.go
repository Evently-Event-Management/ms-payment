package migration

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func RunMigration() {
	// Command line flags
	envFlag := flag.String("env", "dev", "Environment (dev, test, prod)")
	envFileFlag := flag.String("env-file", "", "Path to .env file")
	migrationFlag := flag.String("migration", "migrate.sql", "Path to migration file")
	flag.Parse()

	// Load environment variables
	loadEnv(*envFlag, *envFileFlag)

	// Get database config from environment
	dbConfig := getDatabaseConfig()
	fmt.Printf("Connecting to MySQL at %s:%s as %s\n", dbConfig.Host, dbConfig.Port, dbConfig.Username)

	// Build DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database)

	// Connect to database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Check connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("Connected to database successfully")

	// Read migration file
	migrationFile := *migrationFlag
	migrationSQL, err := ioutil.ReadFile(migrationFile)
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	// Execute migration
	fmt.Printf("Executing migration from %s\n", migrationFile)
	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	fmt.Println("Migration completed successfully")
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
}

func getDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvOrDefault("DB_PORT", "3306"),
		Username: getEnvOrDefault("DB_USER", "root"),
		Password: getEnvOrDefault("DB_PASS", "password"),
		Database: getEnvOrDefault("DB_NAME", "payment_gateway"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func loadEnv(env string, envFile string) {
	// If explicit env file is provided
	if envFile != "" {
		if err := godotenv.Load(envFile); err == nil {
			fmt.Printf("Loaded environment from %s\n", envFile)
			return
		}
	}

	// Try environment-specific .env file
	envSpecificFile := fmt.Sprintf(".env.%s", env)
	if err := godotenv.Load(envSpecificFile); err == nil {
		fmt.Printf("Loaded environment from %s\n", envSpecificFile)
		return
	}

	// Fall back to default .env file
	if err := godotenv.Load(); err == nil {
		fmt.Println("Loaded environment from .env")
		return
	}

	fmt.Println("No .env file found, using default or system environment variables")
}
