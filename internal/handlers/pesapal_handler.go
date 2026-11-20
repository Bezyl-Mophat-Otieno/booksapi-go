package handlers

import (
	"booksapi/internal/external_api"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type PesapalHandler struct {
	client         *http.Client
	consumerKey    string
	consumerSecret string
	pesapalBaseUrl string
}

func NewPesapalHandler(client *http.Client, consumerKey string, consumerSecret string, pesapalBaseUrl string) *PesapalHandler {
	return &PesapalHandler{client: external_api.ApiClient, consumerKey: consumerKey, consumerSecret: consumerSecret, pesapalBaseUrl: pesapalBaseUrl}
}

func (p *PesapalHandler) getRequestToken() (string, error) {
	body, err := json.Marshal(PesapalAuthTokenRequest{
		ConsumerKey:    p.consumerKey,
		ConsumerSecret: p.consumerSecret,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, p.pesapalBaseUrl+"/Auth/RequestToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)

	if err != nil {
		return "", fmt.Errorf("pesapal auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pesapal auth returned http %d", resp.StatusCode)
	}

	var tokenResp PesapalAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding pesapal auth response: %w", err)
	}

	if tokenResp.Error != nil {
		return "", fmt.Errorf("pesapal auth error: %s (%s)", tokenResp.Error.Message, tokenResp.Error.Code)
	}
	if tokenResp.Token == "" {
		return "", errors.New("pesapal returned an empty token")
	}

	return tokenResp.Token, nil
}

func (p *PesapalHandler) RegisterIPN(w http.ResponseWriter, r *http.Request) {
	var pesapalIPNRegistrationRequest PesapalIPNRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&pesapalIPNRegistrationRequest); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	requestJsonPayload, err := json.Marshal(pesapalIPNRegistrationRequest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Invalid Json Body")
		return
	}
	authToken, err := p.getRequestToken()
	if err != nil || strings.TrimSpace(authToken) == "" {
		writeError(w, http.StatusBadGateway, "Failed to authenticate with pesapal")
		return
	}
	req, err := http.NewRequest(http.MethodPost, p.pesapalBaseUrl+"/URLSetup/RegisterIPN", bytes.NewReader(requestJsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	response, err := p.client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to register IPN with Pesapal")
		return
	}
	defer response.Body.Close()
	var pesapalIPNRegistrationResponse PesapalIPNRegistrationResponse
	err = json.NewDecoder(response.Body).Decode(&pesapalIPNRegistrationResponse)

	writeJson(w, http.StatusOK, pesapalIPNRegistrationResponse)
}
func (p *PesapalHandler) GetIPNList(w http.ResponseWriter, r *http.Request) {
	authToken, err := p.getRequestToken()
	if err != nil || strings.TrimSpace(authToken) == "" {
		writeError(w, http.StatusBadGateway, "Failed to authenticate with pesapal")
		return
	}
	req, err := http.NewRequest(http.MethodGet, p.pesapalBaseUrl+"/URLSetup/GetIpnList", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)

	response, err := p.client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to register IPN with Pesapal")
		return
	}
	defer response.Body.Close()
	var pesapalIPNList []PesapalIPNListResponse
	err = json.NewDecoder(response.Body).Decode(&pesapalIPNList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Invalid JSON Response")
		return
	}
	writeJson(w, http.StatusOK, pesapalIPNList)
}

func (p *PesapalHandler) SubmitOrderRequest() {

}

func (p *PesapalHandler) GetTransactionStatus() {

}

func (p *PesapalHandler) IPNUrlHandler(w http.ResponseWriter, r *http.Request) {

}
