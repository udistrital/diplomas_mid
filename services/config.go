package services

import (
	"os"
	"path/filepath"
	"strings"

	beego "github.com/beego/beego/v2/server/web"
)

func configString(confKey, envKey string) string {
	value, _ := beego.AppConfig.String(confKey)
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "${") {
		value = strings.TrimSpace(os.Getenv(envKey))
	}
	return strings.TrimRight(value, "/")
}

// configEndpoint appends the API contract path to the configured service base URL.
func configEndpoint(confKey, envKey, path string) string {
	base := configString(confKey, envKey)
	if base == "" {
		return ""
	}
	return base + path
}

func firmaElectronicaMidURL() string {
	return configString("FirmaElectronicaMidURL", "FIRMA_ELECTRONICA_MID_URL")
}

func diplomasCrudURL() string {
	return configEndpoint("DiplomasCrudURL", "DIPLOMAS_CRUD_URL", "/v1")
}

func administrativaAmazonAPIURL() string {
	return configString("AdministrativaAmazonAPIURL", "ADMINISTRATIVA_AMAZON_API_URL")
}

func documentosCrudURL() string {
	return configEndpoint("DocumentosCrudURL", "DOCUMENTOS_CRUD_URL", "/v1/gestor_documental")
}

func gestorDocumentalURL() string {
	return configEndpoint("GestorDocumentalURL", "GESTOR_DOCUMENTAL_URL", "/v1/document")
}

func pazYSalvosMidURL() string {
	base := configString("PazYSalvosMidURL", "PAZ_Y_SALVOS_MID_URL")
	if base == "" {
		return ""
	}
	return strings.TrimSuffix(base, "/v1") + "/v1"
}

func academicaCoreServiceURL() string {
	return configString("AcademicaCoreServiceURL", "ACADEMICA_CORE_SERVICE_URL")
}

func parametrosCrudURL() string {
	return strings.TrimSuffix(configString("ParametrosCrudURL", "PARAMETROS_CRUD_URL"), "/v1") + "/v1"
}

func academicaPruebasAPIURL() string {
	value := configString("AcademicaPruebasAPIURL", "ACADEMICA_PRUEBAS_API_URL")
	if value == "" {
		value = configString("FacultadSecretariaURL", "FACULTAD_SECRETARIA_URL")
		value = strings.TrimSuffix(value, "/facultad_secretaria")
	}
	return value
}

func homologacionDependenciasAPIURL() string {
	value := configString("HomologacionDependenciasAPIURL", "HOMOLOGACION_DEPENDENCIAS_API_URL")
	if value == "" {
		value = configString("FacultadOikosGedepURL", "FACULTAD_OIKOS_GEDEP_URL")
		value = strings.TrimSuffix(value, "/facultad_oikos_gedep")
	}
	return value
}

func oikosAPIURL() string {
	value := configString("OikosAPIURL", "OIKOS_API_URL")
	if value == "" {
		value = configString("OikosDependenciaURL", "OIKOS_DEPENDENCIA_URL")
		value = strings.TrimSuffix(value, "/dependencia")
	}
	return value
}

func diplomaInstitutionalLogoPath() string {
	value := configString("DiplomaInstitutionalLogoPath", "DIPLOMA_INSTITUTIONAL_LOGO_PATH")
	if value == "" {
		return resolveLocalPath("assets/images/logo-udistrital-escudo.svg")
	}
	return resolveLocalPath(value)
}

func diplomaCambriaFontPath() string {
	value := configString("DiplomaCambriaFontPath", "DIPLOMA_CAMBRIA_FONT_PATH")
	if value == "" {
		return resolveLocalPath("assets/fonts/Cambria.ttf")
	}
	return resolveLocalPath(value)
}

func diplomaCambriaBoldFontPath() string {
	value := configString("DiplomaCambriaBoldFontPath", "DIPLOMA_CAMBRIA_BOLD_FONT_PATH")
	if value == "" {
		return resolveLocalPath("assets/fonts/Cambriab.ttf")
	}
	return resolveLocalPath(value)
}

func diplomaEngraversFontPath() string {
	value := configString("DiplomaEngraversFontPath", "DIPLOMA_ENGRAVERS_FONT_PATH")
	if value == "" {
		return resolveLocalPath("assets/fonts/EngraversOldEnglishBold.otf")
	}
	return resolveLocalPath(value)
}

func resolveLocalPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	candidates := []string{
		path,
		filepath.Join("..", path),
		filepath.Join("..", "..", path),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}

func awsRegion() string {
	value := configString("AWSRegion", "AWS_REGION")
	if value == "" {
		return "us-east-1"
	}
	return value
}

func awsEndpointURL() string {
	return configString("AWSEndpointURL", "AWS_ENDPOINT_URL")
}

func awsAccessKeyID() string {
	return configString("AWSAccessKeyID", "AWS_ACCESS_KEY_ID")
}

func awsSecretAccessKey() string {
	return configString("AWSSecretAccessKey", "AWS_SECRET_ACCESS_KEY")
}

func diplomasS3Bucket() string {
	return configString("DiplomasS3Bucket", "DIPLOMAS_S3_BUCKET")
}
