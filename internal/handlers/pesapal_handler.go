package handlers

import (
	"booksapi/internal/store"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
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

func (p *PesapalHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	orderTrackingId := strings.TrimSpace(r.URL.Query().Get("OrderTrackingId"))
	if orderTrackingId == "" {
		writeError(w, http.StatusBadRequest, "OrderTrackingId is required")
		return
	}
	transactionStatus, err := p.fetchTransactionStatus(orderTrackingId)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJson(w, http.StatusOK, transactionStatus)
}

func (p *PesapalHandler) fetchTransactionStatus(trackingId string) (PesapalTransactionStatusResponse, error) {
	var status PesapalTransactionStatusResponse

	authToken, err := p.getRequestToken()
	if err != nil {
		return status, err
	}
	req, err := http.NewRequest(http.MethodGet,
		p.pesapalBaseUrl+"/Transactions/GetTransactionStatus?orderTrackingId="+url.QueryEscape(trackingId), nil)
	if err != nil {
		return status, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	response, err := p.client.Do(req)
	if err != nil {
		return status, fmt.Errorf("pesapal transaction status request failed: %w", err)
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		return status, fmt.Errorf("decoding pesapal transaction status (http %d): %w", response.StatusCode, err)
	}
	if response.StatusCode != http.StatusOK || status.Error.present() ||
		(status.Status != "" && status.Status != "200") {
		return status, fmt.Errorf("pesapal transaction status error (http %d, status %s): %+v",
			response.StatusCode, status.Status, status.Error)
	}
	if status.StatusCode == nil {
		return status, errors.New("pesapal transaction status response has no status_code")
	}
	return status, nil
}

func toStatusUpdate(s PesapalTransactionStatusResponse) store.TransactionStatusUpdate {
	u := store.TransactionStatusUpdate{
		StatusCode:               s.StatusCode,
		PaymentStatusDescription: nilIfBlank(s.PaymentStatusDescription),
		ConfirmationCode:         nilIfBlank(s.ConfirmationCode),
		PaymentMethod:            nilIfBlank(s.PaymentMethod),
		PaymentAccount:           nilIfBlank(s.PaymentAccount),
	}
	switch *s.StatusCode {
	case 1:
		u.Status = store.TransactionCompleted
	case 2:
		u.Status = store.TransactionFailed
	case 3:
		u.Status = store.TransactionReversed
	default:
		u.Status = store.TransactionInvalid
		if strings.EqualFold(strings.TrimSpace(s.PaymentStatusDescription), "pending") {
			u.Status = store.TransactionPending
		}
	}
	if u.Status == store.TransactionCompleted || u.Status == store.TransactionReversed {
		paid := int64(math.Round(s.Amount * 100))
		u.PaidAmountCents = &paid
		u.PaidCurrency = nilIfBlank(strings.ToUpper(s.Currency))
	}
	return u
}

func nilIfBlank(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (p *PesapalHandler) syncTransactionStatus(trackingId, merchantReference string) (PesapalTransactionStatusResponse, store.TransactionStatusUpdateResult, error) {
	var status PesapalTransactionStatusResponse

	if _, err := p.transactionStore.GetByReferences(trackingId, merchantReference); err != nil {
		return status, store.TransactionStatusUpdateResult{}, err
	}

	status, err := p.fetchTransactionStatus(trackingId)
	if err != nil {
		return status, store.TransactionStatusUpdateResult{}, err
	}
	if status.MerchantReference != "" && status.MerchantReference != merchantReference {
		return status, store.TransactionStatusUpdateResult{}, fmt.Errorf(
			"pesapal reports merchant reference %q for tracking id %s, expected %q",
			status.MerchantReference, trackingId, merchantReference)
	}

	result, err := p.transactionStore.ApplyStatusUpdate(trackingId, merchantReference, toStatusUpdate(status))
	return status, result, err
}

func (p *PesapalHandler) IPNUrlHandler(w http.ResponseWriter, r *http.Request) {
	var notification PesapalIPNNotification
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
			log.Println("ipn: invalid json body:", err)
		}
	} else {
		query := r.URL.Query()
		notification = PesapalIPNNotification{
			OrderTrackingId:        query.Get("OrderTrackingId"),
			OrderMerchantReference: query.Get("OrderMerchantReference"),
			OrderNotificationType:  query.Get("OrderNotificationType"),
		}
	}
	notification.OrderTrackingId = strings.TrimSpace(notification.OrderTrackingId)
	notification.OrderMerchantReference = strings.TrimSpace(notification.OrderMerchantReference)

	acknowledge := func(status int) {
		writeJson(w, status, PesapalIPNAcknowledgement{
			OrderNotificationType:  notification.OrderNotificationType,
			OrderTrackingId:        notification.OrderTrackingId,
			OrderMerchantReference: notification.OrderMerchantReference,
			Status:                 status,
		})
	}

	if notification.OrderTrackingId == "" || notification.OrderMerchantReference == "" {
		log.Printf("ipn: missing OrderTrackingId or OrderMerchantReference: %+v", notification)
		acknowledge(http.StatusBadRequest)
		return
	}

	_, result, err := p.syncTransactionStatus(notification.OrderTrackingId, notification.OrderMerchantReference)
	switch {
	case errors.Is(err, store.ErrStaleTransactionStatus):
		log.Printf("ipn: ignored out-of-order update for %s/%s: transaction is %s",
			notification.OrderTrackingId, notification.OrderMerchantReference, result.Transaction.Status)
		acknowledge(http.StatusOK)
		return
	case err != nil:
		log.Printf("ipn: processing %s/%s: %v", notification.OrderTrackingId, notification.OrderMerchantReference, err)
		acknowledge(http.StatusInternalServerError)
		return
	}

	t := result.Transaction
	switch {
	case !result.Changed:
		log.Printf("ipn: duplicate notification for transaction %d (status %s)", t.Id, t.Status)
	default:
		log.Printf("ipn: transaction %d for order %d is now %s", t.Id, t.OrderID, t.Status)
	}
	if result.OrderMarkedPaid {
		log.Printf("ipn: order %d marked as paid", t.OrderID)
	}
	if result.AmountMismatch {
		log.Printf("ipn: WARNING transaction %d completed but paid %v %v against %d %s owed; order %d left unpaid",
			t.Id, deref(t.PaidAmountCents), deref(t.PaidCurrency), t.AmountCents, t.Currency, t.OrderID)
	}

	if t.Status == store.TransactionCompleted && result.Changed && !result.OrderMarkedPaid && !result.AmountMismatch {
		log.Printf("ipn: WARNING transaction %d completed but order %d was already %s; it may need a refund",
			t.Id, t.OrderID, result.OrderStatus)
	}
	if t.Status == store.TransactionReversed && result.Changed {
		log.Printf("ipn: WARNING payment for order %d was reversed; review the order (status %s)",
			t.OrderID, result.OrderStatus)
	}

	acknowledge(http.StatusOK)
}

func deref[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}
