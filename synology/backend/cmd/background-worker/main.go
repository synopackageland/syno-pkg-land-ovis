// Command background-worker is the package's fixed background worker. DSM runs
// it as the package's systemd user unit under the package account; it never
// runs as root and takes no input except the jobs in its private spool.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"github.com/synopackageland/syno-pkg-land-sdk/background"

	"Ovis/internal/app"
)

// Set by `synopkgland build-spk` through -ldflags -X.
var packageID = "dev"

func main() {
	log.SetFlags(0)
	log.SetPrefix(packageID + " background: ")
	if err := background.RefuseRoot(os.Geteuid()); err != nil {
		log.Fatal(err)
	}
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	info := background.WorkerInfo{PID: os.Getpid(), UID: os.Getuid(), Username: name}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	log.Printf("started as uid %s", strconv.Itoa(info.UID))
	runner := background.NewRunner("/var/packages/"+packageID+"/var/background", info)
	runner.Work = app.HashWork
	if err := runner.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
