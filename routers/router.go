package routers

import (
	beego "github.com/beego/beego/v2/server/web"

	"github.com/udistrital/diplomas_mid/controllers"
)

func init() {
	api := beego.NewNamespace("/v1",
		beego.NSNamespace("/documento_digital",
			beego.NSRouter("/:id/diploma/preview", &controllers.DiplomaPreviewController{}, "get:Generar;post:Preparar"),
			beego.NSRouter("/:id/firmar", &controllers.FirmaDiplomaController{}, "post:Firmar"),
			beego.NSRouter("/:id/firma", &controllers.FirmaDiplomaController{}, "delete:Anular"),
		),
		beego.NSNamespace("/diplomas",
			beego.NSRouter("/periodos-academicos", &controllers.EstudianteGradoController{}, "get:PeriodosAcademicos"),
			beego.NSRouter("/estudiantes-aprobados", &controllers.EstudianteGradoController{}, "get:AprobadosPorFacultad"),
			beego.NSRouter("/estudiantes-aprobados/documentos", &controllers.EstudianteGradoController{}, "post:CrearDocumentosAprobados"),
			beego.NSRouter("/registro-control/archivo", &controllers.DiplomasZIPController{}, "get:Descargar"),
			beego.NSRouter("/registro-control/opciones", &controllers.DiplomasZIPController{}, "get:Opciones"),
		),
		beego.NSNamespace("/firmantes",
			beego.NSRouter("/rol-activo", &controllers.FirmanteController{}, "get:RolActivo"),
			beego.NSRouter("/firma", &controllers.FirmaFirmanteController{}, "post:SubirFirma"),
		),
		beego.NSNamespace("/ceremonias",
			beego.NSRouter("/", &controllers.CeremoniaController{}, "get:Listar;post:Crear"),
			beego.NSRouter("/parametrizacion-registro", &controllers.CeremoniaController{}, "get:ObtenerParametrizacionRegistro;post:GuardarParametrizacionRegistro"),
			beego.NSRouter("/programas/:programa_id/estudiantes", &controllers.CeremoniaController{}, "get:EstudiantesPrograma"),
			beego.NSRouter("/decano", &controllers.CeremoniaController{}, "get:ListarDecano"),
			beego.NSRouter("/secretaria-general", &controllers.CeremoniaController{}, "get:ListarSecretariaGeneral"),
			beego.NSRouter("/rector", &controllers.CeremoniaController{}, "get:ListarRector"),
			beego.NSRouter("/:id/firmas", &controllers.CeremoniaController{}, "get:ListarFirmas;post:FirmarMasivamente"),
			beego.NSRouter("/:id/firmas-decano", &controllers.CeremoniaController{}, "get:ListarFirmasDecano;post:FirmarMasivamenteDecano"),
			beego.NSRouter("/:id/firmas-secretaria-general", &controllers.CeremoniaController{}, "get:ListarFirmasSecretariaGeneral;post:FirmarMasivamenteSecretariaGeneral"),
			beego.NSRouter("/:id/firmas-rector", &controllers.CeremoniaController{}, "get:ListarFirmasRector;post:FirmarMasivamenteRector"),
			beego.NSRouter("/:id/asignar", &controllers.CeremoniaController{}, "post:Asignar"),
			beego.NSRouter("/:id/programa", &controllers.CeremoniaController{}, "delete:QuitarPrograma"),
			beego.NSRouter("/:id", &controllers.CeremoniaController{}, "put:Actualizar;delete:Eliminar"),
		),
	)

	beego.AddNamespace(api)
}
