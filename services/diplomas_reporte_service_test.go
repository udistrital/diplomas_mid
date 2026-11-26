package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func configReporteTest(t *testing.T, key, value string) {
	t.Helper()
	old, _ := beego.AppConfig.String(key)
	if err := beego.AppConfig.Set(key, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { beego.AppConfig.Set(key, old) })
}

func TestReporteExcelConservaTextoYEnlacesOriginales(t *testing.T) {
	fila := make([]string, len(columnasReporteDiplomas))
	fila[0] = "https://verify.example.test/?token=original%2Btoken&context=diploma"
	fila[1] = "Facultad_14/ceremonia/diploma.pdf"
	fila[5] = "001234567890123456789"
	fila[6] = "=HYPERLINK(\"https://invalid.test\")"
	fila[8] = "000123"
	fila[10] = "Ingeniería & título <original>"
	data, err := crearReporteDiplomasExcel([][]string{fila})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = b
		decoder := xml.NewDecoder(bytes.NewReader(b))
		for {
			_, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("XML %s: %v", f.Name, err)
			}
		}
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Type  string `xml:"t,attr"`
				Value struct {
					Text string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(parts["xl/worksheets/sheet1.xml"], &sheet); err != nil {
		t.Fatal(err)
	}
	if len(sheet.Rows) != 2 {
		t.Fatalf("filas=%d", len(sheet.Rows))
	}
	for i, cell := range sheet.Rows[1].Cells {
		if cell.Type != "inlineStr" || cell.Value.Text != fila[i] {
			t.Fatalf("celda %d no conserva texto: %+v", i, cell)
		}
	}
	if bytes.Contains(parts["xl/worksheets/sheet1.xml"], []byte("<f>")) {
		t.Fatal("formula no permitida")
	}
	var rels struct {
		Items []struct {
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(parts["xl/worksheets/_rels/sheet1.xml.rels"], &rels); err != nil {
		t.Fatal(err)
	}
	if len(rels.Items) != 1 || rels.Items[0].Target != fila[0] || rels.Items[0].Mode != "External" {
		t.Fatalf("enlace original alterado: %+v", rels)
	}
	if path := os.Getenv("DIPLOMAS_TEST_XLSX"); path != "" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReporteVerificacionesLotesYAutenticacion(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-institutional-token" {
			t.Error("no se propagó Authorization a Firma")
		}
		if r.Method != "POST" || r.URL.Path != "/api/v2/firma_electronica/reporte" || r.URL.RawQuery != "" {
			t.Errorf("peticion inesperada %s %s", r.Method, r.URL)
		}
		var payload struct {
			UUIDs []string `json:"uuid_documentos"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.UUIDs) > 100 {
			t.Error("autenticacion o lote incorrecto")
		}
		firmas := []verificacionReporteDiploma{}
		for _, id := range payload.UUIDs {
			var num int64
			fmt.Sscanf(id, "document-%d", &num)
			firmas = append(firmas, verificacionReporteDiploma{UUID: id, DocumentoID: num, FirmaID: "signature", Repositorio: "diplomas", Activo: true, URL: "https://verify.example.test/?token=" + id + "&original=true"})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"firmas": firmas})
	}))
	defer server.Close()
	configReporteTest(t, "FirmaElectronicaMidURL", server.URL+"/api")
	items := []diplomaZIPItem{}
	for i := int64(1); i <= 101; i++ {
		items = append(items, diplomaZIPItem{UUID: fmt.Sprintf("document-%d", i), Documento: &documentoDigitalResponse{ID: i}})
	}
	result, err := consultarVerificacionesReporte(context.WithValue(context.Background(), "Authorization", "Bearer test-institutional-token"), items)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 101 || calls != 2 || result["document-1"].URL != "https://verify.example.test/?token=document-1&original=true" {
		t.Fatalf("resultado incompleto %d llamadas %d", len(result), calls)
	}
}

func TestReporteRechazaMetadatosIncompletosOInconsistentes(t *testing.T) {
	original := verificacionReporteDiploma{UUID: "document", DocumentoID: 7, FirmaID: "signature", Repositorio: "diplomas", Activo: true, URL: "https://verify.example.test/?token=original"}
	for _, name := range []string{"missing", "wrong-document", "wrong-uuid", "inactive", "wrong-repository", "invalid-url", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			firma := original
			firmas := []verificacionReporteDiploma{}
			switch name {
			case "wrong-document":
				firma.DocumentoID = 8
			case "wrong-uuid":
				firma.UUID = "other"
			case "inactive":
				firma.Activo = false
			case "wrong-repository":
				firma.Repositorio = "other"
			case "invalid-url":
				firma.URL = "javascript:alert(1)"
			}
			if name != "missing" {
				firmas = append(firmas, firma)
			}
			if name == "duplicate" {
				firmas = append(firmas, firma)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"firmas": firmas})
			}))
			defer server.Close()
			configReporteTest(t, "FirmaElectronicaMidURL", server.URL+"/api")
			_, err := consultarVerificacionesReporte(context.Background(), []diplomaZIPItem{{UUID: "document", Documento: &documentoDigitalResponse{ID: 7}}})
			if err == nil {
				t.Fatal("se aceptaron metadatos invalidos")
			}
		})
	}
}

func TestReporteRechazaURLInvalida(t *testing.T) {
	configReporteTest(t, "FirmaElectronicaMidURL", "invalid")
	if _, err := consultarVerificacionesReporte(context.Background(), nil); err == nil {
		t.Fatal("URL inválida aceptada")
	}
}

func TestGenerarZIPSinReporteDescargaPDFOriginal(t *testing.T) {
	const uuid1 = "00000000-0000-0000-0000-000000000001"
	const uuid2 = "00000000-0000-0000-0000-000000000002"
	original := []byte("%PDF-1.7\noriginal-firmado\n%%EOF")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ceremonia_facultad_programa/":
			fmt.Fprint(w, `[{"id":11,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":3,"fecha_ceremonia":"2026-10-28","lugar":"Auditorio","activo":true}},{"id":12,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":4,"activo":false}},{"id":13,"facultad_id":20,"activo":true,"ceremonia_grado":{"id":5,"activo":true}}]`)
		case "/v1/ceremonia_estudiante/":
			if r.URL.Query().Get("query") != "ceremonia_facultad_programa_id:11,activo:true" {
				t.Errorf("se incluyo otra ceremonia: %s", r.URL)
			}
			fmt.Fprintf(w, `[{"id":1,"activo":true,"documento_digital":{"id":7,"uuid_documento":%q,"codigo_estudiante":2026001}},{"id":2,"activo":true,"documento_digital":{"id":7,"uuid_documento":%q,"codigo_estudiante":2026001}},{"id":3,"activo":true,"documento_digital":{"id":8,"uuid_documento":%q,"codigo_estudiante":2026002}}]`, uuid1, uuid1, uuid2)
		case "/test-bucket":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>test-bucket</Name><IsTruncated>false</IsTruncated><Contents><Key>diplomas/%s/original.pdf</Key><Size>%d</Size></Contents></ListBucketResult>`, uuid1, len(original))
		case "/test-bucket/diplomas/" + uuid1 + "/original.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(original)
		default:
			t.Errorf("endpoint inesperado: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for key, value := range map[string]string{"DiplomasCrudURL": server.URL, "FirmaElectronicaMidURL": server.URL + "/api", "OikosAPIURL": server.URL, "AWSEndpointURL": server.URL, "AWSRegion": "us-east-1", "AWSAccessKeyID": "test", "AWSSecretAccessKey": "test", "DiplomasS3Bucket": "test-bucket"} {
		configReporteTest(t, key, value)
	}
	result, err := (DiplomasZIPService{}).Generar(context.Background(), DiplomasZIPInput{FacultadID: 14, CeremoniaID: 3})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result), int64(len(result)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 {
		t.Fatalf("esperado sólo 1 PDF original, recibido %d", len(archive.File))
	}
	for _, file := range archive.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name != "Facultad_14/Ceremonia_20261028_3/20261028_Diploma_2026001.pdf" || !bytes.Equal(data, original) {
			t.Fatalf("PDF alterado: %s", file.Name)
		}
	}
}

