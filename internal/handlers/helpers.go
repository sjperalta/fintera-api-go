package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// ParseListQuery standardizes the extraction of common pagination, search, and sorting
// parameters from the request query string.
func ParseListQuery(c *gin.Context) *repository.ListQuery {
	query := repository.NewListQuery()

	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))

	// Support both search and search_term
	if search := c.Query("search_term"); search != "" {
		query.Search = search
	} else if search := c.Query("search"); search != "" {
		query.Search = search
	}

	// Parse sort parameter (format: field-direction, e.g. name-asc)
	if sort := c.Query("sort"); sort != "" && sort != "No Sort" {
		parts := strings.Split(sort, "-")
		query.SortBy = parts[0]
		if len(parts) > 1 {
			query.SortDir = parts[1]
		}
	}

	return query
}
