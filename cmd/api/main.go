package main

import (
	"log"

	"apant_be/internal/bootstrap"
	"apant_be/internal/config"
)

func main() {
	cfg := config.Load()
	app, addr := bootstrap.BuildApp(cfg)
	log.Printf("starting %s on %s", cfg.AppName, addr)
	log.Fatal(app.Listen(addr))
}
