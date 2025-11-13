package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Edge-Center/external-dns-ec-webhook/log"
	"github.com/Edge-Center/external-dns-ec-webhook/provider"
)

// Version is assigned during build stage and created from git tag
var Version string

const banner = `
$$$$$$$$\      $$\                      $$$$$$\                       $$\                         
$$  _____|     $$ |                    $$  __$$\                      $$ |                        
$$ |      $$$$$$$ | $$$$$$\   $$$$$$\  $$ /  \__| $$$$$$\  $$$$$$$\ $$$$$$\    $$$$$$\   $$$$$$\  
$$$$$\   $$  __$$ |$$  __$$\ $$  __$$\ $$ |      $$  __$$\ $$  __$$\\_$$  _|  $$  __$$\ $$  __$$\ 
$$  __|  $$ /  $$ |$$ /  $$ |$$$$$$$$ |$$ |      $$$$$$$$ |$$ |  $$ | $$ |    $$$$$$$$ |$$ |  \__|
$$ |     $$ |  $$ |$$ |  $$ |$$   ____|$$ |  $$\ $$   ____|$$ |  $$ | $$ |$$\ $$   ____|$$ |      
$$$$$$$$\\$$$$$$$ |\$$$$$$$ |\$$$$$$$\ \$$$$$$  |\$$$$$$$\ $$ |  $$ | \$$$$  |\$$$$$$$\ $$ |      
\________|\_______| \____$$ | \_______| \______/  \_______|\__|  \__|  \____/  \_______|\__|      
                   $$\   $$ |                                                                     
                   \$$$$$$  |                                                                     
                    \______/                                                                      

external-dns-ec-webhook
Version %s

`

const ENV_SERVER_ADDR = "EC_WEBHOOK_SERVER_ADDR"

func main() {
	if Version == "" {
		Version = "unknown"
	}
	fmt.Printf(banner, Version)

	apiUrl := os.Getenv(provider.ENV_API_URL)
	apiToken := os.Getenv(provider.ENV_API_TOKEN)
	dryRun := os.Getenv(provider.ENV_DRY_RUN) == "true"
	addr := os.Getenv(ENV_SERVER_ADDR)

	provider, err := provider.NewProvider(apiUrl, apiToken, dryRun)
	if err != nil {
		log.Logger(context.Background()).Fatalf("failed to init provider: %s", err)
	}

	StartServer(provider, addr)
}
