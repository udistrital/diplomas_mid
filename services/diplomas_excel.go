package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

var columnasReporteDiplomas = []string{
	"URL de verificación", "Ruta PDF en ZIP", "UUID del documento", "ID documento", "ID diploma", "Código estudiante",
	"Nombre estudiante", "Tipo documento", "Número documento", "Municipio expedición", "Título otorgado", "ID facultad", "Facultad",
	"ID programa", "Programa", "ID período", "Vigencia", "ID ceremonia", "Ceremonia", "Fecha ceremonia", "Fecha grado",
	"Consecutivo diploma", "Consecutivo facultad", "ID registro grado", "Número registro", "Folio", "Libro", "ID firma electrónica", "Código autenticidad", "SHA-256 PDF",
}

type textoCeldaExcel struct {
	Espacio string `xml:"xml:space,attr"`
	Valor   string `xml:",chardata"`
}
type contenidoCeldaExcel struct {
	Texto textoCeldaExcel `xml:"t"`
}
type celdaExcel struct {
	Referencia string              `xml:"r,attr"`
	Tipo       string              `xml:"t,attr"`
	Estilo     int                 `xml:"s,attr,omitempty"`
	Contenido  contenidoCeldaExcel `xml:"is"`
}
type filaExcel struct {
	XMLName xml.Name     `xml:"row"`
	Numero  int          `xml:"r,attr"`
	Celdas  []celdaExcel `xml:"c"`
}
type enlaceExcel struct {
	Referencia string `xml:"ref,attr"`
	Relacion   string `xml:"r:id,attr"`
}
type enlacesExcel struct {
	XMLName xml.Name      `xml:"hyperlinks"`
	Enlaces []enlaceExcel `xml:"hyperlink"`
}
type relacionExcel struct {
	ID      string `xml:"Id,attr"`
	Tipo    string `xml:"Type,attr"`
	Destino string `xml:"Target,attr"`
	Modo    string `xml:"TargetMode,attr,omitempty"`
}
type relacionesExcel struct {
	XMLName    xml.Name        `xml:"Relationships"`
	Namespace  string          `xml:"xmlns,attr"`
	Relaciones []relacionExcel `xml:"Relationship"`
}

func columnaExcel(numero int) string {
	resultado := ""
	for numero > 0 {
		numero--
		resultado = string(rune('A'+numero%26)) + resultado
		numero /= 26
	}
	return resultado
}

func textoSeguroExcel(value string) string {
	return strings.Map(func(r rune) rune {
		if r == 9 || r == 10 || r == 13 || (r >= 0x20 && r <= 0xd7ff) || (r >= 0xe000 && r <= 0xfffd) || (r >= 0x10000 && r <= 0x10ffff) {
			return r
		}
		return '\uFFFD'
	}, strings.ToValidUTF8(value, "\uFFFD"))
}

// XLSX Open XML con la biblioteca estándar. Todas las celdas son texto literal:
// conserva identificaciones y códigos, y evita interpretar metadatos como fórmulas.
func crearReporteDiplomasExcel(filas [][]string) ([]byte, error) {
	if len(filas) > 1048575 {
		return nil, fmt.Errorf("%w: el reporte excede el límite de filas Excel", ErrInvalidInput)
	}
	var hoja bytes.Buffer
	hoja.WriteString(xml.Header + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="2" width="60" customWidth="1"/><col min="3" max="30" width="24" customWidth="1"/></cols><sheetData>`)
	encoder := xml.NewEncoder(&hoja)
	enlaces := enlacesExcel{}
	relaciones := relacionesExcel{Namespace: "http://schemas.openxmlformats.org/package/2006/relationships"}
	escribirFila := func(numero int, valores []string) error {
		if len(valores) != len(columnasReporteDiplomas) {
			return fmt.Errorf("%w: columnas del reporte inconsistentes", ErrInvalidInput)
		}
		fila := filaExcel{Numero: numero}
		for col, texto := range valores {
			estilo := 0
			if numero == 1 {
				estilo = 1
			} else if col == 0 && texto != "" {
				estilo = 2
			}
			fila.Celdas = append(fila.Celdas, celdaExcel{Referencia: fmt.Sprintf("%s%d", columnaExcel(col+1), numero), Tipo: "inlineStr", Estilo: estilo, Contenido: contenidoCeldaExcel{Texto: textoCeldaExcel{Espacio: "preserve", Valor: textoSeguroExcel(texto)}}})
		}
		return encoder.Encode(fila)
	}
	if err := escribirFila(1, columnasReporteDiplomas); err != nil {
		return nil, err
	}
	for i, fila := range filas {
		if err := escribirFila(i+2, fila); err != nil {
			return nil, err
		}
		if fila[0] != "" {
			id := fmt.Sprintf("rId%d", len(enlaces.Enlaces)+1)
			enlaces.Enlaces = append(enlaces.Enlaces, enlaceExcel{Referencia: fmt.Sprintf("A%d", i+2), Relacion: id})
			relaciones.Relaciones = append(relaciones.Relaciones, relacionExcel{ID: id, Tipo: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink", Destino: fila[0], Modo: "External"})
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	hoja.WriteString(fmt.Sprintf(`</sheetData><autoFilter ref="A1:%s%d"/>`, columnaExcel(len(columnasReporteDiplomas)), len(filas)+1))
	if len(enlaces.Enlaces) > 0 {
		if err := encoder.Encode(enlaces); err != nil {
			return nil, err
		}
		if err := encoder.Flush(); err != nil {
			return nil, err
		}
	}
	hoja.WriteString("</worksheet>")
	rels, err := xml.Marshal(relaciones)
	if err != nil {
		return nil, err
	}
	parts := []struct{ nombre, contenido string }{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView activeTab="0"/></bookViews><sheets><sheet name="Diplomas" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/styles.xml", `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="3"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font><font><u/><color rgb="FF0563C1"/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/><xf numFmtId="0" fontId="2" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`},
		{"xl/worksheets/sheet1.xml", hoja.String()},
	}
	if len(enlaces.Enlaces) > 0 {
		parts = append(parts, struct{ nombre, contenido string }{"xl/worksheets/_rels/sheet1.xml.rels", string(rels)})
	}
	var result bytes.Buffer
	zw := zip.NewWriter(&result)
	for _, part := range parts {
		entry, err := zw.Create(part.nombre)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(entry, part.contenido); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
