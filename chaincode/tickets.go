package chaincode

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/looplab/fsm"
	"log"
	"s7ab-platform-hyperledger/platform/core/chaincode/base"
	"s7ab-platform-hyperledger/platform/core/chaincode/base/extensions/crud"
	"s7ab-platform-hyperledger/platform/core/chaincode/base/extensions/meta"
	"s7ab-platform-hyperledger/platform/core/chaincode/base/extensions/owner"
	"s7ab-platform-hyperledger/platform/core/chaincode/base/extensions/router"
	platformEntities "s7ab-platform-hyperledger/platform/core/entities"
	"s7ab-platform-hyperledger/platform/core/logger"
	apiEntities "s7ab-platform-hyperledger/platform/s7ticket/api/tickets/entities"
	"s7ab-platform-hyperledger/platform/s7ticket/entities"
)

const (
	RoleAgent    = "AGENT"
	RoleMerchant = "MERCHANT"
	RoleBank     = "BANK"
	RoleUnknown  = "UNKNOWN"
)

type Ticket struct {
	base.Chaincode
	router      *router.Group
	owner       *owner.Owner
	merchantKey string
	agentKey    string
	paymentKey  string
	meta.Meta
}

func NewTicket(l logger.Logger) Ticket {
	t := Ticket{agentKey: RoleAgent, merchantKey: RoleMerchant, paymentKey: `PAYMENT`}
	t.Log = l
	t.owner = owner.NewOwner(l)
	t.Meta = meta.NewMeta(t)
	r := router.New()

	// add agent handlers
	agentGroup := r.Group(`/agent`)
	agentGroup.Add(`/add`, t.agentAdd)
	agentGroup.Add(`/list`, t.agentList)

	// add meta handlers
	metaGroup := r.Group(`/meta`)
	metaGroup.Add(`/set`, t.SetMeta)
	metaGroup.Add(`/get`, t.GetMeta)

	// add main handlers
	r.Add(`/merchant`, t.merchant)
	r.Add(`/init`, t.initMerchant)
	r.Add(`/create`, t.create)
	r.Add(`/updateState`, t.updateState)
	r.Add(`/issue`, t.issue)
	r.Add(`/get`, t.get)
	r.Add(`/history`, t.history)
	t.router = r
	return t
}

// Init sets current chaincode owner if owner is presented
// Sets current MSP if owner isn't presented
func (t Ticket) Init(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	mspId := args[0]
	err := stub.PutState(t.merchantKey, []byte(mspId))
	if err != nil {
		return t.WriteError(err)
	}

	return t.owner.SetFromFirstArgOrCreator(stub)
}

func (t Ticket) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	return t.router.Handle(stub)
}

// GetKey is interface method for getting key from stub
// Expecting to get from stub less 2 arguments: [key,value]
func (t Ticket) GetKey(stub shim.ChaincodeStubInterface) (string, error) {
	_, args := stub.GetFunctionAndParameters()
	if len(args) < 1 {
		return ``, crud.ErrKeyNotPresented
	}
	return t.getPaymentKey(args[0]), nil
}

// GetData is interface method for getting data from stub
// Expecting to get from stub less 2 arguments: [key,value]
func (t Ticket) GetData(stub shim.ChaincodeStubInterface) ([]byte, error) {
	_, args := stub.GetFunctionAndParameters()
	if len(args) < 2 {
		return nil, crud.ErrKeyNotPresented
	}

	return []byte(args[1]), nil
}

// GetMetaKey is interface method for getting meta key from stub
// Expecting to get from stub less 3 arguments: [key,meta_key,meta_value]
func (t Ticket) GetMetaKey(stub shim.ChaincodeStubInterface) (string, error) {
	_, args := stub.GetFunctionAndParameters()
	if len(args) < 3 {
		return ``, meta.ErrMetaKeyNotPresented
	}
	return string(args[1]), nil
}

