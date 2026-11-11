package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	beego "github.com/beego/beego/v2/server/web"
)

func TestCORSFrontend(t *testing.T) {
	for _, mode := range []string{beego.PROD, beego.DEV} {
		t.Run(mode, func(t *testing.T) { testCORSFrontend(t, mode) })
	}
}

func testCORSFrontend(t *testing.T, mode string) {
	baseURL := startInstitutionalAPI(t, mode)
	for _, origin := range []string{
		"https://portal.udistrital.edu.co",
		"http://localhost:4200", "http://localhost:4227",
		"https://portal.udistrital.edu.co.attacker.invalid", "https://outside.invalid",
	} {
		allowed := mode == beego.DEV || origin == "https://portal.udistrital.edu.co"
		for _, method := range []string{"DELETE", "GET", "OPTIONS", "PATCH", "POST", "PUT"} {
			req, err := http.NewRequest(http.MethodOptions, baseURL+"/v1/ceremonias", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", method)
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,x-amzn-trace-id")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("preflight status %d", res.StatusCode)
			}
			if !allowed {
				if res.Header.Get("Access-Control-Allow-Origin") != "" {
					t.Fatalf("production allowed %s", origin)
				}
				continue
			}
			if res.Header.Get("Access-Control-Allow-Origin") != origin || res.Header.Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatalf("%s: missing origin or credentials", origin)
			}
			if !strings.Contains(res.Header.Get("Access-Control-Allow-Methods"), method) {
				t.Fatalf("method %s blocked", method)
			}
			for _, header := range []string{"authorization", "content-type", "x-amzn-trace-id"} {
				if !strings.Contains(strings.ToLower(res.Header.Get("Access-Control-Allow-Headers")), header) {
					t.Fatalf("header %s blocked", header)
				}
			}
		}
	}
}

func TestInstitutionalMiddlewareHTTP(t *testing.T) {

	baseURL := startInstitutionalAPI(t, beego.PROD)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/", http.StatusOK},
		{"/missing-route", http.StatusNotFound},
		{"/v1/documento_digital/0/diploma/preview", http.StatusBadRequest},
	} {
		res, err := http.Get(baseURL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != tc.status {
			t.Fatalf("%s: status %d: %s", tc.path, res.StatusCode, string(body))
		}
		if tc.status != http.StatusNotFound && res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: missing security headers", tc.path)
		}
		if tc.path == "/" && !strings.Contains(string(body), `"status": "ok"`) && !strings.Contains(string(body), `"status":"ok"`) {
			t.Fatal("health response changed")
		}
		if tc.status == http.StatusNotFound && !strings.Contains(string(body), `"Success": false`) && !strings.Contains(string(body), `"Success":false`) {
			t.Fatal("missing institutional JSON error")
		}
	}
}

func TestInstitutionalAPIProcess(t *testing.T) {
	if os.Getenv("OAS_API_TEST_PROCESS") != "1" {
		return
	}
	main()
}

func startInstitutionalAPI(t *testing.T, mode string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestInstitutionalAPIProcess$")
	cmd.Env = append(os.Environ(), "OAS_API_TEST_PROCESS=1", "PARAMETER_STORE=", "DIPLOMAS_MID_RUNMODE="+mode, fmt.Sprintf("DIPLOMAS_MID_HTTP_PORT=%d", port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: time.Second}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		res, err := client.Get(baseURL + "/")
		if err == nil {
			res.Body.Close()
			return baseURL
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("API failed to start")
	return ""
}
