package azureeif

import (
	"context"
	"fmt"
	"log"
	"strings"

	azureStorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/cucumber/godog"

	azureutil "github.com/citihub/probr-pack-storage/internal/azure"
	"github.com/citihub/probr-pack-storage/internal/connection"
	"github.com/citihub/probr-sdk/audit"
	"github.com/citihub/probr-sdk/probeengine"
	"github.com/citihub/probr-sdk/utils"
)

type scenarioState struct {
	name                      string
	currentStep               string
	audit                     *audit.ScenarioAudit
	probe                     *audit.Probe
	ctx                       context.Context
	tags                      map[string]*string
	httpOption                bool
	httpsOption               bool
	policyAssignmentMgmtGroup string
	storageAccounts           []string
}

// ProbeStruct allows this probe to be added to the ProbeStore
type probeStruct struct {
}

// Probe allows this probe to be added to the ProbeStore
var Probe probeStruct
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

func (scenario *scenarioState) httpAccessIs(arg1 string) error {

	var err error
	var stepTrace strings.Builder
	payload := struct {
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString(fmt.Sprintf(
		"Http Option: %s;", arg1))
	if arg1 == "enabled" {
		scenario.httpOption = true
	} else {
		scenario.httpOption = false
	}
	return nil
}

func (scenario *scenarioState) httpsAccessIs(arg1 string) error {

	var err error
	var stepTrace strings.Builder
	payload := struct {
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString(fmt.Sprintf(
		"Https Option: %s;", arg1))
	if arg1 == "enabled" {
		scenario.httpsOption = true
	} else {
		scenario.httpsOption = false
	}
	return nil
}

func (scenario *scenarioState) creationOfAnObjectStorageBucketWithHTTPSXShouldYWithErrorCodeZ(httpsOption, expectedResult, expectedErrorCode string) error {

	// Supported values for 'httpsOption':
	//	'enabled'
	//  'disabled'

	// Supported values for 'expectedResult':
	//	'succeed'
	//	'fail'

	// Supported values for 'expectedErrorCode':
	//	free text

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	// Validate input values - httpsOption
	var httpsEnabled bool
	switch httpsOption {
	case "enabled":
		httpsEnabled = true
	case "disabled":
		httpsEnabled = false
	default:
		err = utils.ReformatError("Unexpected value provided for httpsOption: '%s' Expected values: ['enabled', 'disabled']", httpsOption)
		return err
	}

	// Validate input values - expectedResult
	var shouldCreate bool
	switch expectedResult {
	case "succeed":
		shouldCreate = true
	case "fail":
		shouldCreate = false
	default:
		err = utils.ReformatError("Unexpected value provided for expectedResult: '%s' Expected values: ['succeeds', 'fails']", expectedResult)
		return err
	}

	resourceGroup := azureutil.ResourceGroup()
	bucketName := utils.RandomString(10)
	stepTrace.WriteString(fmt.Sprintf("Generate a storage account name using a random string: '%s'; ", bucketName))

	stepTrace.WriteString("Use DefaultActionAllow for NetworkRuleSet; ")
	networkRuleSet := azureStorage.NetworkRuleSet{
		DefaultAction: azureStorage.DefaultActionAllow,
	}

	stepTrace.WriteString(fmt.Sprintf(
		"Attempt to create Storage Account with HTTPS: %v; ", httpsEnabled))
	storageAccount, creationErr := azConnection.CreateStorageAccount(bucketName, resourceGroup, scenario.tags, httpsEnabled, &networkRuleSet)
	if creationErr == nil {
		scenario.storageAccounts = append(scenario.storageAccounts, bucketName) // Record for later cleanup
	}

	stepTrace.WriteString(fmt.Sprintf("Validate that storage account creation should %s; ", expectedResult))
	switch shouldCreate {
	case true:
		if creationErr != nil {
			err = utils.ReformatError("Creation of storage account did not succeed: %v", creationErr)
		}
	case false:
		if creationErr == nil {
			err = utils.ReformatError("Creation of storage account succeeded, but should have failed")
		} else {
			// Ensure failure is due to expected reason

			errorCode := ""

			// Perform type assertion for creation error. Expected DetailedError.AzureServiceError
			switch e := creationErr.(type) {
			case autorest.DetailedError:
				originalErr := e.Original

				switch ee := originalErr.(type) {
				case *azure.ServiceError: // This is the expected error type from azure sdk
					errorCode = ee.Code
				}

			default:
				// Error is generic, probably internal issue before invoking creation. Leaving this comment for clarity.
			}

			// Compare actual error with expected code
			if !strings.EqualFold(errorCode, expectedErrorCode) {
				err = fmt.Errorf("Creation of storage account failed with unexpected reason: %v - %v", errorCode, creationErr)
			}
		}
	}

	//Audit log
	payload = struct {
		StorageAccountName string
		ResourceGroup      string
		StorageAccount     azureStorage.Account
		NetworkRuleSet     azureStorage.NetworkRuleSet
		Tags               map[string]*string
	}{
		StorageAccountName: bucketName,
		ResourceGroup:      resourceGroup,
		StorageAccount:     storageAccount,
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

// Name will return this probe's name
func (probe probeStruct) Name() string {
	return "encryption_in_flight"
}

// Path will return this probe's feature path
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
	ctx.Step(`^creation of an Object Storage bucket with https "([^"]*)" should "([^"]*)" with error code "([^"]*)"$`, scenario.creationOfAnObjectStorageBucketWithHTTPSXShouldYWithErrorCodeZ)

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

func afterScenario(scenario scenarioState, probe probeStruct, gs *godog.Scenario, err error) {

	teardown()

	probeengine.LogScenarioEnd(gs)
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
