package external_api

import (
	"net/http"
	"time"
)

var ApiClient = &http.Client{
	Timeout: 10 * time.Second,
}
