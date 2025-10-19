package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	SupabaseURL         string 
	SupabaseServiceKey  string 
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Could not find .env file, using environment variables from OS")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Error: DATABASE_URL is not set in the environment")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("Error: JWT_SECRET is not set")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		log.Fatal("Error: SUPABASE_URL is not set")
	}
	supabaseServiceKey := os.Getenv("SUPABASE_SERVICE_KEY")
	if supabaseServiceKey == "" {
		log.Fatal("Error: SUPABASE_SERVICE_KEY is not set")
	}

	return &Config{
		DatabaseURL: dbURL,
		JWTSecret:          jwtSecret,
		SupabaseURL:        supabaseURL,        
		SupabaseServiceKey: supabaseServiceKey,
	}, nil

	
}