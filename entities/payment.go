package entities

import (
	"s7ab-platform-hyperledger/platform/core/entities"
)

type PaymentCreatePayload struct {
	Id                  string `json:"paymentId"`
	AgentId             string `json:"agent_id"`
	Amount              uint   `json:"amount"`
	Currency            string `json:"currency"`
	InternationalFlight bool   `json:"internationalFlight"`
	PaymentType         string `json:"paymentType"`
	PayerId             string `json:"payerId"`
	PayerAccount        string `json:"payerAccount"`
	PayerNumber         string `json:"payerNumber"`
	RecipientId         string `json:"recipientId"`
	RecipientAccount    string `json:"recipientAccount"`
	RecipientNumber     string `json:"recipientNumber"`
	VatIncluded         bool   `json:"vat"`
}

type Payment struct {
	Id                  string            `json:"paymentId"`
	TicketNumber        string            `json:"ticket_number"`
	State               PaymentState      `json:"state"`
	Amount              uint              `json:"amount"`
	Currency            string            `json:"currency"`
	InternationalFlight bool              `json:"internationalFlight"`
	PaymentType         string            `json:"paymentType"`
	VatIncluded         bool              `json:"vat"`
	Purpose             string            `json:"purpose"`
	Meta                map[string][]byte `json:"meta"`

	PayerOrgId         string `json:"payerOrgId"`
	PayerBankOrgId     string `json:"payerBankOrgId"`
	PayerId            string `json:"payerId"`
	PayerAccount       string `json:"payerAccount"`
	PayerNumber        string `json:"payerNumber"`
	RecipientOrgId     string `json:"recipientOrgId"`
	RecipientBankOrgId string `json:"recipientBankOrgId"`
	RecipientId        string `json:"recipientId"`
	RecipientAccount   string `json:"recipientAccount"`
	RecipientNumber    string `json:"recipientNumber"`
}

type PaymentState string

const (
	PaymentStateEmpty     PaymentState = ``
	CheckFundsRequest     PaymentState = "CheckFundsRequest"
	CheckFundsInProgress  PaymentState = "CheckFundsInProgress"
	CheckFundsSuccess     PaymentState = "CheckFundsSuccess"
	CheckFundsFail        PaymentState = "CheckFundsFail"
	TicketIssuanceTimeout PaymentState = "TicketIssuanceTimeout"
	DebitRequest          PaymentState = "DebitRequest"
	DebitInProgress       PaymentState = "DebitInProgress"
	DebitSuccess          PaymentState = "DebitSuccess"
	DebitFail             PaymentState = "DebitFail"
	TicketCanceled        PaymentState = "TicketCanceled"
	Refunded              PaymentState = "Refunded"
)

type TicketsPaymentStateChangedEvent struct {
	PaymentKey    string          `json:"payment_key"`
	PaymentId     string          `json:"payment_id"`
	PreviousState PaymentState    `json:"previous_state"`
	CurrentState  PaymentState    `json:"current_state"`
	To            entities.Member `json:"to"`
	From          entities.Member `json:"from"`
	Amount        uint            `json:"amount"`
	Currency      string          `json:"currency"`
}

const TicketPaymentCreated = "TicketPaymentCreated"
const TicketPaymentStateChanged = "TicketPaymentStateChanged"
