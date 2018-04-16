package entities

import "s7ab-platform-hyperledger/platform/s7ticket/entities"

type (
	RequestMerchantInit struct {
		Merchant string `json:"merchant"`
		Agent    string `json:"agent"`
	}

	RequestAgentAdd struct {
		AgentId string `json:"agent_id"`
	}

	RequestUpdateState struct {
		PaymentId string                `json:"payment_id"`
		State     entities.PaymentState `json:"state"`
	}
)
