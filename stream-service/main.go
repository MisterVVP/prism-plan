package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"stream-service/api"
)

func main() {
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
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
	readModelUpdatesChannel := os.Getenv("TASK_UPDATES_CHANNEL")
	if readModelUpdatesChannel == "" {
		log.Fatal("TASK_UPDATES_CHANNEL environment variable is empty or not defined")
	}
	cacheExpStr := os.Getenv("REDIS_CACHE_EXPIRATION")
	if cacheExpStr == "" {
		log.Fatal("REDIS_CACHE_EXPIRATION environment variable is empty or not defined")
	}
	cacheExpiration, err := time.ParseDuration(cacheExpStr)
	if err != nil {
		log.Fatalf("parse REDIS_CACHE_EXPIRATION: %v", err)
	}

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
	auth := api.NewAuth(jwks, jwtAudience, "https://"+domain+"/")

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	api.Register(e, rc, auth, readModelUpdatesChannel, cacheExpiration)

	listenAddr := ":9000"
	if val, ok := os.LookupEnv("STREAM_SERVICE_PORT"); ok {
		listenAddr = ":" + val
	}

	e.Logger.Fatal(e.Start(listenAddr))
}
