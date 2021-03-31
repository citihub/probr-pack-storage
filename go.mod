module github.com/citihub/probr-pack-storage

go 1.14

require (
	github.com/Azure/azure-sdk-for-go v44.2.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.12.0
	github.com/Azure/go-autorest/autorest v0.11.0
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.0
	github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/citihub/probr-sdk v0.0.13
	github.com/cucumber/godog v0.11.0
	github.com/markbates/pkger v0.17.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// replace github.com/citihub/probr-sdk => ../probr-sdk
