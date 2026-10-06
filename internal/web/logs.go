package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/logbuf"
)

type logRow struct {
	Time    string
	Level   string
	Message string
	Attrs   string
}

func renderLogs(w http.ResponseWriter, buf *logbuf.Buffer) {
	entries := buf.Entries()
	rows := make([]logRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, logRow{
			Time:    e.Time.Format("2006-01-02 15:04:05"),
			Level:   e.Level.String(),
			Message: e.Message,
			Attrs:   e.Attrs,
		})
	}
	renderPartial(w, "logs_fragment", map[string]any{"Entries": rows})
}

func logsHandler(buf *logbuf.Buffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderLogs(w, buf)
	}
}

func logsClearHandler(buf *logbuf.Buffer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buf.Clear()
		renderLogs(w, buf)
	}
}
