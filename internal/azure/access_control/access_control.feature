@s-azac
Feature: Object Storage Can Only Be Accessed By Authorized Users

  As a Cloud Security Architect
  I want to ensure that suitable security controls are applied to Object Storage
  So that my organisation's data can only be accessed by authorized users

    Background:
      Given an Azure subscription is available
      And azure resource group specified in config exists

    @s-azac-001
    Scenario Outline: Prevent Object Storage from Being Created With Anonymous Access
      Then an attempt to create a storage account "without" anonymous access "succeeds"
      But an attempt to create a storage account "with" anonymous access "fails"
      #TODO: Implement

  