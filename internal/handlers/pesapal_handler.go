package handlers

import (
	"booksapi/internal/store"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const pesapalCurrency = "KES"

type PesapalHandler struct {
	client             *http.Client
	consumerKey        string
	consumerSecret     string
	pesapalBaseUrl     string
	pesapalIPNID       string
	pesapalCallbackUrl string
	orderStore         *store.OrderStore
	transactionStore   *store.TransactionStore
}

func NewPesapalHandler(client *http.Client,
	consumerKey string,
	consumerSecret string,
	pesapalBaseUrl string,
	pesapalIPNID string,
	pesapalCallbackUrl string,
	orderStore *store.OrderStore,
	transactionStore *store.TransactionStore,
) *PesapalHandler {
	return &PesapalHandler{
		client:             client,
		consumerKey:        consumerKey,
		consumerSecret:     consumerSecret,
		pesapalBaseUrl:     pesapalBaseUrl,
		pesapalIPNID:       pesapalIPNID,
		pesapalCallbackUrl: pesapalCallbackUrl,
		orderStore:         orderStore,
		transactionStore:   transactionStore,
	}
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

func (p *PesapalHandler) SubmitOrderRequest(w http.ResponseWriter, r *http.Request) {
	orderId, ok := parseOrderId(w, r)
	if !ok {
		return
	}
	order, err := p.orderStore.GetById(orderId)
	if err != nil {
		writeTransactionError(w, err, "fetch order")
		return
	}
	if order.Status != store.StatusPlaced {
		writeError(w, http.StatusConflict,
			fmt.Sprintf("Only orders with status '%s' can be paid; this order is '%s'", store.StatusPlaced, order.Status))
		return
	}


	attempts, err := p.transactionStore.CountByOrder(orderId)
	if err != nil {
		writeTransactionError(w, err, "count payment attempts")
		return
	}
	merchantReference := fmt.Sprintf("%d-%d", order.Id, attempts+1)

	pesapalOrderRequest := PesapalOrderCreationRequest{
		OrderId:        merchantReference,
		Currency:       pesapalCurrency,
		Amount:         float64(order.TotalCents) / 100,
		Description:    fmt.Sprintf("Payment for order #%d", order.Id),
		CallbackUrl:    p.pesapalCallbackUrl,
		NotificationId: p.pesapalIPNID,
		BillingAddress: PesapalCustomerBillingAddress{
			PhoneNumber:  order.CustomerPhone,
			EmailAddress: order.CustomerEmail,
			FirstName:    order.CustomerFirstName,
			LastName:     order.CustomerLastName,
		},
	}
	jsonPayload, err := json.Marshal(pesapalOrderRequest)
	if err != nil {
		log.Println("encoding pesapal order request:", err)
		writeError(w, http.StatusInternalServerError, "Could not build the Pesapal payment request")
		return
	}

	authToken, err := p.getRequestToken()
	if err != nil {
		log.Println(err)
		writeError(w, http.StatusBadGateway, "Failed to authenticate with pesapal")
		return
	}
	req, err := http.NewRequest(http.MethodPost, p.pesapalBaseUrl+"/Transactions/SubmitOrderRequest", bytes.NewReader(jsonPayload))
	if err != nil {
		log.Println("building pesapal order request:", err)
		writeError(w, http.StatusInternalServerError, "Could not build the Pesapal payment request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	response, err := p.client.Do(req)
	if err != nil {
		log.Println("pesapal order request failed:", err)
		writeError(w, http.StatusBadGateway, "Failed to submit an order request to pesapal")
		return
	}
	defer response.Body.Close()

	var pesapalResponse PesapalOrderCreationResponse
	if err := json.NewDecoder(response.Body).Decode(&pesapalResponse); err != nil {
		log.Printf("decoding pesapal order response (http %d): %v", response.StatusCode, err)
		writeError(w, http.StatusBadGateway, "Invalid response from Pesapal")
		return
	}
	if response.StatusCode != http.StatusOK || pesapalResponse.Error.present() ||
		strings.TrimSpace(pesapalResponse.OrderTrackingID) == "" {
		log.Printf("pesapal rejected order request %s (http %d): %+v", merchantReference, response.StatusCode, pesapalResponse.Error)
		message := "Pesapal rejected the payment request"
		if pesapalResponse.Error.present() && pesapalResponse.Error.Message != "" {
			message += ": " + pesapalResponse.Error.Message
		}
		writeError(w, http.StatusBadGateway, message)
		return
	}

	transaction := store.Transaction{
		OrderID:           order.Id,
		MerchantReference: merchantReference,
		OrderTrackingID:   pesapalResponse.OrderTrackingID,
		Status:            store.TransactionPending,
		AmountCents:       order.TotalCents,
		Currency:          pesapalCurrency,
	}
	if err := p.transactionStore.Create(&transaction); err != nil {
		log.Printf("recording transaction for order %d (merchant reference %s, tracking id %s): %v",
			order.Id, merchantReference, pesapalResponse.OrderTrackingID, err)
		writeError(w, http.StatusInternalServerError, "Failed to record the transaction for the Pesapal payment request")
		return
	}

	writeJson(w, http.StatusCreated, PaymentRequestResponse{
		TransactionID:     transaction.Id,
		OrderID:           order.Id,
		MerchantReference: transaction.MerchantReference,
		OrderTrackingID:   transaction.OrderTrackingID,
		RedirectUrl:       pesapalResponse.RedirectUrl,
		Status:            string(transaction.Status),
	})
}

func (p *PesapalHandler) GetTransactionStatus() {

}

func (p *PesapalHandler) IPNUrlHandler(w http.ResponseWriter, r *http.Request) {

}
