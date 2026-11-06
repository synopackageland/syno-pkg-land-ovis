// Command app is the package backend. DSM runs it as a CGI (ui/cgi/api.cgi in
// the package) under the package account, once per request, and as the
// uninstall hook (`api.cgi uninstall`) when dsm-package.yaml declares one.
//
//line Ovis/cmd/app/main.go:1
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/dsm"
	"github.com/synopackageland/syno-pkg-land-sdk/filestation"
	standaloneidentity "github.com/synopackageland/syno-pkg-land-sdk/identity"
	"github.com/synopackageland/syno-pkg-land-sdk/sharedfolder"

	appapi "Ovis/internal/api"
	"Ovis/internal/app"
)

// Set by `synopkgland build-spk` through -ldflags -X; never read from a request.
var (
	delegateScope            = ""
	delegateOperationsJSON   = ""
	delegateDestinationsJSON = ""
	delegateLifetime         = ""
	executorDigest           = ""
	policyDigest             = ""
	packageID                = "dev"
	version                  = "0.0.0"
	// "session" when the project opted into the interactive File Station adapter.
	fileStation = ""
	// "standalone" enables the SDK identity adapter; the runtime owns authentication.
	identity = ""
	// dsm-package.yaml entry: "admin-only" or "app-privilege"; empty keeps the route policy alone.
	entry = ""
	// "on" when dsm-package.yaml declares portal: serves portal.status;
	// "executor" below DSM 7.2, where the executor registers the entry.
	portal = ""
	// backend.timeout_seconds, api.DefaultRequestTimeout unless set.
	requestTimeout = ""
	// Build enables registry equality only for Apps that opt in to ctl.registry.
	operationRegistry = ""
)

// deadline is the CGI request deadline: requestTimeout seconds, or
// api.DefaultRequestTimeout when it is unset or not a positive number.
func deadline() api.DeadlineOption {
	seconds, err := strconv.Atoi(requestTimeout)
	if err != nil || seconds <= 0 {
		return api.Timeout(api.DefaultRequestTimeout)
	}
	return api.Timeout(time.Duration(seconds) * time.Second)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(packageID + ": ")
	if os.Getenv("GATEWAY_INTERFACE") == "" && len(os.Args) == 2 && os.Args[1] == "uninstall" {
		if err := app.Uninstall("/var/packages/"+packageID+"/var", os.Getenv("SYNOPKGLAND_DELETE_STATE") == "1"); err != nil {
			log.Fatal(err)
		}
		return
	}
	if os.Getenv("GATEWAY_INTERFACE") == "" && len(os.Args) == 2 && os.Args[1] == "--describe" {
		permissions, err := appapi.Permissions()
		if err != nil {
			log.Fatal(err)
		}
		cfg := app.Config{PackageID: packageID, Version: version, DataDir: ".", Portal: portal != "", PortalExecutor: portal == "executor"}
		if app.SessionFilesEnabled(fileStation) {
			cfg.SessionFiles = filestation.Local(cfg.DataDir)
		}
		methods, err := api.DescribeRoutes(permissions, app.Routes(cfg)...)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(methods); err != nil {
			log.Fatal(err)
		}
		return
	}
	if os.Getenv("GATEWAY_INTERFACE") == "" {
		fmt.Fprintf(os.Stderr, "%s %s: this program runs as the package's DSM CGI; open the app from the DSM main menu.\n", packageID, version)
		os.Exit(2)
	}
	cfg := app.Config{
		DelegateScope:            delegateScope,
		DelegateOperationsJSON:   delegateOperationsJSON,
		DelegateDestinationsJSON: delegateDestinationsJSON,
		DelegateLifetime:         delegateLifetime,
		ExecutorDigest:           executorDigest, PolicyDigest: policyDigest,
		PackageID:      packageID,
		Version:        version,
		DataDir:        "/var/packages/" + packageID + "/var",
		Portal:         portal != "",
		PortalExecutor: portal == "executor",
		// install.sharedFolder in dsm-package.yaml: `synopkgland init --shared-folder`
		// writes key "shared" with permission rw. Keep this equal to it.
		SharedFolderRequired: map[string]sharedfolder.Access{"shared": sharedfolder.Write},
	}
	if _, err := app.ParseDelegateConfig(cfg); err != nil {
		log.Fatal(err)
	}
	if app.SessionFilesEnabled(fileStation) {
		cfg.SessionFiles = filestation.Local(cfg.DataDir)
	}
	// Kernels without memfd_create stage DSM's bridges in the package's own temp.
	dsm.SetBridgeFallbackDir("/var/packages/" + packageID + "/tmp")
	var auth api.Authenticator = dsm.NewAuthenticator()
	var standalone *standaloneidentity.Service
	if identity == "standalone" {
		var err error
		standalone, err = app.StandaloneIdentity(cfg, os.Getenv("SCRIPT_NAME"))
		if err != nil {
			log.Fatal("identity configuration unavailable")
		}
		auth = standalone
	}
	auth, err := dsm.NewEntryGate(auth, entry, packageID, cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	permissions, err := appapi.Permissions()
	if err != nil {
		log.Fatal(err)
	}
	var srv *api.Server
	if operationRegistry == "on" {
		srv, err = api.NewServerWithOperations(auth, appapi.Operations(), permissions, app.Routes(cfg)...)
	} else {
		srv, err = api.NewServerWithPermissions(auth, permissions, app.Routes(cfg)...)
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Migrate(cfg); err != nil {
		log.Fatalf("schema migration failed: %v", err)
	}
	var handler http.Handler = srv
	if standalone != nil {
		handler = standalone.Wrap(handler)
	}
	descriptorAuth, err := dsm.NewEntryGate(dsm.NewAuthenticator(), entry, packageID, cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	handler = api.WithDescription(handler, descriptorAuth, filepath.Join("/var/packages", packageID, "target", api.DescriptionFile))
	if err := cgi.Serve(api.WithRequestDeadline(handler, deadline())); err != nil {
		log.Fatal(err)
	}
}
