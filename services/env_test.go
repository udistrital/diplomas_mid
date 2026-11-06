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
