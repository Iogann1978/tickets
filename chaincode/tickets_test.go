package chaincode

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"

	coreCC "s7ab-platform-hyperledger/platform/core/chaincode"
	"s7ab-platform-hyperledger/platform/core/logger"

	. "s7ab-platform-hyperledger/platform/s7platform/testing"
	s7t "s7ab-platform-hyperledger/platform/s7platform/testing"
	"s7ab-platform-hyperledger/platform/s7platform/tests/fixture"
	"s7ab-platform-hyperledger/platform/s7ticket/entities"
	ticketFixture "s7ab-platform-hyperledger/platform/s7ticket/tests/fixture"
)

func msp(org fixture.OrgFixture) [2]string {
	return [2]string{org.OrgData().OrganizationId, org.OrgData().OrganizationCACert}
}

func TestTickets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tickets Suite")
}

func orgToCreatorTransformer(params ...interface{}) (mspID, cert string) {
	org := params[0].(fixture.OrgFixture).OrgData()
	return org.OrganizationId, org.OrganizationCACert
}

func ExpectPaymentState(tickets *s7t.FullMockStub, paymentId string, state entities.PaymentState) {
	paymentFromChaincode, _ := ticketFixture.FromBytes(tickets.Invoke("/get", paymentId).Payload)
	Expect(paymentFromChaincode.State).To(Equal(state))
}

