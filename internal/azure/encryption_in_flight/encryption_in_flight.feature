@s-azeif
Feature: Object Storage Encryption in Flight

    As a Cloud Security Architect
    I want to ensure that suitable security controls are applied to Object Storage
    So that my organisation is not vulnerable to interception of data in transit

    Background:
      Given an Azure subscription is available
      And azure resource group specified in config exists

    @s-azeif-001
    Scenario Outline: Prevent Creation of Object Storage Without Encryption in Flight

      Security Standard References:
        - CHC2-AGP140 : Ensure cryptographic controls are in place to protect the confidentiality and integrity of data in-transit, stored, generated and processed in the cloud

      Then creation of an Object Storage bucket with https "enabled" "succeeds"
      But creation of an Object Storage bucket with https "disabled" "fails" with error code "RequestDisallowedByPolicy"
      
      # TODO: Verify HTTP and HTTPs cannot be enabled at the same time
      # Try setting up storage account with custom subdomain and enabled http and https
      # https://docs.microsoft.com/en-us/azure/storage/blobs/storage-custom-domain-name?tabs=azure-portal#enable-https

  