// GetMetaData is interface method for getting meta data from stub
// Expecting to get from stub less 2 arguments: [meta_key,meta_value]
func (t Ticket) GetMetaData(stub shim.ChaincodeStubInterface) ([]byte, error) {
	_, args := stub.GetFunctionAndParameters()
	if len(args) < 2 {
		return nil, meta.ErrMetaKeyNotPresented
	}

	return []byte(args[2]), nil
}

// GetStateDataWithMeta is interface method for getting mail data
func (t Ticket) GetStateDataWithMeta(stub shim.ChaincodeStubInterface) ([]byte, error) {
	if data, err := t.GetData(stub); err != nil {
		return nil, err
	} else {
		if metaKey, err := t.GetMetaKey(stub); err != nil {
			return nil, err
		} else {
			if metaData, err := t.GetMetaData(stub); err != nil {
				return nil, err
			} else {
				var p entities.Payment
				if err = json.Unmarshal(data, &p); err != nil {
					return nil, err
				}
				p.Meta[metaKey] = metaData
				if pBytes, err := json.Marshal(p); err != nil {
					return nil, err
				} else {
					return pBytes, nil
				}
			}
		}
	}
}

//todo: remove
func (t Ticket) initMerchant(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	if len(args) != 1 {
		return t.WriteError(fmt.Sprintf("Arguments count mismatch: %v", args))
	}

	existsMerchantId, err := stub.GetState(t.merchantKey)
	if err != nil {
		return t.WriteError(err)
	}

	if existsMerchantId != nil {
		return t.WriteError(fmt.Sprintf("MerchantId already set: %s", existsMerchantId))
	}

	err = stub.PutState(t.merchantKey, []byte(args[0]))
	if err != nil {
		return t.WriteError(err)
	}

	return t.WriteSuccess(nil)
}

func (t Ticket) getMember(APIstub shim.ChaincodeStubInterface, memberId string) (*platformEntities.Member, error) {
	var member platformEntities.Member

	response := APIstub.InvokeChaincode("organizations", t.ToChaincodeArgs("/get", memberId), platformEntities.SYSTEM_CHANNEL_NAME)
	if response.Status != shim.OK {
		return nil, fmt.Errorf("%s", response.Message)
	}

	if len(response.Payload) == 0 {
		return nil, fmt.Errorf("member not found: %s", memberId)
	}

	err := json.Unmarshal(response.Payload, &member)
	if err != nil {
		return nil, err
	}

	if member.Type != platformEntities.BANK_TYPE && !member.ConfirmedByBank {
		return &member, errors.New(fmt.Sprintf("member is not confirmed by bank: %s", memberId))
	}

	return &member, nil
}

func (t Ticket) getMerchant(stub shim.ChaincodeStubInterface) (merchant *platformEntities.Member, err error) {
	merchantId, err := stub.GetState(t.merchantKey)
	if err != nil {
		return
	}

	if merchantId == nil {
		return merchant, errors.New("merchantId not set in state")
	}

	merchant, err = t.getMember(stub, string(merchantId))
	return
}

func (t Ticket) merchant(stub shim.ChaincodeStubInterface) pb.Response {
	merchant, err := t.getMerchant(stub)
	if err != nil {
		return t.WriteError(err)
	}

	result, err := json.Marshal(merchant)
	if err != nil {
		return t.WriteError(err)
	}
	return t.WriteSuccess(result)
}

func (t Ticket) getAgentKey(stub shim.ChaincodeStubInterface, organizationId string) (string, error) {
	return stub.CreateCompositeKey(t.agentKey, []string{organizationId})
}

func (t Ticket) getPaymentKey(paymentId string) string {
	return fmt.Sprintf("%s_%s", t.paymentKey, paymentId)
}

func (t Ticket) getAgent(stub shim.ChaincodeStubInterface, agentId string) (agent *platformEntities.Member, err error) {

	agentKey, err := t.getAgentKey(stub, agentId)
	if err != nil {
		return
	}

	if existsCheck, err := stub.GetState(agentKey); err != nil {
		return agent, err
	} else if existsCheck == nil {
		return agent, errors.New(`agent not added with msp id = ` + agentId)
	}

	agent, err = t.getMember(stub, agentId)
	return
}

