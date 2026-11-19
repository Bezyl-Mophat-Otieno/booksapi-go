package handlers

import (
	"booksapi/internal/external_api"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

type PesapalHandler struct {
	client *http.Client
}

var baseUrl = os.Getenv("PESAPAL_API")

func NewPesapalHandler(client *http.Client) *PesapalHandler {
	return &PesapalHandler{client: external_api.ApiClient}
}

// POST /api/Auth/RequestToken

func (p *PesapalHandler) getRequestToken() (*string, error) {
	var payload = map[string]string{
		"consumer_key":    os.Getenv("CONSUMER_KEY"),
		"consumer_secret": os.Getenv("CONSUMER_SECRET"),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", baseUrl+"/Auth/RequestToken", bytes.NewReader(body))
	response, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return nil, nil
}

// POST /api/URLSetup/RegisterIPN
func (p *PesapalHandler) RegisterIPN() {

}

// GET /api/URLSetup/GetIpnList
func (p *PesapalHandler) GetIPNList() {

}

// POST /api/Transactions/SubmitOrderRequest
func (p *PesapalHandler) SubmitOrderRequest() {

}

// GET /api/Transactions/GetTransactionStatus?orderTrackingId={id}
func (p *PesapalHandler) GetTransactionStatus() {

}
