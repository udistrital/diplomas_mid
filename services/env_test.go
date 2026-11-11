package services

import (
	beego "github.com/beego/beego/v2/server/web"
	"os"
	"path/filepath"
	"testing"
)

func TestDotEnvPreservaEntornoYConfigMID(t *testing.T) {
	t.Setenv("PAZ_Y_SALVOS_MID_URL", "http://mid.environment")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("PAZ_Y_SALVOS_MID_URL=http://mid.file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PAZ_Y_SALVOS_MID_URL") != "http://mid.environment" {
		t.Fatal("environment overwritten")
	}
	old, _ := beego.AppConfig.String("PazYSalvosMidURL")
	defer beego.AppConfig.Set("PazYSalvosMidURL", old)
	t.Setenv("OIKOS_API_URL", "http://oikos.environment")
	oldOikos, _ := beego.AppConfig.String("OikosAPIURL")
	defer beego.AppConfig.Set("OikosAPIURL", oldOikos)
	if err := syncEnvToAppConfig(); err != nil {
		t.Fatal(err)
	}
	if pazYSalvosMidURL() != "http://mid.environment/v1" || oikosAPIURL() != "http://oikos.environment" {
		t.Fatal("environment mapping missing")
	}
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestParametrosNoDuplicaVersion(t *testing.T) {
	old, _ := beego.AppConfig.String("ParametrosCrudURL")
	defer beego.AppConfig.Set("ParametrosCrudURL", old)
	for _, base := range []string{"http://parametros", "http://parametros/v1", "http://parametros/v1/"} {
		beego.AppConfig.Set("ParametrosCrudURL", base)
		if got := parametrosCrudURL(); got != "http://parametros/v1" {
			t.Fatalf("duplicate prefix: %s", got)
		}
	}
}

func TestMIDNoDuplicaVersion(t *testing.T) {
	old, _ := beego.AppConfig.String("PazYSalvosMidURL")
	defer beego.AppConfig.Set("PazYSalvosMidURL", old)
	for _, base := range []string{"http://mid/paz_y_salvos_mid", "http://mid/paz_y_salvos_mid/v1", "http://mid/paz_y_salvos_mid/v1/"} {
		beego.AppConfig.Set("PazYSalvosMidURL", base)
		if got := pazYSalvosMidURL(); got != "http://mid/paz_y_salvos_mid/v1" {
			t.Fatalf("incorrect version: %s", got)
		}
	}
}

func TestRuntimePortValidation(t *testing.T) {
	oldPort := beego.BConfig.Listen.HTTPPort
	oldConfig, _ := beego.AppConfig.String("httpport")
	defer func() { beego.BConfig.Listen.HTTPPort = oldPort; beego.AppConfig.Set("httpport", oldConfig) }()
	for _, value := range []string{"abc", "0", "-1", "65536"} {
		t.Setenv("DIPLOMAS_MID_HTTP_PORT", value)
		if err := syncEnvToAppConfig(); err == nil {
			t.Fatalf("invalid port accepted: %s", value)
		}
	}
	t.Setenv("DIPLOMAS_MID_HTTP_PORT", "8091")
	if err := syncEnvToAppConfig(); err != nil {
		t.Fatal(err)
	}
	if beego.BConfig.Listen.HTTPPort != 8091 {
		t.Fatal("runtime port not applied")
	}
}

func TestDotEnvPreservesExplicitEmptyEnvironment(t *testing.T) {
	t.Setenv("PARAMETER_STORE", "")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("PARAMETER_STORE=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PARAMETER_STORE") != "" {
		t.Fatal("explicit environment overwritten")
	}
}

func TestDotEnvSynchronizesOASAndAcademicConfig(t *testing.T) {
	keys := map[string]string{
		"PARAMETER_STORE":                   "parameterStore",
		"ACADEMICA_PRUEBAS_API_URL":         "AcademicaPruebasAPIURL",
		"HOMOLOGACION_DEPENDENCIAS_API_URL": "HomologacionDependenciasAPIURL",
		"DIPLOMAS_MID_RUNMODE":              "runmode",
	}
	oldMode := beego.BConfig.RunMode
	t.Cleanup(func() { beego.BConfig.RunMode = oldMode })
	for envKey, confKey := range keys {
		old, _ := beego.AppConfig.String(confKey)
		t.Cleanup(func() { beego.AppConfig.Set(confKey, old) })
		t.Setenv(envKey, "")
		if err := os.Unsetenv(envKey); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), ".env")
	data := "PARAMETER_STORE=test-namespace\nACADEMICA_PRUEBAS_API_URL=http://academic.invalid\nHOMOLOGACION_DEPENDENCIAS_API_URL=http://mapping.invalid\nDIPLOMAS_MID_RUNMODE=dev\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if err := syncEnvToAppConfig(); err != nil {
		t.Fatal(err)
	}
	for envKey, confKey := range keys {
		got, _ := beego.AppConfig.String(confKey)
		if got != os.Getenv(envKey) {
			t.Fatalf("%s not synchronized", confKey)
		}
	}
	if beego.BConfig.RunMode != beego.DEV {
		t.Fatal("runtime runmode not synchronized")
	}
}