func (t Ticket) getBank(stub shim.ChaincodeStubInterface, bankId string) (bank *platformEntities.Member, err error) {
	bank, err = t.getMember(stub, bankId)

	if err == nil && bank.Type != platformEntities.BANK_TYPE {
		err = errors.New(fmt.Sprintf(`Organization is not bank, msp_id=%s, type = %s`, bank.OrganizationId, bank.Type))
	}
	return
}

// getActors returns current actors in chaincode - merchant, invoker, invokerRole
func (t Ticket) getActors(stub shim.ChaincodeStubInterface) (merchant *platformEntities.Member, invoker *platformEntities.Member, invokerRole string, err error) {

	//if no merchant in chaincode - no reason to check creator role
	merchant, err = t.getMerchant(stub)
	if err != nil {
		return
	}

	//fmt.Printf("\n\nMerchant id %s", merchant.OrganizationId)
	creator, err := t.GetCreator(stub)

	//fmt.Printf("\n\nCreator MSP is %s", creator.MspID)
	if err != nil {
		return
	}

	//Creator of transaction is merchant
	if creator.MspID == merchant.OrganizationId {
		invoker = merchant
		invokerRole = RoleMerchant
		return
	}

	//Try to find agent with creator.MspId
	invoker, err = t.getAgent(stub, creator.MspID)
	if err == nil {
		invokerRole = RoleAgent
		return
	}

	//Try to find agent with creator.MspId
	invoker, err = t.getBank(stub, creator.MspID)
	if err == nil {
		invokerRole = RoleBank
		return
	} else {
		err = nil
	}

	invokerRole = RoleUnknown
	return
}

// Add agent to smart contract, arg[0] - agent MSP id
func (t Ticket) agentAdd(stub shim.ChaincodeStubInterface) pb.Response {

	_, args := stub.GetFunctionAndParameters()
	if len(args) != 1 {
		return t.WriteError(fmt.Sprintf("arguments count mismatch: %v", args))
	}

	_, _, invokerRole, err := t.getActors(stub)
	if err != nil {
		return t.WriteError(err)
	}

	if invokerRole != RoleMerchant {
		return t.WriteError(fmt.Sprintf("only merchant can add agent, your role is: %s", invokerRole))
	}

	agentKey, err := t.getAgentKey(stub, args[0])
	if err != nil {
		return t.WriteError(err)
	}

	if err = stub.PutState(agentKey, []byte(args[0])); err != nil {
		return t.WriteError(err)
	}

	return t.WriteSuccess(nil)
}

func (t Ticket) agentList(stub shim.ChaincodeStubInterface) (r pb.Response) {
	var agents []*platformEntities.Member
	var agent *platformEntities.Member

	iter, err := stub.GetStateByPartialCompositeKey(t.agentKey, []string{})
	if err != nil {
		return t.WriteError(err)
	}

	defer iter.Close()
	for iter.HasNext() {
		v, err := iter.Next()
		if err != nil {
			return t.WriteError(err)
		}
		agent, err = t.getMember(stub, string(v.Value))
		if err != nil {
			return t.WriteError(err)
		}
		agents = append(agents, agent)
		//fmt.Println("\n KEY: ", string(v.Value))
	}

	result, err := json.Marshal(agents)
	if err != nil {
		return t.WriteError(err)
	}
	return t.WriteSuccess(result)
}

// Get organization data fron Organizations chaincode
func (t Ticket) getMemberByItn(stub shim.ChaincodeStubInterface, itn string) (member *platformEntities.Member, err error) {
	response := stub.InvokeChaincode("organizations", t.ToChaincodeArgs("/member/byITN", itn), platformEntities.SYSTEM_CHANNEL_NAME)
	if response.Status != shim.OK {
		return member, errors.New(fmt.Sprintf("Error getting member by itn: %s, error: %s", itn, response.Message))
	}
	if len(response.Payload) == 0 {
		return member, errors.New(fmt.Sprintf("Member with itn %s not found", itn))
	}
	err = json.Unmarshal(response.Payload, &member)
	return
}

