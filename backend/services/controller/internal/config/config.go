package config

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const LOCAL_ENV = ".env.local"

type Nats struct {
	Url                string
	Name               string
	VerifyCertificates bool
	Ctx                context.Context
}

type Mongo struct {
	Uri string
	Ctx context.Context
}

type RestApi struct {
	Port string
	Ctx  context.Context
}

type Enterprise struct {
	Enable          bool
	SupportPassword string
	SupportEmail    string
}

type Config struct {
	RestApi    RestApi
	Nats       Nats
	Mongo      Mongo
	Enterprise Enterprise
}

func NewConfig() *Config {

	loadEnvVariables()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	natsUrl := flag.String("nats_url", lookupEnvOrString("NATS_URL", "nats://localhost:4222"), "url for nats server")
	natsName := flag.String("nats_name", lookupEnvOrString("NATS_NAME", "controller"), "name for nats client")
	natsVerifyCertificates := flag.Bool("nats_verify_certificates", lookupEnvOrBool("NATS_VERIFY_CERTIFICATES", false), "verify validity of certificates from nats server")
	flApiPort := flag.String("api_port", lookupEnvOrString("REST_API_PORT", "8000"), "Rest api port")
	mongoUri := flag.String("mongo_uri", lookupEnvOrString("MONGO_URI", "mongodb://localhost:27017"), "uri for mongodb server")
	enterpise := flag.Bool("enterprise", lookupEnvOrBool("ENTERPRISE", false), "enterprise version enable")
	enterprise_support_password := flag.String("enterprise_support_password", lookupEnvOrString("ENTERPRISE_SUPPORT_PASSWORD", ""), "enterprise support password")
	enterpise_support_email := flag.String("enterprise_support_email", lookupEnvOrString("ENTERPRISE_SUPPORT_EMAIL", ""), "enterprise support email")
	flHelp := flag.Bool("help", false, "Help")

	/*
		App variables priority:
		1º - Flag through command line.
		2º - Env variables.
		3º - Default flag value.
	*/

	flag.Parse()

	if *flHelp {
		flag.Usage()
		os.Exit(0)
	}

	ctx := context.TODO()

	return &Config{
		RestApi: RestApi{
			Port: *flApiPort,
			Ctx:  ctx,
		},
		Nats: Nats{
			Url:                *natsUrl,
			Name:               *natsName,
			VerifyCertificates: *natsVerifyCertificates,
			Ctx:                ctx,
		},
		Mongo: Mongo{
			Uri: *mongoUri,
			Ctx: ctx,
		},
		Enterprise: Enterprise{
			Enable:          *enterpise,
			SupportPassword: *enterprise_support_password,
			SupportEmail:    *enterpise_support_email,
		},
	}
}

func loadEnvVariables() {
	err := godotenv.Load()

	if _, err := os.Stat(LOCAL_ENV); err == nil {
		_ = godotenv.Overload(LOCAL_ENV)
		log.Printf("Loaded variables from '%s'", LOCAL_ENV)
		return
	}

	if err != nil {
		log.Println("Error to load environment variables:", err)
	} else {
		log.Println("Loaded variables from '.env'")
	}
}

func lookupEnvOrString(key string, defaultVal string) string {
	if val, _ := os.LookupEnv(key); val != "" {
		return val
	}
	return defaultVal
}

func lookupEnvOrBool(key string, defaultVal bool) bool {
	if val, _ := os.LookupEnv(key); val != "" {
		v, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("LookupEnvOrBool[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}
