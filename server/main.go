// Command server runs the Montana Route Planner: the JSON API, the admin
// pages, and the static frontend in web/.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // the distroless image has no time zone files

	"github.com/DefNotXavier/WeatherApp/server/api"
	"github.com/DefNotXavier/WeatherApp/server/auth"
	"github.com/DefNotXavier/WeatherApp/server/geo"
	"github.com/DefNotXavier/WeatherApp/server/store"
	"github.com/DefNotXavier/WeatherApp/server/weather"
	"github.com/redis/go-redis/v9"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check /healthz and exit (used by the Docker healthcheck)")
	flag.Parse()

	port := env("PORT", "8080")
	if *healthcheck {
		os.Exit(checkHealth(port))
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(port, log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(port string, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	// The highways don't change while the app is running, so the routing
	// graph is built once at startup.
	vertices, err := db.HighwayVertices(ctx)
	if err != nil {
		return fmt.Errorf("loading highway nodes: %w", err)
	}
	graph := geo.NewGraph(vertices)
	log.Info("routing graph loaded", "vertices", len(vertices), "nodes", graph.Len())

	srv := &api.Server{
		Store:    db,
		Graph:    graph,
		Sessions: auth.RedisSessions{RDB: rdb},
		Weather: weather.New(
			env("NWS_BASE_URL", "https://api.weather.gov"),
			env("NWS_USER_AGENT", "MontanaRoutePlanner/2.0 (github.com/DefNotXavier/WeatherApp)"),
			weather.RedisCache{RDB: rdb},
			log,
		),
		Log:             log,
		WebDir:          env("WEB_DIR", "../web"),
		CheckpointMiles: envFloat("CHECKPOINT_MILES", 15),
		NearbyMiles:     envFloat("NEARBY_MILES", 5),
		SecureCookies:   env("SECURE_COOKIES", "false") == "true",
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdown)
	}()

	log.Info("listening", "port", port)
	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func checkHealth(port string) int {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://localhost:" + port + "/healthz")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", key)
		os.Exit(1)
	}
	return v
}

func envFloat(key string, fallback float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return f
	}
	return fallback
}
