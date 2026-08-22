package module

import (
	"fmt"

	"tls-rest/go/engine/controllers/db/pgdb"
)

// GetAllWithPagination fetches rows as a slice of maps with pagination support
func GetAllWithPagination(db *pgdb.Db, query string, limit, offset int, args ...interface{}) ([]map[string]interface{}, error) {
	paginatedQuery := fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
	results, err := db.RQuery(paginatedQuery, args...)
	if err != nil {
		return nil, err
	}
	return results, nil
}
