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

func (scenario *scenarioState) creationOfAnObjectStorageBucketWithHTTPSXShouldY(httpsOption, expectedResult string) error {

	// Supported values for 'httpsOption':
	//	'enabled'
	//  'disabled'

	// Supported values for 'expectedResult':
	//	'succeed'
	//	'fail'

	// Standard auditing logic to ensures panics are also audited
	stepTrace, payload, err := utils.AuditPlaceholders()
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	// Validate input values

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
	log.Print(httpsEnabled) //TODO:Remove

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
	log.Print(shouldCreate) //TODO:Remove

	// TODO: Validate input for whitelistEntry using some regex

	err = godog.ErrPending
	return err
}

func (scenario *scenarioState) creationWillWithAnErrorMatching(expectation, errDescription string) error {

	var err error
	var stepTrace strings.Builder
	payload := struct {
		AccountName    string
		NetworkRuleSet azureStorage.NetworkRuleSet
		HTTPOption     bool
		HTTPSOption    bool
	}{}
	defer func() {
		scenario.audit.AuditScenarioStep(scenario.currentStep, stepTrace.String(), payload, err)
	}()

	stepTrace.WriteString("Generating random value for account name;")
	accountName := utils.RandomString(5) + "storageac"
	payload.AccountName = accountName

	networkRuleSet := azureStorage.NetworkRuleSet{
		DefaultAction: azureStorage.DefaultActionDeny,
		IPRules:       &[]azureStorage.IPRule{},
	}
	payload.NetworkRuleSet = networkRuleSet
	payload.HTTPOption = scenario.httpOption
	payload.HTTPSOption = scenario.httpsOption

	// Both true take it as http option is try
	if scenario.httpsOption && scenario.httpOption {
		stepTrace.WriteString(fmt.Sprintf(
			"Creating Storage Account with HTTPS: %v;", false))
		log.Printf("[DEBUG] Creating Storage Account with HTTPS: %v;", false)
		_, err = connection.CreateWithNetworkRuleSet(scenario.ctx, accountName,
			azureutil.ResourceGroup(), scenario.tags, false, &networkRuleSet)
	} else if scenario.httpsOption {
		stepTrace.WriteString(fmt.Sprintf(
			"Creating Storage Account with HTTPS: %v;", scenario.httpsOption))
		log.Printf("[DEBUG] Creating Storage Account with HTTPS: %v", scenario.httpsOption)
		_, err = connection.CreateWithNetworkRuleSet(scenario.ctx, accountName,
			azureutil.ResourceGroup(), scenario.tags, scenario.httpsOption, &networkRuleSet)
	} else if scenario.httpOption {
		stepTrace.WriteString(fmt.Sprintf(
			"Creating Storage Account with HTTPS: %v;", scenario.httpsOption))
		log.Printf("[DEBUG] Creating Storage Account with HTTPS: %v", scenario.httpsOption)
		_, err = connection.CreateWithNetworkRuleSet(scenario.ctx, accountName,
			azureutil.ResourceGroup(), scenario.tags, scenario.httpsOption, &networkRuleSet)
	}
	if err == nil {
		// storage account created so add to state
		stepTrace.WriteString(fmt.Sprintf(
			"Created Storage Account: %s;", accountName))
		log.Printf("[DEBUG] Created Storage Account: %s", accountName)
		scenario.storageAccounts = append(scenario.storageAccounts, accountName)
	}

	if expectation == "Fail" {

		if err == nil {
			err = fmt.Errorf("storage account was created, but should not have been: policy is not working or incorrectly configured")
			return err
		}

		detailedError := err.(autorest.DetailedError)
		originalErr := detailedError.Original
		detailed := originalErr.(*azure.ServiceError)

		log.Printf("[DEBUG] Detailed Error: %v", detailed)

		if strings.EqualFold(detailed.Code, "RequestDisallowedByPolicy") {
			stepTrace.WriteString("Request was Disallowed By Policy;")
			log.Printf("[DEBUG] Request was Disallowed By Policy: [Step PASSED]")
			return nil
		}

		err = fmt.Errorf("storage account was not created but not due to policy non-compliance")
		return err

	} else if expectation == "Succeed" {
		if err != nil {
			log.Printf("[ERROR] Unexpected failure in create storage ac [Step FAILED]")
			return err
		}
		return nil
	}

	err = fmt.Errorf("unsupported `result` option '%s' in the Gherkin feature - use either 'Fail' or 'Succeed'", expectation)
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
	ctx.Step(`^creation of an Object Storage bucket with https "([^"]*)" should "([^"]*)"$`, scenario.creationOfAnObjectStorageBucketWithHTTPSXShouldY)

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

	for _, account := range scenario.storageAccounts {
		log.Printf("[DEBUG] need to delete the storageAccount: %s", account)
		err := connection.DeleteAccount(scenario.ctx, azureutil.ResourceGroup(), account)

		if err != nil {
			log.Printf("[ERROR] error deleting the storageAccount: %v", err)
		}
	}

	log.Println("[DEBUG] Teardown completed")
}
