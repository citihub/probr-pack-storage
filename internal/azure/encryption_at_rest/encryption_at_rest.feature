@s-azear
Feature: Object Storage Encryption at Rest

  As a Cloud Security Architect
  I want to ensure that suitable security controls are applied to Object Storage
  So that my organisation is protected against data leakage due to misconfiguration

    Background:
      Given an Azure subscription is available
      And azure resource group specified in config exists

    @s-azear-001
    Scenario Outline: Prevent Creation of Object Storage Without Encryption at Rest using Customer Managed Keys

      Security Standard References:
        - CHC2-AGP140 : Ensure cryptographic controls are in place to protect the confidentiality and integrity of data in-transit, stored, generated and processed in the cloud

      # TODO: Modify to include customer-managed keys similar to client implementation
      # Ticket created: https://github.com/citihub/probr-pack-storage/issues/5
