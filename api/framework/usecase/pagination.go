package usecase

const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 50
)

type PageQuery struct {
	Page     int
	PageSize int
}

type PageResult struct {
	Page        int
	PageSize    int
	TotalItems  int
	TotalPages  int
	HasPrevious bool
	HasNext     bool
}

func NormalizePageQuery(query PageQuery) (PageQuery, error) {
	if query.Page == 0 {
		query.Page = DefaultPage
	}
	if query.PageSize == 0 {
		query.PageSize = DefaultPageSize
	}
	if query.Page < 0 {
		return PageQuery{}, E(CodeValidation, "page must be greater than 0", nil)
	}
	if query.PageSize < 0 {
		return PageQuery{}, E(CodeValidation, "page_size must be greater than 0", nil)
	}
	if query.PageSize > MaxPageSize {
		return PageQuery{}, E(CodeValidation, "page_size is too large", nil)
	}

	return query, nil
}

func (query PageQuery) Limit() int {
	return query.PageSize
}

func (query PageQuery) Offset() int {
	return (query.Page - 1) * query.PageSize
}

func NewPageResult(query PageQuery, totalItems int) PageResult {
	if totalItems < 0 {
		totalItems = 0
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + query.PageSize - 1) / query.PageSize
	}

	return PageResult{
		Page:        query.Page,
		PageSize:    query.PageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasPrevious: query.Page > 1 && totalPages > 0,
		HasNext:     totalPages > 0 && query.Page < totalPages,
	}
}
