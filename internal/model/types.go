package model

type SearchMode string

const (
	SortRelevance = "relevance"
	SortDateDesc  = "date_desc"
	SortDateAsc   = "date_asc"
	SortTitle     = "title"
)

const (
	SearchModeHybrid  SearchMode = "hybrid"
	SearchModeVector  SearchMode = "vector"
	SearchModeKeyword SearchMode = "keyword"
	MaxSearchLimit               = 50
)

type ContactInfo struct {
	Name  string `json:"name" yaml:"name"`
	Role  string `json:"role" yaml:"role"`
	Phone string `json:"phone,omitempty" yaml:"phone,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

type MunicipalityProfile struct {
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	ShortName        string  `json:"shortName"`
	State            string  `json:"state"`
	District         string  `json:"district"`
	FormationDate    string  `json:"formationDate"`
	AreaSqKm         float64 `json:"areaSqKm"`
	PopulationApprox string  `json:"populationApprox"`
	Households       int     `json:"households"`
	WardsCount       int     `json:"wardsCount"`
	ZonesCount       int     `json:"zonesCount"`
	DivisionsCount   int     `json:"divisionsCount"`
	VillagesCount    string  `json:"villagesCount"`
	SectorsCount     int     `json:"sectorsCount"`
	Headquarters     string  `json:"headquarters"`
	HelpdeskPhone    string  `json:"helpdeskPhone"`
	TollFree         string  `json:"tollFree"`
	WhatsApp         string  `json:"whatsapp,omitempty"`
	Email            string  `json:"email"`
	OfficialWebsite  string  `json:"officialWebsite"`
	Status           string  `json:"status"`
	Description      string  `json:"description"`
}

type DocumentFrontmatter struct {
	ID               string        `json:"id" yaml:"id"`
	Municipality     string        `json:"municipality" yaml:"municipality"`
	Title            string        `json:"title" yaml:"title"`
	Category         string        `json:"category" yaml:"category"`
	Subcategory      string        `json:"subcategory" yaml:"subcategory"`
	DocType          string        `json:"doc_type" yaml:"doc_type"`
	Date             string        `json:"date" yaml:"date"`
	SourceURL        string        `json:"source_url" yaml:"source_url"`
	SourcePortal     string        `json:"source_portal" yaml:"source_portal"`
	OriginalFilename string        `json:"original_filename,omitempty" yaml:"original_filename,omitempty"`
	Department       string        `json:"department" yaml:"department"`
	Wards            []int         `json:"wards" yaml:"wards"`
	Sectors          []string      `json:"sectors" yaml:"sectors"`
	Villages         []string      `json:"villages" yaml:"villages"`
	Tags             []string      `json:"tags" yaml:"tags"`
	Contacts         []ContactInfo `json:"contacts" yaml:"contacts"`
	ActionURL        string        `json:"action_url,omitempty" yaml:"action_url,omitempty"`
	Summary          string        `json:"summary" yaml:"summary"`
	VerifiedAt       string        `json:"verified_at" yaml:"verified_at"`
}

type ArchiveDocument struct {
	DocumentFrontmatter
	FilePath   string `json:"file_path"`
	WordCount  int    `json:"word_count"`
	ChunkCount int    `json:"chunk_count"`
	Content    string `json:"content,omitempty"`
}

type HeadingItem struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
}

type RelatedDocument struct {
	ID           string  `json:"id"`
	Municipality string  `json:"municipality"`
	Title        string  `json:"title"`
	Category     string  `json:"category"`
	Date         string  `json:"date"`
	Summary      string  `json:"summary"`
	Similarity   float64 `json:"similarity"`
}

type ArchiveDocumentFull struct {
	ArchiveDocument
	HTMLContent string            `json:"html_content,omitempty"`
	Headings    []HeadingItem     `json:"headings"`
	Related     []RelatedDocument `json:"related"`
}

type DocumentChunk struct {
	ChunkID      string `json:"chunk_id"`
	DocID        string `json:"doc_id"`
	Municipality string `json:"municipality"`
	ChunkIndex   int    `json:"chunk_index"`
	Heading      string `json:"heading"`
	Body         string `json:"body"`
	TokenCount   int    `json:"token_count"`
}

type SearchHit struct {
	DocID            string        `json:"doc_id"`
	ChunkID          string        `json:"chunk_id"`
	Municipality     string        `json:"municipality"`
	Title            string        `json:"title"`
	Category         string        `json:"category"`
	Subcategory      string        `json:"subcategory"`
	DocType          string        `json:"doc_type"`
	Date             string        `json:"date"`
	Department       string        `json:"department"`
	SourceURL        string        `json:"source_url"`
	SourcePortal     string        `json:"source_portal"`
	ActionURL        string        `json:"action_url,omitempty"`
	Wards            []int         `json:"wards"`
	Sectors          []string      `json:"sectors"`
	Villages         []string      `json:"villages"`
	Tags             []string      `json:"tags"`
	Contacts         []ContactInfo `json:"contacts"`
	Summary          string        `json:"summary"`
	Heading          string        `json:"heading"`
	Snippet          string        `json:"snippet"`
	MatchedChunkText string        `json:"matched_chunk_text"`
	Score            float64       `json:"score"`
	VectorScore      float64       `json:"vector_score"`
	KeywordScore     float64       `json:"keyword_score"`
	MatchSources     []string      `json:"match_sources"`
}

type FacetBucket struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type WardFacetBucket struct {
	Value int `json:"value"`
	Count int `json:"count"`
}

type FacetCounts struct {
	Categories []FacetBucket     `json:"categories"`
	DocTypes   []FacetBucket     `json:"doc_types"`
	Wards      []WardFacetBucket `json:"wards"`
}

type DetectedEntities struct {
	Wards    []int    `json:"wards"`
	Villages []string `json:"villages"`
	Sectors  []string `json:"sectors"`
}

type SearchRequest struct {
	Query        string
	Municipality string
	Category     string
	DocType      string
	Ward         int
	Mode         SearchMode
	Sort         string
	Limit        int
}

type SearchResponse struct {
	Query            string           `json:"query"`
	Municipality     string           `json:"municipality"`
	Mode             SearchMode       `json:"mode"`
	Sort             string           `json:"sort"`
	TookMs           int64            `json:"took_ms"`
	Total            int              `json:"total"`
	Hits             []SearchHit      `json:"hits"`
	Facets           FacetCounts      `json:"facets"`
	DetectedEntities DetectedEntities `json:"detected_entities"`
}

type PromptedCitation struct {
	Index        int    `json:"index"`
	DocID        string `json:"doc_id"`
	Municipality string `json:"municipality"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Heading      string `json:"heading"`
	Excerpt      string `json:"excerpt"`
	SourceURL    string `json:"source_url"`
	Date         string `json:"date"`
}

