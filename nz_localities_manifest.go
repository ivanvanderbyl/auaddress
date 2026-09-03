package anzaddress

const (
	// NZLocalityDatasetURL identifies the authoritative LINZ dataset.
	NZLocalityDatasetURL = "https://data.linz.govt.nz/layer/113764-nz-suburbs-and-localities/"
	// NZLocalitySourceURL identifies the public LINZ ArcGIS query service used for this snapshot.
	NZLocalitySourceURL = "https://services.arcgis.com/xdsHIIxuCWByZiCB/arcgis/rest/services/LINZ_NZ_Suburbs_and_Localities/FeatureServer/0/query"
	// NZLocalityRetrievedAt is when this repository retrieved the source snapshot.
	NZLocalityRetrievedAt = "2026-09-03T02:07:00Z"
	// NZLocalityReleaseAt is the published-at timestamp for the LINZ dataset release.
	NZLocalityReleaseAt = "2026-08-25T21:39:50.458737Z"
	// NZLocalitySourceSHA256 is the SHA-256 checksum of the uncommitted ArcGIS query JSON snapshot.
	NZLocalitySourceSHA256 = "44fa75a973e65793d2be66297d810d2c7cc4cc7d078ab4b9e0966731713c0247"
	// NZLocalityLicense is the licence declared by LINZ for this dataset.
	NZLocalityLicense = "Creative Commons Attribution 4.0 International"
	// NZLocalityAttribution is the attribution required when reusing the source data.
	NZLocalityAttribution = "Sourced from the LINZ Data Service and licensed for reuse under CC BY 4.0."
	// NZLocalityFeatureCount is the number of source features in the recorded snapshot.
	NZLocalityFeatureCount = 6_563
	// NZLocalityNameCount is the number of canonical and alternate locality names in the generated index.
	NZLocalityNameCount = 8_080
)
