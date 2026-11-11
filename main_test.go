package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
	beegoctx "github.com/beego/beego/v2/server/web/context"
)

func TestCORSFrontend(t *testing.T) {
	// Beego 2.3.10 accumulates origin patterns globally. Isolate modes so a dev
	// wildcard cannot contaminate the production assertions.
	mode := os.Getenv("OAS_CORS_TEST_MODE")
	if mode == "" {
		for _, mode := range []string{beego.PROD, beego.DEV} {
			t.Run(mode, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestCORSFrontend$")
				cmd.Env = append(os.Environ(), "OAS_CORS_TEST_MODE="+mode)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v: %s", err, output)
				}
			})
		}
		return
	}
	beego.BConfig.RunMode = mode
	filter := configuredCORSFilter()
	for _, origin := range []string{
		"https://portal.udistrital.edu.co",
		"http://localhost:4200", "http://localhost:4227",
		"https://portal.udistrital.edu.co.attacker.invalid", "https://outside.invalid",
	} {
		allowed := mode == beego.DEV || origin == "https://portal.udistrital.edu.co"
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			req := httptest.NewRequest(http.MethodOptions, "/v1/ceremonias", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", method)
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,x-amzn-trace-id")
			res := httptest.NewRecorder()
			ctx := beegoctx.NewContext()
			ctx.Reset(res, req)
			filter(ctx)
			if res.Code != http.StatusOK {
				t.Fatalf("preflight status %d", res.Code)
			}
			if !allowed {
				if res.Header().Get("Access-Control-Allow-Origin") != "" {
					t.Fatalf("production allowed %s", origin)
				}
				continue
			}
			if res.Header().Get("Access-Control-Allow-Origin") != origin || res.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatalf("%s: missing origin or credentials", origin)
			}
			if !strings.Contains(res.Header().Get("Access-Control-Allow-Methods"), method) {
				t.Fatalf("method %s blocked", method)
			}
			for _, header := range []string{"authorization", "content-type", "x-amzn-trace-id"} {
				if !strings.Contains(strings.ToLower(res.Header().Get("Access-Control-Allow-Headers")), header) {
					t.Fatalf("header %s blocked", header)
				}
			}
		}
		req := httptest.NewRequest(http.MethodGet, "/v1/diplomas/registro-control/archivo", nil)
		req.Header.Set("Origin", origin)
		res := httptest.NewRecorder()
		ctx := beegoctx.NewContext()
		ctx.Reset(res, req)
		filter(ctx)
		if allowed && !strings.Contains(res.Header().Get("Access-Control-Expose-Headers"), "Content-Disposition") {
			t.Fatal("download filename unavailable to frontend")
		}
	}
}

func TestInstitutionalMiddlewareHTTP(t *testing.T) {
	// Avoid AWS access; Init logs missing parameterStore and leaves tracing disabled.
	old, _ := beego.AppConfig.String("parameterStore")
	defer beego.AppConfig.Set("parameterStore", old)
	beego.AppConfig.Set("parameterStore", "")
	initializeOASMiddleware()
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/", http.StatusOK},
		{"/missing-route", http.StatusNotFound},
		{"/v1/documento_digital/0/diploma/preview", http.StatusBadRequest},
	} {
		res := httptest.NewRecorder()
		beego.BeeApp.Handlers.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if res.Code != tc.status {
			t.Fatalf("%s: status %d: %s", tc.path, res.Code, res.Body.String())
		}
		if tc.status != http.StatusNotFound && res.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: missing security headers", tc.path)
		}
		if tc.path == "/" && !strings.Contains(res.Body.String(), `"status": "ok"`) && !strings.Contains(res.Body.String(), `"status":"ok"`) {
			t.Fatal("health response changed")
		}
		if tc.status == http.StatusNotFound && !strings.Contains(res.Body.String(), `"Success": false`) && !strings.Contains(res.Body.String(), `"Success":false`) {
			t.Fatal("missing institutional JSON error")
		}
	}
}
