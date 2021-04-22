package azureana

import (
	"context"
	"fmt"
	"log"
	"net"

	azureStorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/cucumber/godog"

	azureutil "github.com/citihub/probr-pack-storage/internal/azure"
	"github.com/citihub/probr-pack-storage/internal/connection"
	"github.com/citihub/probr-sdk/audit"
	"github.com/citihub/probr-sdk/probeengine"

	"github.com/citihub/probr-sdk/utils"
)

// ProbeStruct allows this probe to be added to the ProbeStore
type probeStruct struct {
}

type scenarioState struct {
	name            string
	currentStep     string
	audit           *audit.ScenarioAudit
	probe           *audit.Probe
	ctx             context.Context
	tags            map[string]*string
	bucketName      string
	storageAccount  azureStorage.Account
	storageAccounts []string
	networkSegments NetworkSegments
}

// Probe ...
var Probe probeStruct             // Probe allows this probe to be added to the ProbeStore
var scenario scenarioState        // Local container of scenario state
var azConnection connection.Azure // Provides functionality to interact with Azure

func (scenario *scenarioState) anAzureSubscriptionIsAvailable() error {

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		// Catching any errors from panic
		if panicErr := recover(); panicErr != nil {
			err = utils.ReformatError("Unexpected error occured: ", panicErr)
		}
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()
	stepTrace.WriteString(fmt.Sprintf("Validate that Azure subscription specified in config file is available; "))

	payload = struct {
		SubscriptionID string
		TenantID       string
	}{
		azureutil.SubscriptionID(),
		azureutil.TenantID(),
	}

	err = azConnection.IsCloudAvailable() // Must be assigned to 'err' be audited
	return err
}

func (scenario *scenarioState) azureResourceGroupSpecifiedInConfigExists() error {

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		// Catching any errors from panic
		if panicErr := recover(); panicErr != nil {
			err = utils.ReformatError("Unexpected error occured: ", panicErr)
		}
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString("Check if value for Azure resource group is set in config vars; ")
	if azureutil.ResourceGroup() == "" {
		err = utils.ReformatError("Azure resource group config var not set")
		return err
	}

	stepTrace.WriteString("Check the resource group exists in the specified azure subscription; ")
	_, getGrpErr := azConnection.GetResourceGroupByName(azureutil.ResourceGroup())
	if getGrpErr != nil {
		err = utils.ReformatError("Azure resource group '%s' does not exists. Error: %v", azureutil.ResourceGroup(), getGrpErr)
		return err
	}

	//Audit log
	payload = struct {
		SubscriptionID string
		ResourceGroup  string
	}{
		SubscriptionID: azureutil.SubscriptionID(),
		ResourceGroup:  azureutil.ResourceGroup(),
	}

	return nil
}

func (scenario *scenarioState) aListWithAllowedAndDisallowedNetworkSegmentsIsProvidedInConfig() error {

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		// Catching any errors from panic
		if panicErr := recover(); panicErr != nil {
			err = utils.ReformatError("Unexpected error occured: ", panicErr)
		}
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString("Validate that allowed and disallowed network segments are provided in config; ")

	scenario.networkSegments = getNetworkSegments()

	if !(len(scenario.networkSegments.Allowed) > 0) || !(len(scenario.networkSegments.Disallowed) > 0) {
		err = utils.ReformatError("The list of allowed and disallowed network segments has not been defined in config")
	}

	//Audit log
	payload = struct {
		//NetworkSegments config.NetworkSegments
		NetworkSegments NetworkSegments
	}{
		NetworkSegments: scenario.networkSegments,
	}

	return err
}