type ExtractedFact struct {
	Label         string `json:"label"`
	Value         string `json:"value"`
	Kind          string `json:"kind"`
	Href          string `json:"href,omitempty"`
	CitationIndex int    `json:"citationIndex"`
}

type CitedContact struct {
	ContactInfo
	CitationIndex int `json:"citationIndex"`
}

type KeyPoint struct {
	Text          string `json:"text"`
	CitationIndex int    `json:"citationIndex"`
}

type ActionLink struct {
	Label         string `json:"label"`
	URL           string `json:"url"`
	CitationIndex int    `json:"citationIndex"`
}

type PromptedAnswer struct {
	Query            string             `json:"query"`
	Headline         string             `json:"headline"`
	AnswerParagraphs []string           `json:"answer_paragraphs"`
	KeyPoints        []KeyPoint         `json:"key_points"`
	Facts            []ExtractedFact    `json:"facts"`
	Contacts         []CitedContact     `json:"contacts"`
	ActionLinks      []ActionLink       `json:"action_links"`
	Citations        []PromptedCitation `json:"citations"`
	FollowUpQueries  []string           `json:"follow_up_queries"`
	Confidence       string             `json:"confidence"`
}

type AskResponse struct {
	SearchResponse
	Brief PromptedAnswer `json:"brief"`
}

type SuggestItem struct {
	Type         string `json:"type"`
	Label        string `json:"label"`
	Sublabel     string `json:"sublabel"`
	Query        string `json:"query"`
	Municipality string `json:"municipality,omitempty"`
	DocID        string `json:"doc_id,omitempty"`
	Ward         int    `json:"ward,omitempty"`
}

type CouncillorInfo struct {
	Name  string `json:"name"`
	Party string `json:"party,omitempty"`
	Phone string `json:"phone,omitempty"`
	Notes string `json:"notes,omitempty"`
}

type WardRecord struct {
	Municipality      string          `json:"municipality,omitempty"`
	WardNumber        int             `json:"ward_number"`
	Name              string          `json:"name"`
	Zone              string          `json:"zone"`
	Division          string          `json:"division"`
	Villages          []string        `json:"villages"`
	Sectors           []string        `json:"sectors"`
	Colonies          []string        `json:"colonies,omitempty"`
	Councillor        *CouncillorInfo `json:"councillor,omitempty"`
	ExecutiveEngineer *ContactInfo    `json:"executive_engineer,omitempty"`
	AssistantEngineer *ContactInfo    `json:"assistant_engineer,omitempty"`
	JuniorEngineer    *ContactInfo    `json:"junior_engineer,omitempty"`
	JuniorEngineers   []ContactInfo   `json:"junior_engineers,omitempty"`
	BoundarySummary   string          `json:"boundary_summary,omitempty"`
}

type OfficerRecord struct {
	Municipality string `json:"municipality,omitempty"`
	SrNo         int    `json:"sr_no"`
	Name         string `json:"name"`
	Designation  string `json:"designation"`
	Branch       string `json:"branch"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	Wards        []int  `json:"wards"`
	Zone         string `json:"zone,omitempty"`
	Division     string `json:"division,omitempty"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type IndexStats struct {
	MunicipalitiesIndexed int             `json:"municipalities_indexed"`
	TotalDocuments        int             `json:"total_documents"`
	TotalChunks           int             `json:"total_chunks"`
	TotalWords            int             `json:"total_words"`
	WardsMapped           int             `json:"wards_mapped"`
	OfficersMapped        int             `json:"officers_mapped"`
	EmbeddingModel        string          `json:"embedding_model"`
	EmbeddingDimensions   int             `json:"embedding_dimensions"`
	VectorDBEngine        string          `json:"vector_db_engine"`
	FTSEngine             string          `json:"fts_engine"`
	IndexedAt             string          `json:"indexed_at"`
	Categories            []CategoryCount `json:"categories"`
}
