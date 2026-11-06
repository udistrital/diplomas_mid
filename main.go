package main

import (
	"os"
	"strings"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/filter/cors"
	_ "github.com/udistrital/diplomas_mid/routers"
	"github.com/udistrital/diplomas_mid/services"
)

func main() {
	if err := services.LoadRuntimeConfig(); err != nil {
		logs.Error("error cargando configuracion: %v", err)
		return
	}

	allowedOrigins := strings.TrimSpace(os.Getenv("DIPLOMAS_CORS_ALLOWED_ORIGINS"))
	if allowedOrigins == "" {
		allowedOrigins, _ = beego.AppConfig.String("CORSAllowedOrigins")
	}
	if filter := configuredCORSFilter(allowedOrigins); filter != nil {
		beego.InsertFilter("/v1/*", beego.BeforeRouter, filter)
	}

	beego.Run()
}

// Empty configuration keeps CORS disabled; deployments choose their own origins.
func configuredCORSFilter(raw string) beego.FilterFunc {
	if strings.HasPrefix(strings.TrimSpace(raw), "${") {
		return nil
	}
	origins := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		if origin := strings.TrimSpace(value); origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return nil
	}
	return cors.Allow(&cors.Options{
		AllowOrigins: origins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Accept", "Content-Type", "Authorization"},
	})
}
