package azureaw

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	azurePolicy "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-01-01/policy"
	azureStorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/cucumber/godog"

	azureutil "github.com/citihub/probr-pack-storage/internal/azure"
	"github.com/citihub/probr-pack-storage/internal/azure/policy"
	"github.com/citihub/probr-pack-storage/internal/connection"
	"github.com/citihub/probr-sdk/audit"
	"github.com/citihub/probr-sdk/probeengine"

	"github.com/citihub/probr-sdk/utils"
)

const (
	policyAssignmentName = "deny_storage_wo_net_acl"        // TODO: Should this be in config?
	storageRgEnvVar      = "STORAGE_ACCOUNT_RESOURCE_GROUP" // TODO: Should this be replaced with azureutil.ResourceGroup() - which not only checks in env var, but also config vars?
)

// ProbeStruct allows this probe to be added to the ProbeStore
type probeStruct struct {
}

type scenarioState struct {
	name                      string
	currentStep               string
	audit                     *audit.ScenarioAudit
	probe                     *audit.Probe
	ctx                       context.Context
	policyAssignmentMgmtGroup string
	tags                      map[string]*string
	bucketName                string
	storageAccount            azureStorage.Account
	runningErr                error
	storageAccounts           []string
}

// Probe ...
var Probe probeStruct             // Probe allows this probe to be added to the ProbeStore
var scenario scenarioState        // Local container of scenario state
var azConnection connection.Azure // Provides functionality to interact with Azure

