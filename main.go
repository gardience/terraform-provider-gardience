// The Gardience provider for Terraform and OpenTofu: enrollments with first-boot scripts, node groups, saved
// states, event rules and machines, through the Gardience configuration API (configure keys).
package main

import (
	"context"
	"flag"
	"log"

	"github.com/gardience/terraform-provider-gardience/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is set by the release build.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/gardience/gardience",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
