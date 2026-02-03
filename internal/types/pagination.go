package types

// PaginationResponse 分页响应结构
type PaginationResponse struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
	Pages    int `json:"pages"`
}

// PaginatedQuery 分页查询参数
type PaginatedQuery struct {
	Page     int               `json:"page,omitempty"`
	PageSize int               `json:"page_size,omitempty"`
	OrderBy  string            `json:"order_by,omitempty"`
	Order    string            `json:"order,omitempty"`
	Filters  map[string]string `json:"filters,omitempty"`
}

// NewPaginatedQuery 创建新的分页查询参数
func NewPaginatedQuery(page, pageSize int) *PaginatedQuery {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return &PaginatedQuery{
		Page:     page,
		PageSize: pageSize,
		OrderBy:  "created_at",
		Order:    "DESC",
		Filters:  make(map[string]string),
	}
}

// GetOffset 计算偏移量
func (pq *PaginatedQuery) GetOffset() int {
	return (pq.Page - 1) * pq.PageSize
}

// SetFilter 设置过滤条件
func (pq *PaginatedQuery) SetFilter(key, value string) {
	if pq.Filters == nil {
		pq.Filters = make(map[string]string)
	}
	pq.Filters[key] = value
}

// GetFilter 获取过滤条件
func (pq *PaginatedQuery) GetFilter(key string) string {
	if pq.Filters == nil {
		return ""
	}
	return pq.Filters[key]
}