// validate payload for creating payment
func (t Ticket) validatePaymentPayload(stub shim.ChaincodeStubInterface,
	paymentCreatePayload entities.PaymentCreatePayload,
	invoker *platformEntities.Member) (err error) {
	if exists, err := t.isPaymentExists(stub, paymentCreatePayload.Id); err != nil {
		return err
	} else if exists {
		return errors.New(`payment already exists`)

	}

	agentByItn, err := t.getMemberByItn(stub, paymentCreatePayload.PayerNumber)
	if err != nil {
		return
	}

	merchantByItn, err := t.getMemberByItn(stub, paymentCreatePayload.RecipientNumber)
	if err != nil {
		return
	}

	if invoker.OrganizationId != agentByItn.OrganizationId {
		return errors.New(fmt.Sprintf("agent itn mismatch in payment attributes %s", agentByItn.OrganizationId))
	}

	if invoker.Requisites.SettlementAccount != paymentCreatePayload.PayerAccount {
		return errors.New(fmt.Sprintf("agent account mismatch in payment attributes, agent account: %s, payerAccount: %s",
			invoker.Requisites.SettlementAccount, paymentCreatePayload.PayerAccount))
	}

	if merchantByItn.Requisites.SettlementAccount != paymentCreatePayload.RecipientAccount {
		return errors.New(fmt.Sprintf("merchant account mismatch in payment attributes, merchant account: %s, recipientAccount: %s",
			merchantByItn.Requisites.SettlementAccount, paymentCreatePayload.RecipientAccount))
	}

	return nil
}

//Create new payment, allowed only from confirmed agent
func (t Ticket) create(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	payload := []byte(args[0])

	var paymentCreatePayload entities.PaymentCreatePayload
	if err := json.Unmarshal(payload, &paymentCreatePayload); err != nil {
		return t.WriteError(err)
	}

	merchant, invoker, invokerRole, err := t.getActors(stub)
	if err != nil {
		return t.WriteError(err)
	}

	//fmt.Printf("\n\nIvoker role: %s, invoker : %s", invokerRole, invoker.OrganizationId)

	if invokerRole != RoleAgent {
		return t.WriteError(fmt.Sprintf("only agent can add payment, your role is: %s, id: %s", invokerRole, invoker.OrganizationId))
	}

	_, err = t.getMember(stub, invoker.BankOrganizationId)
	if err != nil {
		return t.WriteError(err)
	}

	log.Println("Payer number:", paymentCreatePayload.PayerNumber)

	if err = t.validatePaymentPayload(stub, paymentCreatePayload, invoker); err != nil {
		return t.WriteError(err)
	}

	payment := entities.Payment{
		Id:                  paymentCreatePayload.Id,
		Amount:              paymentCreatePayload.Amount,
		Currency:            paymentCreatePayload.Currency,
		InternationalFlight: paymentCreatePayload.InternationalFlight,
		PaymentType:         `SALE`,
		State:               entities.CheckFundsRequest,

		PayerOrgId:     invoker.OrganizationId,
		PayerId:        paymentCreatePayload.PayerId,
		PayerBankOrgId: invoker.BankOrganizationId,
		PayerNumber:    paymentCreatePayload.PayerNumber,
		PayerAccount:   paymentCreatePayload.PayerAccount,

		RecipientOrgId:     merchant.OrganizationId,
		RecipientBankOrgId: merchant.BankOrganizationId,
		RecipientId:        paymentCreatePayload.RecipientId,
		RecipientNumber:    paymentCreatePayload.RecipientNumber,
		RecipientAccount:   paymentCreatePayload.RecipientAccount,
	}

	paymentKey := t.getPaymentKey(payment.Id)

	if paymentToSaveBytes, err := json.Marshal(payment); err != nil {
		return t.WriteError(err)
	} else if err = stub.PutState(paymentKey, paymentToSaveBytes); err != nil {
		return t.WriteError(err)
	}

	event := entities.TicketsPaymentStateChangedEvent{
		PaymentId:    payment.Id,
		CurrentState: entities.CheckFundsRequest,
		PaymentKey:   paymentKey,
		To:           *merchant,
		From:         *invoker,
		Amount:       payment.Amount,
	}

	if eventBytes, err := json.Marshal(event); err != nil {
		return t.WriteError(err)
	} else {
		stub.SetEvent(string(entities.TicketPaymentCreated), eventBytes)
	}

	return shim.Success(nil)
}

