package pgdb

import (
	"testing"

	"github.com/lib/pq"
)

func TestPostgres(t *testing.T) {
	db, err := GetInstance()
	if err != nil {
		t.Error(err, "Try running /init/init_all.zsh")
	}
	// Assuming your *Db struct has a field named 'DB' of type *sql.DB
	txn, err := db.conn.Begin()
	if err != nil {
		t.Error(err)
	}

	stmt, err := txn.Prepare(pq.CopyIn("users", "name", "age"))
	if err != nil {
		t.Error(err)
	}

	type user struct {
		Name string
		Age  int64
	}

	users := []user{}

	for _, user := range users {
		_, err = stmt.Exec(user.Name, int64(user.Age))
		if err != nil {
			t.Error(err)
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		t.Error(err)
	}

	err = stmt.Close()
	if err != nil {
		t.Error(err)
	}

	err = txn.Commit()
	if err != nil {
		t.Error(err)
	}

}

func TestPostgreConnection(t *testing.T) {
	db, err := GetInstance()
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
}

func TestInsertRowAndGetAll(t *testing.T) {
	db, err := GetInstance()
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create test table
	table := "test_table"
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS " + db.Quote(table) + " (id SERIAL PRIMARY KEY, name TEXT, age INT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer db.Exec("DROP TABLE " + db.Quote(table))

	// Insert a row
	data := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}
	id, err := db.InsertRow(table, data)
	if err != nil {
		t.Fatalf("InsertRow failed: %v", err)
	}
	if id == 0 {
		t.Error("Expected non-zero insert id")
	}

	// GetAll
	rows, err := db.GetAll("SELECT * FROM "+db.Quote(table)+" WHERE id = ?", id)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got %v", rows[0]["name"])
	}
}

func TestUpdateRow(t *testing.T) {
	db, err := GetInstance()
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	table := "test_table"
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS " + db.Quote(table) + " (id SERIAL PRIMARY KEY, name TEXT, age INT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer db.Exec("DROP TABLE " + db.Quote(table))

	// Insert a row
	id, err := db.InsertRow(table, map[string]interface{}{"name": "Bob", "age": 25})
	if err != nil {
		t.Fatalf("InsertRow failed: %v", err)
	}

	// Update the row
	affected, err := db.UpdateRow(table, map[string]interface{}{"name": "Bobby"}, "id", id)
	if err != nil {
		t.Fatalf("UpdateRow failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 affected row, got %d", affected)
	}

	// Check update
	rows, err := db.GetAll("SELECT * FROM "+db.Quote(table)+" WHERE id = ?", id)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if rows[0]["name"] != "Bobby" {
		t.Errorf("Expected name 'Bobby', got %v", rows[0]["name"])
	}
}

func TestDeleteRow(t *testing.T) {
	db, err := GetInstance()
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	table := "test_table"
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS " + db.Quote(table) + " (id SERIAL PRIMARY KEY, name TEXT, age INT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer db.Exec("DROP TABLE " + db.Quote(table))

	// Insert a row
	id, err := db.InsertRow(table, map[string]interface{}{"name": "Charlie", "age": 40})
	if err != nil {
		t.Fatalf("InsertRow failed: %v", err)
	}

	// Delete the row
	affected, err := db.DeleteRow(table, "id", id)
	if err != nil {
		t.Fatalf("DeleteRow failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 affected row, got %d", affected)
	}

	// Check delete
	rows, err := db.GetAll("SELECT * FROM "+db.Quote(table)+" WHERE id = ?", id)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(rows))
	}
}