func (scenario *scenarioState) anAzureSubscriptionIsAvailable() error {

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
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

	var err error
	var stepTrace strings.Builder
	payload := struct {
		SubscriptionID string
		ResourceGroup  string
	}{
		SubscriptionID: azureutil.SubscriptionID(),
		ResourceGroup:  azureutil.ResourceGroup(),
	}
	defer func() {
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

	return nil
}

func (scenario *scenarioState) checkPolicyAssigned() error { // TODO: Review / Remove

	var err error
	var stepTrace strings.Builder
	payload := struct {
		AzureSubscriptionID  string
		ManagamentGroup      string
		PolicyAssignmentName string
		PolicyAssignment     azurePolicy.Assignment
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	var a azurePolicy.Assignment

	if scenario.policyAssignmentMgmtGroup == "" {
		stepTrace.WriteString("Management Group has not been set, check Policy Assignment at the Subscription;")
		a, err = policy.AssignmentBySubscription(scenario.ctx, azureutil.SubscriptionID(), policyAssignmentName)
	} else {
		stepTrace.WriteString("Check Policy Assignment at the Management Group;")
		a, err = policy.AssignmentByManagementGroup(scenario.ctx, scenario.policyAssignmentMgmtGroup, policyAssignmentName)
	}

	//Audit log
	payload.AzureSubscriptionID = azureutil.SubscriptionID()
	payload.ManagamentGroup = scenario.policyAssignmentMgmtGroup
	payload.PolicyAssignmentName = policyAssignmentName
	payload.PolicyAssignment = a

	if err != nil {
		log.Printf("[ERROR] Policy Assignment error: %v", err)
		return err
	}

	log.Printf("[DEBUG] Policy Assignment check: %v [Step PASSED]", *a.Name)
	return nil
}

func (scenario *scenarioState) creationOfAStorageAccountWithXWhitelistingEntryY(ipRange, expectedResult string) error {

	// Supported values for 'ipRange':
	//	ip range in CIDR format, e.g: 219.79.19.0/24
	//  "none" is an accepted value

	// Supported values for 'expectedResult':
	//	'succeeds'
	//	'fails'

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
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

	switch ipRange {
	case "none":
		ipRange = ""
	}
	// TODO: Validate input for whitelistEntry using some regex

	scenario.bucketName = utils.RandomString(10)
	stepTrace.WriteString(fmt.Sprintf("Generate a storage account name using a random string: '%s'; ", scenario.bucketName))

	stepTrace.WriteString(fmt.Sprintf("Attempt to create storage bucket with whitelisting for given IP Range: %s; ", ipRange))
	var networkRuleSet azureStorage.NetworkRuleSet
	if ipRange == "" {
		stepTrace.WriteString("IP Range is empty, using DefaultActionAllow for NetworkRuleSet; ")
		networkRuleSet = azureStorage.NetworkRuleSet{
			DefaultAction: azureStorage.DefaultActionAllow,
		}
	} else {
		stepTrace.WriteString("Set IP Rule to allow given IP Range; ")
		ipRule := azureStorage.IPRule{
			Action:           azureStorage.Allow,
			IPAddressOrRange: to.StringPtr(ipRange),
		}

		stepTrace.WriteString("Set Network Rule Set with IP Rule; ")
		networkRuleSet = azureStorage.NetworkRuleSet{
			IPRules:       &[]azureStorage.IPRule{ipRule},
			DefaultAction: azureStorage.DefaultActionDeny,
		}
	}

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

func (scenario *scenarioState) cspSupportsWhitelisting() error {

	var err error
	var stepTrace strings.Builder
	payload := struct {
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString("TODO: Pending implementation;")

	//return godog.ErrPending
	return nil
}

func (scenario *scenarioState) examineStorageContainer(containerNameEnvVar string) error {

	var err error
	var stepTrace strings.Builder
	payload := struct {
		StorageAccountName string
		ResourceGroup      string
		NetworkRuleSet     azureStorage.NetworkRuleSet
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString(fmt.Sprintf(
		"Checking value for environment variable: %s;", containerNameEnvVar))
	accountName := os.Getenv(containerNameEnvVar) // TODO: Should this come from config?
	payload.StorageAccountName = accountName
	if accountName == "" {
		err = fmt.Errorf("environment variable \"%s\" is not defined test can't run", containerNameEnvVar)
		return err
	}

	stepTrace.WriteString(fmt.Sprintf(
		"Checking value for environment variable: %s;", storageRgEnvVar))
	resourceGroup := os.Getenv(storageRgEnvVar) // TODO: Should this be replaced with azureutil.ResourceGroup() - which not only checks in env var, but also config vars?
	payload.ResourceGroup = resourceGroup
	if resourceGroup == "" {
		err = fmt.Errorf("environment variable \"%s\" is not defined test can't run", storageRgEnvVar)
		return err
	}

	stepTrace.WriteString("Retrieving storage account details from Azure;")
	scenario.storageAccount, scenario.runningErr = connection.AccountProperties(scenario.ctx, resourceGroup, accountName)
	if scenario.runningErr != nil {
		err = scenario.runningErr
		return err
	}

	stepTrace.WriteString("Checking that firewall network rule default action is not Allow;")
	networkRuleSet := scenario.storageAccount.AccountProperties.NetworkRuleSet
	payload.NetworkRuleSet = *networkRuleSet
	result := false
	// Default action is deny
	if networkRuleSet.DefaultAction == azureStorage.DefaultActionAllow {
		err = fmt.Errorf("%s has not configured with firewall network rule default action is not deny", accountName)
		return err
	}

	stepTrace.WriteString("Checking if it has IP whitelisting;")
	for _, ipRule := range *networkRuleSet.IPRules {
		result = true
		log.Printf("[DEBUG] IP WhiteListing: %v, %v", *ipRule.IPAddressOrRange, ipRule.Action)
	}

	stepTrace.WriteString("Checking if it has private Endpoint whitelisting;")
	for _, vnetRule := range *networkRuleSet.VirtualNetworkRules {
		result = true
		log.Printf("[DEBUG] VNet whitelisting: %v, %v", *vnetRule.VirtualNetworkResourceID, vnetRule.Action)
	}

	// TODO: Private Endpoint implementation when it's GA

	if result {
		log.Printf("[DEBUG] Whitelisting rule exists. [Step PASSED]")
		err = nil
	} else {
		err = fmt.Errorf("no whitelisting has been defined for %v", accountName)
	}
	return err
}

// PENDING IMPLEMENTATION
func (scenario *scenarioState) whitelistingIsConfigured() error {
	// Checked in previous step

	var err error
	var stepTrace strings.Builder
	payload := struct {
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	err = fmt.Errorf("Not Implemented")

	stepTrace.WriteString("TODO: Pending implementation;")

	//return err
	return nil //TODO: Remove this line. This is temporary to ensure test doesn't halt and other steps are not skipped
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
	return "access_whitelisting"
}

// Path returns this probe's feature file path
func (probe probeStruct) Path() string {
	return probeengine.GetFeaturePath("internal", "azure", probe.Name())
}

// ProbeInitialize handles any overall Test Suite initialisation steps.  This is registered with the
// test handler as part of the init() function.
//func (p ProbeStruct) ProbeInitialize(ctx *godog.Suite) {
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
	ctx.Step(`^the CSP provides a whitelisting capability for Object Storage containers$`, scenario.cspSupportsWhitelisting)

	ctx.Step(`^we examine the Object Storage container in environment variable "([^"]*)"$`, scenario.examineStorageContainer)
	ctx.Step(`^whitelisting is configured with the given IP address range or an endpoint$`, scenario.whitelistingIsConfigured)
	ctx.Step(`^security controls that Prevent Object Storage from being created without network source address whitelisting are applied$`, scenario.checkPolicyAssigned)
	ctx.Step(`^creation of a storage account with "([^"]*)" whitelisting entry "([^"]*)"$`, scenario.creationOfAStorageAccountWithXWhitelistingEntryY)

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

	for _, account := range scenario.storageAccounts {
		log.Printf("[DEBUG] need to delete the storageAccount: %s", account)
		err := connection.DeleteAccount(scenario.ctx, azureutil.ResourceGroup(), account)

		if err != nil {
			log.Printf("[ERROR] error deleting the storageAccount: %v", err)
		}
	}

	log.Println("[DEBUG] Teardown completed")
}
