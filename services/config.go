package services

import (
	"strings"

	"github.com/astaxie/beego"
)

func firmaElectronicaMidURL() string {
	return strings.TrimRight(beego.AppConfig.String("FirmaElectronicaMidURL"), "/")
}

func diplomasCrudURL() string {
	return strings.TrimRight(beego.AppConfig.String("DiplomasCrudURL"), "/")
}

func administrativaAmazonAPIURL() string {
	return strings.TrimRight(beego.AppConfig.String("AdministrativaAmazonAPIURL"), "/")
}

func documentosCrudGestorDocumentalURL() string {
	return strings.TrimRight(beego.AppConfig.String("DocumentosCrudGestorDocumentalURL"), "/")
}

func awsRegion() string {
	value := strings.TrimSpace(beego.AppConfig.String("AWSRegion"))
	if value == "" {
		return "us-east-1"
	}
	return value
}

func awsEndpointURL() string {
	return strings.TrimSpace(beego.AppConfig.String("AWSEndpointURL"))
}

func awsAccessKeyID() string {
	return strings.TrimSpace(beego.AppConfig.String("AWSAccessKeyID"))
}

func awsSecretAccessKey() string {
	return strings.TrimSpace(beego.AppConfig.String("AWSSecretAccessKey"))
}

func diplomasS3Bucket() string {
	return strings.TrimSpace(beego.AppConfig.String("DiplomasS3Bucket"))
}