// update ticket state
func (t Ticket) updateState(stub shim.ChaincodeStubInterface) pb.Response {

	_, args := stub.GetFunctionAndParameters()
	payloadBytes := []byte(args[0])

	var payload apiEntities.RequestUpdateState

	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return t.WriteError(err)
	}

	if payload.State == entities.PaymentStateEmpty {
		return t.WriteError("state is empty")
	}

	if payload.PaymentId == "" {
		return t.WriteError("paymentId is empty")
	}

	merchant, invoker, invokerRole, err := t.getActors(stub)
	if err != nil {
		return t.WriteError(err)
	}

	payment, err := t.getPayment(stub, payload.PaymentId)
	if err != nil {
		return t.WriteError(err)
	}

	if err = t.canChangePaymentState(payment, payload.State, invoker, invokerRole); err != nil {
		return t.WriteError(err)
	}

	agent, err := t.getMemberByItn(stub, payment.PayerNumber)
	if err != nil {
		return t.WriteError(err)
	}

	event := entities.TicketsPaymentStateChangedEvent{
		PaymentKey:    t.getPaymentKey(payment.Id),
		PaymentId:     payment.Id,
		PreviousState: payment.State,
		CurrentState:  payload.State,
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		To:            *merchant,
		From:          *agent,
	}

	payment.State = payload.State

	paymentWithNewStateBytes, err := json.Marshal(payment)
	if err != nil {
		return t.WriteError(err)
	}

	if err = stub.PutState(t.getPaymentKey(payment.Id), paymentWithNewStateBytes); err != nil {
		return t.WriteError(err)
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return t.WriteError(err)
	}

	if err = stub.SetEvent(entities.TicketPaymentStateChanged, eventBytes); err != nil {
		return t.WriteError(err)
	}
	return shim.Success(nil)

}

func (t Ticket) canChangePaymentState(
	payment *entities.Payment,
	newPaymentState entities.PaymentState,
	invoker *platformEntities.Member,
	invokerRole string) (err error) {

	if !t.roleCanChangeState(invokerRole, payment.State) {
		return fmt.Errorf(`role can't change from state: %s, role: %s`, payment.State, invokerRole)
	}

	//fmt.Printf ("\n%s %s %s", invokerRole, invoker.OrganizationId, payment.PayerBankOrgId)

	if invokerRole == RoleAgent && invoker.OrganizationId != payment.PayerOrgId {
		return fmt.Errorf(`agent can't operate with payment of another agent. try to updatestate from: %s, payment originally from: %s`,
			invoker.OrganizationId, payment.PayerOrgId)
	}

	if invokerRole == RoleBank && invoker.OrganizationId != payment.PayerBankOrgId {
		return errors.New(`bank can't process payment of another bank`)
	}

	if err := t.createFSM(payment.State).Event(string(newPaymentState)); err != nil {
		return fmt.Errorf("can't change payment state from: %s, to: %s, role: %s", payment.State, newPaymentState, invokerRole)
	}

	return
}

func (t Ticket) issue(stub shim.ChaincodeStubInterface) pb.Response {
	return t.WriteError(`not implemented`)
}

// get payment struct by payment id
func (t Ticket) getPayment(stub shim.ChaincodeStubInterface, paymentId string) (payment *entities.Payment, err error) {
	paymentBytes, err := stub.GetState(t.getPaymentKey(paymentId))
	if err != nil {
		return
	}
	if paymentBytes == nil {
		err = errors.New(fmt.Sprintf("\npayment not found with id %s", paymentId))
		return
	}
	err = json.Unmarshal(paymentBytes, &payment)
	return
}