func (scenario *scenarioState) anAttemptToCreateAStorageAccountWithAListOfXNetworkSegmentsY(access, expectedResult string) error {

	// Supported values for 'access':
	//	'allowed'
	//  'disallowed'

	// Supported values for 'expectedResult':
	//	'succeeds'
	//	'fails'

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		// Catching any errors from panic
		if panicErr := recover(); panicErr != nil {
			err = utils.ReformatError("Unexpected error occured: ", panicErr)
		}
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	// Validate input values
	var shouldCreate bool
	switch expectedResult {
	case "succeeds":
		shouldCreate = true
	case "fails":
		shouldCreate = false
	default:
		err = utils.ReformatError("Unexpected value provided for expectedResult: '%s' Expected values: ['succeeds', 'fails']", expectedResult)
		return err
	}

	var ipRangeList []string
	switch access {
	case "allowed":
		ipRangeList = scenario.networkSegments.Allowed
	case "disallowed":
		ipRangeList = scenario.networkSegments.Disallowed
	default:
		err = utils.ReformatError("Unexpected value provided for access: '%s' Expected values: ['allowed', 'disallowed']", access)
		return err
	}

	scenario.bucketName = utils.RandomString(10)
	stepTrace.WriteString(fmt.Sprintf("Generate a storage account name using a random string: '%s'; ", scenario.bucketName))

	stepTrace.WriteString(fmt.Sprintf("Set IP Rules to allow given IP Ranges %v; ", ipRangeList))
	var ipRules []azureStorage.IPRule
	for _, ipRange := range ipRangeList {

		// Validate input for ip range.
		// Acceptable values are single IP or IP Range in CIDR format.
		// Warning: this validation will pass IPv6, however only IPv4 address is allowed at the time this was written.
		// See azureStorage.IPRule.IPAddressOrRange property description.
		ipAddr := net.ParseIP(ipRange)
		if ipAddr == nil {
			_, _, ipErr := net.ParseCIDR(ipRange)
			if ipErr != nil {
				err = utils.ReformatError("Invalid IP Range '%s'. Acceptable values are single IP or IP Range in CIDR format.", ipRange)
				return err
			}
		}

		ipRule := azureStorage.IPRule{
			Action:           azureStorage.Allow,
			IPAddressOrRange: to.StringPtr(ipRange),
		}

		ipRules = append(ipRules, ipRule)
	}

	stepTrace.WriteString("Set Network Rule Set with IP Rules; ")
	var networkRuleSet azureStorage.NetworkRuleSet
	networkRuleSet = azureStorage.NetworkRuleSet{
		DefaultAction: azureStorage.DefaultActionDeny,
		IPRules:       &ipRules,
	}

	stepTrace.WriteString(fmt.Sprintf("Attempt to create storage bucket with allowed network IP Ranges: %v; ", ipRangeList))
	storageAccount, creationErr := azConnection.CreateStorageAccount(scenario.bucketName, azureutil.ResourceGroup(), scenario.tags, true, &networkRuleSet)

	scenario.storageAccount = storageAccount
	if creationErr == nil {
		scenario.storageAccounts = append(scenario.storageAccounts, scenario.bucketName) // Record for later cleanup
	}

	stepTrace.WriteString(fmt.Sprintf("Validate storage account creation %s; ", expectedResult))
	switch shouldCreate {
	case true:
		if creationErr != nil {
			err = utils.ReformatError("Creation of storage account did not succeed: %v", creationErr)
		}
	case false:
		if creationErr == nil {
			err = utils.ReformatError("Creation of storage account succeeded, but should have failed")
		}
		//TODO: Is this required? What is the appropriate error?
		// } else {
		// 	// stepTrace.WriteString(fmt.Sprintf("Check that storage account creation failed due to expected reason (403 Forbidden); "))
		// 	// if !errors.IsStatusCode(403, creationErr) {
		// 	// 	err = utils.ReformatError("Unexpected error during storage account creation : %v", creationErr)
		// 	// }
		// }
	}

	//Audit log
	payload = struct {
		StorageAccountName string
		ResourceGroup      string
		StorageAccount     azureStorage.Account
		NetworkRuleSet     azureStorage.NetworkRuleSet
		Tags               map[string]*string
	}{
		StorageAccountName: scenario.bucketName,
		ResourceGroup:      azureutil.ResourceGroup(),
		StorageAccount:     scenario.storageAccount,
		NetworkRuleSet:     networkRuleSet,
	}

	return err
}

