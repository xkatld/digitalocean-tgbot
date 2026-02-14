package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

type DB struct {
	Conn *sql.DB
}

func Init(path string) *DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}

	createTables := `
	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT,
		token TEXT UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS droplets (
		id INTEGER PRIMARY KEY,
		account_id INTEGER,
		name TEXT,
		password TEXT,
		ip TEXT,
		status TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(account_id) REFERENCES accounts(id)
	);`

	if _, err := db.Exec(createTables); err != nil {
		panic(err)
	}

	return &DB{Conn: db}
}

func (d *DB) AddAccount(email, token string) error {
	_, err := d.Conn.Exec("INSERT OR REPLACE INTO accounts (email, token) VALUES (?, ?)", email, token)
	return err
}

func (d *DB) ListAccounts() ([]Account, error) {
	rows, err := d.Conn.Query("SELECT id, email, token FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Account
	for rows.Next() {
		var a Account
		rows.Scan(&a.ID, &a.Email, &a.Token)
		results = append(results, a)
	}
	return results, nil
}

func (d *DB) GetAccount(id int64) (*Account, error) {
	var a Account
	err := d.Conn.QueryRow("SELECT id, email, token FROM accounts WHERE id = ?", id).Scan(&a.ID, &a.Email, &a.Token)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) DeleteAccount(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM accounts WHERE id = ?", id)
	return err
}

// Droplet operations

func (d *DB) SaveDroplet(id int, accountID int64, name, password, ip, status string) error {
	_, err := d.Conn.Exec("INSERT OR REPLACE INTO droplets (id, account_id, name, password, ip, status) VALUES (?, ?, ?, ?, ?, ?)",
		id, accountID, name, password, ip, status)
	return err
}

func (d *DB) GetDroplet(id int) (*Droplet, error) {
	var dr Droplet
	err := d.Conn.QueryRow("SELECT id, account_id, name, password, ip, status FROM droplets WHERE id = ?", id).
		Scan(&dr.ID, &dr.AccountID, &dr.Name, &dr.Password, &dr.IP, &dr.Status)
	if err != nil {
		return nil, err
	}
	return &dr, nil
}

func (d *DB) DeleteDroplet(id int) error {
	_, err := d.Conn.Exec("DELETE FROM droplets WHERE id = ?", id)
	return err
}

type Account struct {
	ID    int64
	Email string
	Token string
}

type Droplet struct {
	ID        int
	AccountID int64
	Name      string
	Password  string
	IP        string
	Status    string
}
