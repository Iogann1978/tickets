package common

import (
	"encoding/json"
	coreEntities "s7ab-platform-hyperledger/platform/core/entities"
	"s7ab-platform-hyperledger/platform/core/logger"
	"s7ab-platform-hyperledger/platform/s7platform/sdk"
	"s7ab-platform-hyperledger/platform/s7ticket/entities"
)

const (
	chaincode = `tickets`
)

type PaymentSDK struct {
	*sdk.SDKControlStructure
}

func (ts *PaymentSDK) PaymentByNumber(key string) (*entities.Payment, error) {
	paymentString, err := ts.SDKCore.Query(chaincode, `GetByKey`, []string{key})
	if err != nil {
		return nil, err
	}

	var p entities.Payment
	if err := json.Unmarshal([]byte(paymentString), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (ts *PaymentSDK) PaymentsList(limit int, offset int) ([]entities.Payment, error) {
	paymentBytes, err := ts.SDKCore.Query(chaincode, `List`, []string{string(limit), string(offset)})
	if err != nil {
		return nil, err
	}

	var payments []entities.Payment

	if err := json.Unmarshal(paymentBytes, &payments); err != nil {
		return nil, err
	}
	return payments, nil
}

func (ts *PaymentSDK) AgentsList() ([]coreEntities.Member, error) {
	agentsBytes, err := ts.SDKCore.Query(chaincode, `/agent/list`, []string{})
	if err != nil {
		return nil, err
	}

	var agents []coreEntities.Member

	if err = json.Unmarshal(agentsBytes, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (ts *PaymentSDK) Agent() (agent coreEntities.Member, err error) {
	agentBytes, err := ts.SDKCore.Query(chaincode, `/agent`, []string{})
	if err != nil {
		return
	}

	err = json.Unmarshal(agentBytes, &agent)
	return
}

func (ts *PaymentSDK) PaymentHistory(key string) ([]coreEntities.KeyModification, error) {
	historyBytes, err := ts.SDKCore.Query(chaincode, `/history`, []string{key})
	if err != nil {
		return nil, err
	}
	var mods []coreEntities.KeyModification
	if err = json.Unmarshal(historyBytes, &mods); err != nil {
		return nil, err
	}
	var p entities.Payment
	for i := 0; i < len(mods); i++ {
		if err = json.Unmarshal(mods[i].Payload, &p); err != nil {
			return nil, err
		} else {
			mods[i].PayloadParsed = p
		}
	}
	return mods, nil
}

func (ts *PaymentSDK) GetMerchant() (*coreEntities.Member, error) {
	merchantBytes, err := ts.SDKCore.Query(chaincode, `/merchant`, []string{})
	if err != nil {
		return nil, err
	}
	var m coreEntities.Member
	if err = json.Unmarshal(merchantBytes, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func InitSDK(org string, channel string, l logger.Logger) (*PaymentSDK, error) {

	s, err := sdk.Init(org, channel, l)
	if err != nil {
		return nil, err
	}
	return &PaymentSDK{s}, nil
}
