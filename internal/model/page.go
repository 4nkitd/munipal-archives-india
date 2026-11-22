package model

import "html/template"

// PageData is the shared view model passed by internal/server to internal/web templates.
type PageData struct {
	PageTitle          string
	ActiveNav          string
	Municipality       MunicipalityProfile
	MunicipalityLabel  string
	MunicipalityCount  int
	MunicipalityGroups []MunicipalityGroup
	Stats              *IndexStats
	Query              string
	Mode               SearchMode
	Sort               string
	SelectedCat        string
	SelectedDocType    string
	SelectedWard       int
	AskResult          *AskResponse
	Document           *ArchiveDocumentFull
	RenderedHTML       template.HTML
	Wards              []WardRecord
	Officers           []OfficerRecord
	Documents          []ArchiveDocument
	OtherDocuments     []ArchiveDocument
	Categories         []CategoryCount
	PromptExamples     []string
	ErrorMessage       string
}

type MunicipalityGroup struct {
	State    string
	Profiles []MunicipalityProfile
}
