package model

import "context"

// Embedder turns text into L2-normalized dense vector embeddings.
type Embedder interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	ModelName() string
	Dimensions() int
	Close() error
}

// SearchService defines the query and lookup contract used by the HTTP layer.
type SearchService interface {
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
	Ask(ctx context.Context, req SearchRequest) (*AskResponse, error)
	Suggest(ctx context.Context, municipality, prefix string, limit int) ([]SuggestItem, error)
	GetDocument(ctx context.Context, municipality, id string) (*ArchiveDocumentFull, error)
	ListDocuments(ctx context.Context, municipality, category, docType string, ward int) ([]ArchiveDocument, error)
	ListDocumentsSorted(ctx context.Context, municipality, category, docType string, ward int, sortBy string) ([]ArchiveDocument, error)
	ListMunicipalities(ctx context.Context) ([]MunicipalityProfile, error)
	ListWards(ctx context.Context, municipality string) ([]WardRecord, error)
	ListOfficers(ctx context.Context, municipality string) ([]OfficerRecord, error)
	Stats(ctx context.Context) (*IndexStats, error)
}
