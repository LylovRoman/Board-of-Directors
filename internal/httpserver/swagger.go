package httpserver

import (
	"net/http"

	swgui "github.com/swaggest/swgui/v5"
)

func swaggerUI() http.Handler {
	return swgui.New(
		"BoardOfDirectors API",
		"/openapi.yaml",
		"/",
	)
}
