package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lunar-tear/server/internal/database"
	"lunar-tear/server/internal/gametime"
	"lunar-tear/server/internal/runtime"
	"lunar-tear/server/internal/store/sqlite"
	"lunar-tear/server/internal/tunables"
)

const masterDataPath = "assets/release/20240404193219.bin.e"

func main() {
	listen := flag.String("listen", "0.0.0.0:443", "gRPC listen address (host:port)")
	publicAddr := flag.String("public-addr", "127.0.0.1:443", "externally-reachable host:port advertised to clients")
	dbPath := flag.String("db", "db/game.db", "SQLite database path")
	octoURL := flag.String("octo-url", "", "Octo CDN base URL the client will use for assets (e.g. http://10.0.2.2:8080)")
	authURL := flag.String("auth-url", "", "Auth server base URL for Facebook token validation (e.g. http://localhost:3000)")
	adminListen := flag.String("admin-listen", "127.0.0.1:8082", "admin webhook listen address (host:port). Loopback by default; only binds when LUNAR_ADMIN_TOKEN is set.")
	noRegister := flag.Bool("no-register", false, "Disallow new account registrations for clients, when present. Default = false")
	tunablesPath := flag.String("tunables", "tunables.json", "JSON file overriding odds, prices and pity (see README: Tunables). Missing or empty = built-in defaults.")
	flag.Parse()

	if *octoURL == "" {
		log.Fatalf("--octo-url is required (e.g. http://10.0.2.2:8080)")
	}

	// Tuning has to be in place before the holder is built: gacha prices and
	// pity are baked into the banner catalog when master data is loaded.
	tun, err := tunables.Load(*tunablesPath)
	if err != nil {
		log.Fatalf("tunables: %v", err)
	}
	tun.Apply()
	if _, statErr := os.Stat(*tunablesPath); statErr == nil {
		log.Printf("tunables: loaded %s (%s)", *tunablesPath, tun.Summary())
	} else {
		log.Printf("tunables: no %s, using built-in defaults (%s)", *tunablesPath, tun.Summary())
	}

	holder, err := runtime.NewHolder(masterDataPath)
	if err != nil {
		log.Fatalf("init master data: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	log.Printf("database opened: %s", *dbPath)

	userStore := sqlite.New(db, gametime.Now)

	grpcServer := startGRPC(*listen, *publicAddr, *octoURL, *authURL, userStore, holder, *noRegister)

	startAdmin(*adminListen, holder)

	<-ctx.Done()
	log.Println("shutting down...")

	grpcServer.GracefulStop()
	database.Checkpoint(db)

	log.Println("shutdown complete")
}
