package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Edge-Center/external-dns-ec-webhook/log"
	"github.com/Edge-Center/external-dns-ec-webhook/provider"
	"sigs.k8s.io/external-dns/endpoint"
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

func main() {
	if Version == "" {
		Version = "unknown"
	}
	fmt.Printf(banner, Version)

	apiUrl := os.Getenv(provider.ENV_API_URL)
	apiToken := os.Getenv(provider.ENV_API_TOKEN)

	provider, err := provider.NewProvider(endpoint.DomainFilter{}, apiUrl, apiToken)
	if err != nil {
		log.Logger(context.Background()).Fatalf("failed to init provider: %s", err)
	}
	StartServer(provider)
}
