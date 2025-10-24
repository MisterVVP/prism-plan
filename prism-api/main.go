package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"prism-api/api"
	"prism-api/storage"
)

func configureJSONLogger(logger *log.Logger) {
	if logger == nil {
		return
	}
	logger.SetFormatter(&log.JSONFormatter{
		TimestampFormat:   time.RFC3339Nano,
		DisableHTMLEscape: true,
	})
	logger.SetOutput(os.Stdout)
}

func main() {
	configureJSONLogger(log.StandardLogger())
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
	}
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tasksTableName := os.Getenv("TASKS_TABLE")
	settingsTableName := os.Getenv("SETTINGS_TABLE")
	commandQueueName := os.Getenv("COMMAND_QUEUE")
	if connStr == "" || tasksTableName == "" || settingsTableName == "" || commandQueueName == "" {
		log.Fatal("missing storage config")
	}

	taskPageSize := 10
	if v := os.Getenv("TASKS_PAGE_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("invalid TASKS_PAGE_SIZE: %v", err)
		}
		if n <= 0 {
			log.Fatalf("invalid TASKS_PAGE_SIZE: must be greater than zero")
		}
		taskPageSize = n
	}
	queueConcurrency := storage.DefaultQueueConcurrency()
	if v := os.Getenv("COMMAND_QUEUE_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("invalid COMMAND_QUEUE_CONCURRENCY: %v", err)
		}
		if n <= 0 {
			log.Fatalf("invalid COMMAND_QUEUE_CONCURRENCY: must be greater than zero")
		}
		queueConcurrency = n
	}

	redisConn := os.Getenv("REDIS_CONNECTION_STRING")
	if redisConn == "" {
		log.Fatal("missing redis config")
	}
	redisOpts, err := redis.ParseURL(redisConn)
	if err != nil {
		parts := strings.Split(redisConn, ",")
		redisOpts = &redis.Options{Addr: parts[0]}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch strings.ToLower(kv[0]) {
			case "password":
				redisOpts.Password = kv[1]
			case "ssl":
				if strings.ToLower(kv[1]) == "true" {
					redisOpts.TLSConfig = &tls.Config{}
				}
			}
		}
	}
	rc := redis.NewClient(redisOpts)

	store, err := storage.New(
		connStr,
		tasksTableName,
		settingsTableName,
		commandQueueName,
		taskPageSize,
		storage.WithQueueConcurrency(queueConcurrency),
		storage.WithCache(rc),
	)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	warmupCtx, cancelWarmup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelWarmup()
	if err := store.Warmup(warmupCtx); err != nil {
		log.WithError(err).Warn("storage warmup failed")
	} else {
		log.Info("storage warmup completed")
	}

	ttl := 24 * time.Hour
	if v := os.Getenv("DEDUPER_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Fatalf("invalid DEDUPER_TTL: %v", err)
		}
		ttl = d
	}
	deduper := api.NewRedisDeduper(rc, ttl)

	testMode := os.Getenv("AUTH0_TEST_MODE") == "1"
	var auth *api.Auth
	if testMode {
		auth = api.NewAuth(nil, "", "")
	} else {
		jwtAudience := os.Getenv("AUTH0_AUDIENCE")
		domain := os.Getenv("AUTH0_DOMAIN")
		if jwtAudience == "" || domain == "" {
			log.Fatal("missing Auth0 config")
		}
		jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
		jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{})
		if err != nil {
			log.Fatalf("jwks: %v", err)
		}
		auth = api.NewAuth(jwks, jwtAudience, "https://"+domain+"/")
	}

	e := echo.New()
	e.Use(middleware.Decompress())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.Use(middleware.Gzip())
	logger := log.New()
	configureJSONLogger(logger)
	logger.SetLevel(log.GetLevel())
	api.Register(e, store, auth, deduper, logger)
	if os.Getenv("APP_ENV") == "development" {
		log.Println("Enabling pprof for profiling")
		pprof.Register(e)
	}
	if port := os.Getenv("PORT"); port != "" {
		e.Logger.Fatal(e.Start(":" + port))
	} else {
		log.Fatal("PORT is empty")
	}
}