var _ = Describe("Tickets", func() {

	l := logger.NewZapLogger(nil)

	var operator, bank, bank2 fixture.OrganizationFixture
	var merchant, agent, agent2, someOrg fixture.MemberFixture

	var orgs, tickets *s7t.FullMockStub

	var payment, payment2 ticketFixture.PaymentFixture

	BeforeSuite(func() {

		operator, _ = fixture.GetOrgFixture("Org1MSP.json")

		bank, _ = fixture.GetOrgFixture("Org2MSP.json")
		bank2, _ = fixture.GetOrgFixture("Org5MSP.json")

		merchant, _ = fixture.GetMemberFixture("Org3MSP.json", bank.OrganizationId, false)
		agent, _ = fixture.GetMemberFixture("Org4MSP.json", bank.OrganizationId, false)
		agent2, _ = fixture.GetMemberFixture("Org6MSP.json", bank2.OrganizationId, false)

		someOrg, _ = fixture.GetMemberFixture("Org8MSP.json", bank.OrganizationId, false)

		orgs = s7t.NewFullMockStub(`organizations`, coreCC.NewOrganization(l))
		orgs.MockInit("1", orgs.ArgsToBytes(operator.OrganizationId))
		orgs.RegisterCreatorTransformer(orgToCreatorTransformer)

		tickets = s7t.NewFullMockStub(`tickets`, NewTicket(l))
		//Merchant is owner of tickets chaincode
		tickets.MockInit("2", orgs.ArgsToBytes(merchant.OrganizationId))
		tickets.MockPeerChaincode("organizations/mychannel", orgs)
		tickets.RegisterCreatorTransformer(orgToCreatorTransformer)

		payment, _ = ticketFixture.GetFixture("payment_1_SALE_from_Org4MSP.json")
		payment2, _ = ticketFixture.GetFixture("payment_2_SALE_from_Org6MSP.json")
	})

	Describe("Initialization", func() {

		It("Allow init orgs and confirm by bank", func() {

			for _, o := range []interface{}{bank, bank2, merchant, agent, agent2, someOrg} {
				ExpectResponseOk(orgs.From(operator).Invoke("/create", o))
			}

			ExpectResponseOk(orgs.From(bank).Invoke("/bank/member/confirm", merchant.OrganizationId, merchant))
			ExpectResponseOk(orgs.From(bank).Invoke("/bank/member/confirm", agent.OrganizationId, agent))
			ExpectResponseOk(orgs.From(bank2).Invoke("/bank/member/confirm", agent2.OrganizationId, agent2))

			//fmt.Println(orgs.MockInvokeFunc("/get", agent.OrganizationId))
		})

		It("Allow add agent", func() {
			merchantFromChaincode, _ := fixture.GetMemberFromBytes(tickets.MockInvokeFunc("/merchant").Payload)
			Expect(merchantFromChaincode.OrganizationId).To(Equal(merchant.OrganizationId))

			tickets.MockCreator(merchant.OrganizationId, merchant.OrganizationCACert)
			ExpectResponseOk(tickets.From(merchant).Invoke("/agent/add", agent.OrganizationId))
			ExpectResponseOk(tickets.From(merchant).Invoke("/agent/add", agent2.OrganizationId))

			agents, _ := fixture.GetMembersFromBytes(tickets.MockInvokeFunc("/agent/list").Payload)

			Expect(len(agents)).To(Equal(2))
			Expect(agents[0].OrganizationId).To(Equal(agent.OrganizationId))
			Expect(agents[1].OrganizationId).To(Equal(agent2.OrganizationId))
		})

	})

	Describe("Payments", func() {
		It("Allow agent to  add payment", func() {

			ExpectResponseOk(tickets.From(agent).Invoke("/create", payment))
			ExpectResponseError(tickets.From(agent).Invoke("/create", payment), `payment already exists`)

			paymentFromChaincode, _ := ticketFixture.FromBytes(tickets.MockInvokeFunc("/get", payment.Id).Payload)
			Expect(paymentFromChaincode.Id).To(Equal(payment.Id))
			Expect(paymentFromChaincode.State).To(Equal(entities.CheckFundsRequest))

			ExpectResponseOk(tickets.From(agent2).Invoke("/create", payment2))
		})

		It("Disallow non agents to  add payment", func() {
			paymentNew, _ := ticketFixture.GetFixture("payment_1_SALE_from_Org4MSP.json")

			paymentNew.Id = `some new id`
			ExpectResponseError(tickets.From(merchant).Invoke("/create", paymentNew), `only agent can add payment, your role is: MERCHANT,`)

			//payment.PayerNumber = someOrg.Requisites.ITN

			ExpectResponseError(tickets.From(bank).Invoke("/create", paymentNew), `only agent can add payment, your role is: BANK`)
			ExpectResponseError(tickets.From(someOrg).Invoke("/create", paymentNew), `only agent can add payment, your role is: UNKNOWN`)
		})

		It("Disallow incorect transitions from debit request", func() {

			ExpectResponseError(tickets.From(bank).Invoke("/updateState", ticketFixture.UpdateState(payment.Id, entities.CheckFundsSuccess)),
				`can't change payment state from: CheckFundsRequest, to: CheckFundsSuccess, role: BANK`)

		})

		It("Allow bank1 to check funds for payment 1", func() {

			updateState := ticketFixture.UpdateState(payment.Id, entities.CheckFundsInProgress)

			ExpectResponseError(tickets.From(merchant).Invoke("/updateState", updateState),
				`role can't change from state: CheckFundsRequest, role: MERCHANT`)
			ExpectResponseError(tickets.From(agent).Invoke("/updateState", updateState),
				`role can't change from state: CheckFundsRequest, role: AGENT`)

			ExpectResponseOk(tickets.From(bank).Invoke("/updateState", updateState))
			ExpectPaymentState(tickets, payment.Id, entities.CheckFundsInProgress)

			updateState = ticketFixture.UpdateState(payment.Id, entities.CheckFundsSuccess)
			ExpectResponseError(tickets.From(agent).Invoke("/updateState", updateState),
				`role can't change from state: CheckFundsInProgress, role: AGENT`)
			ExpectResponseOk(tickets.From(bank).Invoke("/updateState", updateState))
			ExpectPaymentState(tickets, payment.Id, entities.CheckFundsSuccess)
		})

		It("Allow bank2 to check funds for payment 2", func() {
			ExpectResponseOk(tickets.From(bank2).Invoke("/updateState", ticketFixture.UpdateState(payment2.Id, entities.CheckFundsInProgress)))
			ExpectPaymentState(tickets, payment2.Id, entities.CheckFundsInProgress)

			checkFundsSuccess := ticketFixture.UpdateState(payment2.Id, entities.CheckFundsSuccess)
			//try to update  payment from agent2 serviced by bank2  from bank1
			ExpectResponseError(tickets.From(bank).Invoke("/updateState", checkFundsSuccess), `bank can't process payment of another bank`)
			ExpectResponseOk(tickets.From(bank2).Invoke("/updateState", checkFundsSuccess))
		})

		//It("Disallow banks to check funds for other bank payments", func() {
		//
		//})

		It(`Allow agents to DebitRequest their payments`, func() {

			debitRequest := ticketFixture.UpdateState(payment.Id, entities.DebitRequest)
			debitRequest2 := ticketFixture.UpdateState(payment2.Id, entities.DebitRequest)

			ExpectResponseError(tickets.From(bank).Invoke("/updateState", debitRequest),
				`role can't change from state: CheckFundsSuccess, role: BANK`)

			ExpectResponseError(tickets.From(merchant).Invoke("/updateState", debitRequest),
				`role can't change from state: CheckFundsSuccess, role: MERCHANT`)

			ExpectResponseOk(tickets.From(agent).Invoke("/updateState", debitRequest))
			ExpectResponseError(tickets.From(agent).Invoke("/updateState", debitRequest2),
				`agent can't operate with payment of another agent. try to updatestate from: Org4MSP, payment originally from: Org6MSP`)
			ExpectResponseOk(tickets.From(agent2).Invoke("/updateState", debitRequest2))

			ExpectPaymentState(tickets, payment.Id, entities.DebitRequest)
			ExpectPaymentState(tickets, payment2.Id, entities.DebitRequest)
		})

		It(`Allow bank to process debit process`, func() {

			debitInProgress := ticketFixture.UpdateState(payment.Id, entities.DebitInProgress)
			debitInProgress2 := ticketFixture.UpdateState(payment2.Id, entities.DebitInProgress)
			ExpectResponseError(tickets.From(agent).Invoke("/updateState", debitInProgress),
				`role can't change from state: DebitRequest, role: AGENT`)

			ExpectResponseError(tickets.From(merchant).Invoke("/updateState", debitInProgress),
				`role can't change from state: DebitRequest, role: MERCHANT`)

			ExpectResponseOk(tickets.From(bank).Invoke("/updateState", debitInProgress))
			ExpectPaymentState(tickets, payment.Id, entities.DebitInProgress)

			ExpectResponseError(tickets.From(bank).Invoke("/updateState", debitInProgress2),
				`bank can't process payment of another bank`)

			ExpectResponseOk(tickets.From(bank2).Invoke("/updateState", debitInProgress2))

			ExpectPaymentState(tickets, payment.Id, entities.DebitInProgress)
			ExpectPaymentState(tickets, payment2.Id, entities.DebitInProgress)

			debitSuccess := ticketFixture.UpdateState(payment.Id, entities.DebitSuccess)
			debitFail := ticketFixture.UpdateState(payment2.Id, entities.DebitFail)

			ExpectResponseError(tickets.From(bank2).Invoke("/updateState", debitSuccess),
				`bank can't process payment of another bank`)

			ExpectResponseOk(tickets.From(bank).Invoke("/updateState", debitSuccess))
			ExpectResponseOk(tickets.From(bank2).Invoke("/updateState", debitFail))

			ExpectPaymentState(tickets, payment.Id, entities.DebitSuccess)
			ExpectPaymentState(tickets, payment2.Id, entities.DebitFail)

		})

	})
})