func (t Ticket) isPaymentExists(stub shim.ChaincodeStubInterface, paymentId string) (exists bool, err error) {
	paymentBytes, err := stub.GetState(t.getPaymentKey(paymentId))
	return paymentBytes != nil, err
}

// public func, return byte representa
func (t Ticket) get(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	if len(args) != 1 {
		return shim.Error(fmt.Sprintf("Arguments count mismatch: %v", args))
	}
	paymentBytes, err := stub.GetState(t.getPaymentKey(args[0]))

	if err != nil {
		return t.WriteError(paymentBytes)
	}
	return t.WriteSuccess(paymentBytes)
}

func (t Ticket) roleCanChangeState(role string, state entities.PaymentState) bool {

	roleCanChangeState := map[entities.PaymentState]string{
		entities.CheckFundsRequest:    RoleBank,
		entities.CheckFundsInProgress: RoleBank,
		entities.CheckFundsSuccess:    RoleAgent,
		entities.DebitRequest:         RoleBank,
		entities.DebitInProgress:      RoleBank,
		entities.DebitFail:            RoleAgent,
	}

	needRole, ok := roleCanChangeState[state]
	return ok && role == needRole
}

func (t Ticket) createFSM(currentState entities.PaymentState) *fsm.FSM {
	return fsm.NewFSM(
		string(currentState),
		fsm.Events{
			{Name: string(entities.CheckFundsInProgress), Src: []string{string(entities.CheckFundsRequest)}, Dst: string(entities.CheckFundsInProgress)},
			{Name: string(entities.CheckFundsSuccess), Src: []string{string(entities.CheckFundsInProgress)}, Dst: string(entities.CheckFundsSuccess)},
			{Name: string(entities.CheckFundsFail), Src: []string{string(entities.CheckFundsInProgress)}, Dst: string(entities.CheckFundsFail)},
			{Name: string(entities.DebitRequest), Src: []string{string(entities.CheckFundsSuccess)}, Dst: string(entities.DebitRequest)},
			{Name: string(entities.DebitInProgress), Src: []string{string(entities.DebitRequest)}, Dst: string(entities.DebitInProgress)},
			{Name: string(entities.DebitSuccess), Src: []string{string(entities.DebitInProgress)}, Dst: string(entities.DebitSuccess)},
			{Name: string(entities.DebitFail), Src: []string{string(entities.DebitInProgress)}, Dst: string(entities.DebitFail)},
			{Name: string(entities.TicketCanceled), Src: []string{string(entities.DebitFail)}, Dst: string(entities.TicketCanceled)},
			{Name: string(entities.TicketIssuanceTimeout), Src: []string{}, Dst: string(entities.TicketIssuanceTimeout)},

			{Name: string(entities.Refunded), Src: []string{string(entities.DebitSuccess)}, Dst: string(entities.Refunded)},
		},
		fsm.Callbacks{},
	)
}

func (t Ticket) history(stub shim.ChaincodeStubInterface) pb.Response {
	key, err := t.GetKey(stub)
	if err != nil {
		return t.WriteError(err)
	}
	if iterator, err := stub.GetHistoryForKey(key); err != nil {
		return t.WriteError(err)
	} else {
		defer iterator.Close()
		var mods []platformEntities.KeyModification

		for iterator.HasNext() {
			if modification, err := iterator.Next(); err != nil {
				return t.WriteError(err)
			} else {
				mods = append(mods, platformEntities.KeyModification{
					TxID:     modification.TxId,
					Payload:  modification.Value,
					Time:     modification.Timestamp.Seconds,
					IsDelete: modification.IsDelete,
				})
			}
		}
		if modsBytes, err := json.Marshal(mods); err != nil {
			return t.WriteError(err)
		} else {
			return t.WriteSuccess(modsBytes)
		}
	}
}
