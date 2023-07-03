package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Configuration holds the project configuration
type Configuration struct {
	HostedServerUrl string
	RpcUrlMumbai    string
	FrontendUrl     string
	VerifierDID     string
}

// Load loads the configuration from a file
func Load() (*Configuration, error) {
	ctx := context.Background()
	err := godotenv.Load(".env")
	if err != nil {
		log.Println(ctx, "error loading .env file", err)
	}
	config := &Configuration{
		HostedServerUrl: os.Getenv("HOSTED_SERVER_URL"),
		RpcUrlMumbai:    os.Getenv("RPC_URL_MUMBAI"),
		FrontendUrl:     os.Getenv("FRONTEND_URL"),
		VerifierDID:     os.Getenv("VERIFIER_DID"),
	}
	checkEnvVars(ctx, config)
	return config, nil
}

func checkEnvVars(ctx context.Context, cfg *Configuration) {
	if cfg.HostedServerUrl == "" {
		log.Println(ctx, "HOSTED_SERVER_URL value is missing")
	}

	if cfg.RpcUrlMumbai == "" {
		log.Println(ctx, "RPC_URL_MUMBAI value is missing")
	}

	if cfg.FrontendUrl == "" {
		log.Println(ctx, "FRONTEND_URL value is missing")
	}

	if cfg.VerifierDID == "" {
		log.Println(ctx, "VERIFIER_DID value is missing")
	}
}
