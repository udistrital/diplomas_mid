package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	beegoctx "github.com/beego/beego/v2/server/web/context"
)

func TestCORSAuthorizationPreflight(t *testing.T) {
	filter := configuredCORSFilter("http://localhost:4200,http://localhost:4227")
	for _, tc := range []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:4200", true},
		{"http://localhost:4227", true},
		{"https://outside.example", false},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/v1/firmantes/rol-activo", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			req.Header.Set("Access-Control-Request-Headers", "authorization")
			res := httptest.NewRecorder()
			ctx := beegoctx.NewContext()
			ctx.Reset(res, req)
			filter(ctx)
			if res.Code != http.StatusOK {
				t.Fatalf("preflight status %d", res.Code)
			}
			if tc.allowed {
				if res.Header().Get("Access-Control-Allow-Origin") != tc.origin {
					t.Fatal("missing origin")
				}
				if !strings.Contains(strings.ToLower(res.Header().Get("Access-Control-Allow-Headers")), "authorization") {
					t.Fatal("Authorization not allowed")
				}
				if !strings.Contains(res.Header().Get("Access-Control-Allow-Methods"), "GET") {
					t.Fatal("GET not allowed")
				}
			} else if res.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unconfigured origin allowed")
			}
		})
	}
	if configuredCORSFilter(" , ") != nil {
		t.Fatal("empty configuration enabled CORS")
	}
}
