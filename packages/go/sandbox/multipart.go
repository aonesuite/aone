package sandbox

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"strings"
)

type multipartFileWriter struct {
	w *multipart.Writer
}

func newMultipartWriter(w io.Writer) *multipartFileWriter {
	return &multipartFileWriter{w: multipart.NewWriter(w)}
}

func (m *multipartFileWriter) contentType() string {
	return m.w.FormDataContentType()
}

func (m *multipartFileWriter) writeFile(fieldName, fileName string, data []byte) error {
	part, err := m.w.CreateFormFile(fieldName, filepath.Base(fileName))
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func (m *multipartFileWriter) writeFileFullPath(fieldName, fullPath string, data []byte) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldName), escapeQuotes(fullPath)))
	h.Set("Content-Type", "application/octet-stream")
	part, err := m.w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (m *multipartFileWriter) close() error {
	return m.w.Close()
}
