package services

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	beego "github.com/beego/beego/v2/server/web"
)

func LoadRuntimeConfig() error {
	if err := loadDotEnv(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	return syncEnvToAppConfig()
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}

func syncEnvToAppConfig() error {
	// AppConfig.Set writes into the current runmode section. Select it first.
	if runMode := os.Getenv("DIPLOMAS_MID_RUNMODE"); runMode != "" {
		beego.BConfig.RunMode = runMode
	}

	envToConf := map[string]string{
		"PARAMETER_STORE":                   "parameterStore",
		"ACADEMICA_PRUEBAS_API_URL":         "AcademicaPruebasAPIURL",
		"HOMOLOGACION_DEPENDENCIAS_API_URL": "HomologacionDependenciasAPIURL",
		"OIKOS_API_URL":                     "OikosAPIURL",
		"DIPLOMAS_MID_HTTP_PORT":            "httpport",
		"DIPLOMAS_MID_RUNMODE":              "runmode",
		"FIRMA_ELECTRONICA_MID_URL":         "FirmaElectronicaMidURL",
		"DIPLOMAS_CRUD_URL":                 "DiplomasCrudURL",
		"ADMINISTRATIVA_AMAZON_API_URL":     "AdministrativaAmazonAPIURL",
		"DOCUMENTOS_CRUD_URL":               "DocumentosCrudURL",
		"GESTOR_DOCUMENTAL_URL":             "GestorDocumentalURL",
		"PAZ_Y_SALVOS_MID_URL":              "PazYSalvosMidURL",
		"ACADEMICA_CORE_SERVICE_URL":        "AcademicaCoreServiceURL",
		"PARAMETROS_CRUD_URL":               "ParametrosCrudURL",
		"DIPLOMA_INSTITUTIONAL_LOGO_PATH":   "DiplomaInstitutionalLogoPath",
		"DIPLOMA_CAMBRIA_FONT_PATH":         "DiplomaCambriaFontPath",
		"DIPLOMA_CAMBRIA_BOLD_FONT_PATH":    "DiplomaCambriaBoldFontPath",
		"DIPLOMA_ENGRAVERS_FONT_PATH":       "DiplomaEngraversFontPath",
		"AWS_REGION":                        "AWSRegion",
		"AWS_ENDPOINT_URL":                  "AWSEndpointURL",
		"AWS_ACCESS_KEY_ID":                 "AWSAccessKeyID",
		"AWS_SECRET_ACCESS_KEY":             "AWSSecretAccessKey",
		"DIPLOMAS_S3_BUCKET":                "DiplomasS3Bucket",
	}

	for envKey, confKey := range envToConf {
		if value := os.Getenv(envKey); value != "" {
			_ = beego.AppConfig.Set(confKey, value)
		}
	}

	if portValue := os.Getenv("DIPLOMAS_MID_HTTP_PORT"); portValue != "" {
		port, err := strconv.Atoi(portValue)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("DIPLOMAS_MID_HTTP_PORT must be an integer between 1 and 65535")
		}
		beego.BConfig.Listen.HTTPPort = port
	}

	return nil
}
