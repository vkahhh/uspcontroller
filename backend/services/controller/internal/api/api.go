package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/leandrofars/oktopus/internal/api/cors"
	"github.com/leandrofars/oktopus/internal/api/middleware"
	"github.com/leandrofars/oktopus/internal/bridge"
	"github.com/leandrofars/oktopus/internal/config"
	"github.com/leandrofars/oktopus/internal/db"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Api struct {
	port      string
	js        jetstream.JetStream
	nc        *nats.Conn
	bridge    bridge.Bridge
	db        db.Database
	kv        jetstream.KeyValue
	ctx       context.Context
	enterpise config.Enterprise
}

const REQUEST_TIMEOUT = time.Second * 30

func NewApi(c *config.Config, js jetstream.JetStream, nc *nats.Conn, bridge bridge.Bridge, d db.Database, kv jetstream.KeyValue) Api {
	return Api{
		port:      c.RestApi.Port,
		js:        js,
		nc:        nc,
		ctx:       c.RestApi.Ctx,
		bridge:    bridge,
		db:        d,
		kv:        kv,
		enterpise: c.Enterprise,
	}
}

func (a *Api) StartApi() {

	if a.enterpise.SupportPassword != "" && a.enterpise.SupportEmail != "" {
		go registerEnterpriseSupport(a.enterpise.SupportEmail, a.enterpise.SupportPassword, a.db)
	}

	r := mux.NewRouter()
	authentication := r.PathPrefix("/api/auth").Subrouter()
	authentication.HandleFunc("/login", a.generateToken).Methods("PUT")
	authentication.HandleFunc("/register", a.registerUser).Methods("POST")
	authentication.HandleFunc("/delete/{user}", a.deleteUser).Methods("DELETE")
	authentication.HandleFunc("/password/{user}", a.changePassword).Methods("PUT")
	authentication.HandleFunc("/password", a.changePassword).Methods("PUT")
	authentication.HandleFunc("/admin/register", a.registerAdminUser).Methods("POST")
	authentication.HandleFunc("/admin/exists", a.adminUserExists).Methods("GET")
	iot := r.PathPrefix("/api/device").Subrouter()
	iot.HandleFunc("/alias", a.setDeviceAlias).Methods("PUT")
	iot.HandleFunc("/auth", a.deviceAuth).Methods("GET", "POST", "DELETE")
	iot.HandleFunc("/cwmp/{sn}/getParameterNames", a.cwmpGetParameterNamesMsg).Methods("PUT")
	iot.HandleFunc("/cwmp/{sn}/getParameterValues", a.cwmpGetParameterValuesMsg).Methods("PUT")
	iot.HandleFunc("/cwmp/{sn}/getParameterAttributes", a.cwmpGetParameterAttributesMsg).Methods("PUT")
	iot.HandleFunc("/cwmp/{sn}/setParameterValues", a.cwmpSetParameterValuesMsg).Methods("PUT")
	iot.HandleFunc("/cwmp/{sn}/addObject", a.cwmpAddObjectMsg).Methods("PUT")
	iot.HandleFunc("/cwmp/{sn}/deleteObject", a.cwmpDeleteObjectMsg).Methods("PUT")
	iot.HandleFunc("", a.retrieveDevices).Methods("GET")
	iot.HandleFunc("/{id}", a.retrieveDevices).Methods("GET")
	iot.HandleFunc("/{sn}/{mtp}/get", a.deviceGetMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/add", a.deviceCreateMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/del", a.deviceDeleteMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/set", a.deviceUpdateMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/notify", a.deviceNotifyMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/parameters", a.deviceGetSupportedParametersMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/instances", a.deviceGetParameterInstances).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/operate", a.deviceOperateMsg).Methods("PUT")
	iot.HandleFunc("/{sn}/{mtp}/fw_update", a.deviceFwUpdate).Methods("PUT") //TODO: put it to work and generalize for usp and cwmp
	if a.enterpise.Enable {
		iot.HandleFunc("/{sn}/sitesurvey", a.deviceSiteSurvey).Methods("GET")
		iot.HandleFunc("/{sn}/connecteddevices", a.deviceConnectedDevices).Methods("GET")
		iot.HandleFunc("/{sn}/traceroute", a.deviceTraceRoute).Methods("GET", "PUT")
		iot.HandleFunc("/{sn}/speedtest", a.deviceSpeedTest).Methods("PUT")
		iot.HandleFunc("/{sn}/ping", a.devicePing).Methods("PUT", "GET")
	}
	iot.HandleFunc("/{sn}/wifi", a.deviceWifi).Methods("PUT", "GET")
	dash := r.PathPrefix("/api/info").Subrouter()
	dash.HandleFunc("/vendors", a.vendorsInfo).Methods("GET")
	dash.HandleFunc("/status", a.statusInfo).Methods("GET")
	dash.HandleFunc("/device_class", a.productClassInfo).Methods("GET")
	dash.HandleFunc("/general", a.generalInfo).Methods("GET")
	users := r.PathPrefix("/api/users").Subrouter()
	users.HandleFunc("", a.retrieveUsers).Methods("GET")

	/* ----- Middleware for requests which requires user to be authenticated ---- */
	iot.Use(func(handler http.Handler) http.Handler {
		return middleware.Middleware(handler)
	})

	dash.Use(func(handler http.Handler) http.Handler {
		return middleware.Middleware(handler)
	})

	users.Use(func(handler http.Handler) http.Handler {
		return middleware.Middleware(handler)
	})
	/* -------------------------------------------------------------------------- */

	corsOpts := cors.GetCorsConfig()

	srv := &http.Server{
		Addr:         "0.0.0.0:" + a.port,
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
		IdleTimeout:  time.Second * 60,
		Handler:      corsOpts.Handler(r),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	log.Println("Running REST API at port", a.port)
}

func registerEnterpriseSupport(email, password string, d db.Database) {

	user := db.User{
		Email:    email,
		Password: password,
		Name:     "Enterprise Support",
		Level:    db.AdminUser,
	}

	for {
		if err := user.HashPassword(password); err != nil {
			return
		}

		err := d.RegisterUser(user)
		if err != nil {
			if err == db.ErrorUserExists {
				log.Println("Enterprise support user already registered.")
				return
			}
			log.Println("Error to register enterprise support user:", err)
			time.Sleep(time.Second * 5)
			continue
		}
		log.Println("Enterprise support user registered successfully.")
		return
	}
}
