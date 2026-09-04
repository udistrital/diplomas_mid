package routers

import (
	"github.com/astaxie/beego"

	"github.com/udistrital/diplomas_mid/controllers"
)

func init() {
	api := beego.NewNamespace("/v1",
		beego.NSNamespace("/documento_digital",
			beego.NSRouter("/:id/firmar", &controllers.FirmaDiplomaController{}, "post:Firmar"),
		),
		beego.NSNamespace("/firmantes",
			beego.NSRouter("/rol-activo", &controllers.FirmanteController{}, "get:RolActivo"),
			beego.NSRouter("/firma", &controllers.FirmaFirmanteController{}, "post:SubirFirma"),
		),
	)

	beego.AddNamespace(api)
}