func TestGenerarZIPIncluyeExcelSoloParaPDFOriginales(t *testing.T) {
	const uuid1 = "00000000-0000-0000-0000-000000000001"
	const uuid2 = "00000000-0000-0000-0000-000000000002"
	const qr = "https://verify.example.test/?token=exact%2Btoken&context=diploma"
	original := []byte("%PDF-1.7\noriginal-firmado\n%%EOF")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ceremonia_facultad_programa/":
			fmt.Fprint(w, `[{"id":11,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":3,"fecha_ceremonia":"2026-10-28","lugar":"Auditorio","activo":true}},{"id":12,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":4,"activo":false}},{"id":13,"facultad_id":20,"activo":true,"ceremonia_grado":{"id":5,"activo":true}}]`)
		case "/v1/ceremonia_estudiante/":
			if r.URL.Query().Get("query") != "ceremonia_facultad_programa_id:11,activo:true" {
				t.Errorf("se incluyo otra ceremonia: %s", r.URL)
			}
			fmt.Fprintf(w, `[{"id":1,"activo":true,"documento_digital":{"id":7,"uuid_documento":%q,"codigo_estudiante":2026001}},{"id":2,"activo":true,"documento_digital":{"id":7,"uuid_documento":%q,"codigo_estudiante":2026001}},{"id":3,"activo":true,"documento_digital":{"id":8,"uuid_documento":%q,"codigo_estudiante":2026002}}]`, uuid1, uuid1, uuid2)
		case "/v1/diploma_digital/":
			if r.URL.Query().Get("query") != "documento_digital_id:7" {
				t.Error("se consulto diploma sin PDF")
			}
			fmt.Fprint(w, `[{"id":70,"facultad_id":14,"vigencia":2026,"fecha_grado":"2026-10-30T00:00:00Z","nombre_estudiante":"Estudiante original","tipo_documento_estudiante":"CC","numero_documento_estudiante":"000123","titulo_otorgado":"Ingeniería","consecutivo_diploma":45,"consecutivo_facultad":12,"registro_grado":{"id":9,"numero_registro":25,"folio_registro":{"numero_folio":2,"libro_registro":{"numero_libro":4}}}}]`)
		case "/api/v2/firma_electronica/reporte":
			var payload struct {
				UUIDs []string `json:"uuid_documentos"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if len(payload.UUIDs) != 1 || payload.UUIDs[0] != uuid1 {
				t.Errorf("consulta incorrecta: cantidad %d", len(payload.UUIDs))
			}
			json.NewEncoder(w).Encode(map[string]any{"firmas": []verificacionReporteDiploma{{FirmaID: "signature", DocumentoID: 7, UUID: uuid1, Repositorio: "diplomas", Activo: true, URL: qr}}})
		case "/v1/dependencia/14":
			fmt.Fprint(w, `{"Id":14,"Nombre":"Facultad de Ingeniería"}`)
		case "/test-bucket":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>test-bucket</Name><IsTruncated>false</IsTruncated><Contents><Key>diplomas/%s/original.pdf</Key><Size>%d</Size></Contents></ListBucketResult>`, uuid1, len(original))
		case "/test-bucket/diplomas/" + uuid1 + "/original.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(original)
		default:
			t.Errorf("endpoint inesperado: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for key, value := range map[string]string{"DiplomasCrudURL": server.URL, "FirmaElectronicaMidURL": server.URL + "/api", "OikosAPIURL": server.URL, "AWSEndpointURL": server.URL, "AWSRegion": "us-east-1", "AWSAccessKeyID": "test", "AWSSecretAccessKey": "test", "DiplomasS3Bucket": "test-bucket"} {
		configReporteTest(t, key, value)
	}
	result, err := (DiplomasZIPService{}).Generar(context.Background(), DiplomasZIPInput{FacultadID: 14, CeremoniaID: 3, IncluirReporte: true})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result), int64(len(result)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("esperado 1 PDF + Excel, recibido %d", len(archive.File))
	}
	var excel []byte
	for _, file := range archive.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == "reporte_diplomas.xlsx" {
			excel = data
			continue
		}
		if file.Name != "Facultad_14/Ceremonia_20261028_3/20261028_Diploma_2026001.pdf" || !bytes.Equal(data, original) {
			t.Fatalf("PDF alterado: %s", file.Name)
		}
	}
	workbook, err := zip.NewReader(bytes.NewReader(excel), int64(len(excel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range workbook.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		r, _ := file.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		var sheet struct {
			Rows []struct {
				Cells []struct {
					Value struct {
						Text string `xml:"t"`
					} `xml:"is"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if err := xml.Unmarshal(data, &sheet); err != nil {
			t.Fatal(err)
		}
		if len(sheet.Rows) != 2 {
			t.Fatalf("filas sin PDF o duplicadas: %d", len(sheet.Rows))
		}
		row := sheet.Rows[1].Cells
		for col, want := range map[int]string{0: qr, 1: "Facultad_14/Ceremonia_20261028_3/20261028_Diploma_2026001.pdf", 4: "70", 6: "Estudiante original", 8: "000123", 20: "2026-10-30", 21: "45", 24: "25", 25: "002", 26: "4"} {
			if row[col].Value.Text != want {
				t.Errorf("columna %d = %q, esperado %q", col, row[col].Value.Text, want)
			}
		}
	}
}
