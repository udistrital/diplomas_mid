package main

import (
	"os"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/filter/cors"
	_ "github.com/udistrital/diplomas_mid/routers"
	"github.com/udistrital/diplomas_mid/services"
	apistatus "github.com/udistrital/utils_oas/v2/apiStatusLib"
	"github.com/udistrital/utils_oas/v2/auditoria"
	customerrorv2 "github.com/udistrital/utils_oas/v2/customerror"
	"github.com/udistrital/utils_oas/v2/security"
	"github.com/udistrital/utils_oas/v2/xray"
)

func main() {
	if err := services.LoadRuntimeConfig(); err != nil {
		logs.Error("error cargando configuracion: %v", err)
		os.Exit(1)
	}

	beego.InsertFilter("*", beego.BeforeRouter, configuredCORSFilter())

	initializeOASMiddleware()
	beego.Run()
}

func initializeOASMiddleware() {
	apistatus.Init()
	auditoria.InitMiddleware()
	security.SetSecurityHeaders()
	xray.Init()
	beego.ErrorController(&customerrorv2.CustomErrorController{})
}

// Keep the institutional CORS policy in one place for startup and HTTP tests.
func configuredCORSFilter() beego.FilterFunc {
	allowedOrigins := []string{"*.udistrital.edu.co"}
	if beego.BConfig.RunMode == beego.DEV {
		allowedOrigins = []string{"*"}
	}
	return cors.Allow(&cors.Options{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Accept", "Content-Type", "Authorization", "User-Agent", "X-Amzn-Trace-Id"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
	})
}
