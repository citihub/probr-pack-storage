@s-azana
Feature: Object Storage Has Allowed Network Access Measures Enforced

  As a Cloud Security Architect
  I want to ensure that suitable security controls are applied to Object Storage
  So that my organisation's data can only be accessed from allowed network IP addresses

  #Rule: CHC2-SVD030 - protect cloud service network access by limiting access from the appropriate source network only

    Background:
      Given an Azure subscription is available
      And azure resource group specified in config exists

    @s-azana-001
    Scenario: Prevent Object Storage from Being Created Without Allowed Network Source Address
      Given a list with allowed and disallowed network segments is provided in config
      When an attempt to create a storage account with a list of "allowed" network segments "succeeds"
      Then an attempt to create a storage account with a list of "disallowed" network segments "fails"