func beforeScenario(s *scenarioState, probeName string, gs *godog.Scenario) {
	s.name = gs.Name
	s.probe = audit.State.GetProbeLog(probeName)
	s.audit = audit.State.GetProbeLog(probeName).InitializeAuditor(gs.Name, gs.Tags)
	s.ctx = context.Background()
	s.storageAccounts = make([]string, 0)
	probeengine.LogScenarioStart(gs)
}

func afterScenario(scenario scenarioState, probe probeStruct, gs *godog.Scenario, err error) {

	teardown()

	probeengine.LogScenarioEnd(gs)
}

// Name returns this probe's name
func (probe probeStruct) Name() string {
	return "allowed_network_access"
}

// Path returns this probe's feature file path
func (probe probeStruct) Path() string {
	return probeengine.GetFeaturePath("internal", "azure", probe.Name())
}

// ProbeInitialize handles any overall Test Suite initialisation steps.  This is registered with the
// test handler as part of the init() function.
func (probe probeStruct) ProbeInitialize(ctx *godog.TestSuiteContext) {

	ctx.BeforeSuite(func() {

		// Initialize azure connection
		azConnection = connection.NewAzureConnection(
			context.Background(),
			azureutil.SubscriptionID(),
			azureutil.TenantID(),
			azureutil.ClientID(),
			azureutil.ClientSecret(),
		)
	})

	ctx.AfterSuite(func() {
	})
}

// ScenarioInitialize initialises the scenario
func (probe probeStruct) ScenarioInitialize(ctx *godog.ScenarioContext) {

	ctx.BeforeScenario(func(s *godog.Scenario) {
		beforeScenario(&scenario, probe.Name(), s)
	})

	// Background
	ctx.Step(`^an Azure subscription is available$`, scenario.anAzureSubscriptionIsAvailable)
	ctx.Step(`^azure resource group specified in config exists$`, scenario.azureResourceGroupSpecifiedInConfigExists)

	// Steps
	ctx.Step(`^a list with allowed and disallowed network segments is provided in config$`, scenario.aListWithAllowedAndDisallowedNetworkSegmentsIsProvidedInConfig)
	ctx.Step(`^an attempt to create a storage account with a list of "([^"]*)" network segments "([^"]*)"$`, scenario.anAttemptToCreateAStorageAccountWithAListOfXNetworkSegmentsY)

	ctx.AfterScenario(func(s *godog.Scenario, err error) {
		afterScenario(scenario, probe, s, err)
	})

	ctx.BeforeStep(func(st *godog.Step) {
		scenario.currentStep = st.Text
	})

	ctx.AfterStep(func(st *godog.Step, err error) {
		scenario.currentStep = ""
	})
}

func teardown() {

	log.Printf("[DEBUG] Cleanup - removing storage accounts used during tests")

	for _, account := range scenario.storageAccounts {
		log.Printf("[DEBUG] need to delete the storageAccount: %s", account)
		err := azConnection.DeleteStorageAccount(azureutil.ResourceGroup(), account)

		if err != nil {
			log.Printf("[ERROR] error deleting the storageAccount: %v", err)
		}
	}

	log.Println("[DEBUG] Teardown completed")
}

// NetworkSegments represents the required config settings fr this probe. This shall be removed and replaced with actual config vars once sdk refactor is complete.
type NetworkSegments struct {
	Allowed    []string `yaml:"Allowed"`    // A list of allowed network segments to be used when creating storage accounts
	Disallowed []string `yaml:"Disallowed"` // A list of disallowed network segments to be used when creating storage accounts
}

func getNetworkSegments() NetworkSegments {

	//return config.Vars.ServicePacks.Storage.NetworkSegments

	// TODO: This is here until config refactoring in SDK is finished
	return NetworkSegments{
		Allowed: []string{
			"219.79.19.0/24",
			"170.74.231.168",
		},
		Disallowed: []string{
			"219.79.19.1",
			"219.108.32.1",
		},
	}
}
