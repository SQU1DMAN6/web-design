package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int64  `bun:",pk,autoincrement"`
	Name     string `bun:",notnull"`
	Email    string `bun:",notnull"`
	Password string `bun:",notnull"`
	PFP      string `bun:",notnull"`
	Bio      string `bun:",notnull"`
}

type ContactRequest struct {
	ID        int64     `bun:",pk,autoincrement"`
	Requester string    `bun:",notnull"`
	Recipient string    `bun:",notnull"`
	Status    string    `bun:",notnull"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

func ModelUser(db *bun.DB) error {
	ctx := context.Background()
	_, err := db.NewCreateTable().
		Model((*User)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = db.NewCreateTable().
		Model((*ContactRequest)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	return RepairDatabase(db)
}

func RepairDatabase(db *bun.DB) error {
	sqldb := db.DB
	if err := ensureColumn(sqldb, "users", "bio", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`UPDATE users SET bio = '' WHERE bio IS NULL`); err != nil {
		return err
	}
	if err := ensureColumn(sqldb, "contact_requests", "created_at", "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	if err := ensureColumn(sqldb, "contact_requests", "updated_at", "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_name ON users(name)`); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_pair ON contact_requests(requester, recipient)`); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`CREATE INDEX IF NOT EXISTS idx_contacts_recipient_status ON contact_requests(recipient, status)`); err != nil {
		return err
	}
	if _, err := sqldb.Exec(`CREATE INDEX IF NOT EXISTS idx_contacts_requester_status ON contact_requests(requester, status)`); err != nil {
		return err
	}
	if err := RepairLegacyProfilePicturePaths(db); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *sql.DB, tableName string, columnName string, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, columnName) {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, definition))
	return err
}

func CreateUser(db *bun.DB, name string, email string, password string) error {
	pass, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", name)
	fmt.Println("Regex:", pass)
	if pass != true {
		err = errors.New("username contains special characters")
		return err
	}
	if err != nil {
		return err
	}

	ctx := context.Background()
	hashedPassword, _ := HashPassword(password)
	user := &User{Name: name, Email: email, Password: hashedPassword, PFP: "/pfp/default.png", Bio: ""}
	query, err := db.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return errors.New("a user already exists with that email or username")
		}
		return err
	}
	fmt.Println("Database insert complete:", query)
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 16)
	return string(bytes), err
}

func CheckPassword(db *bun.DB, identifier, plainTextPassword string) (*User, error) {
	user, err := GetUserByEmail(identifier, db)
	if err != nil {
		user, err = GetUserByName(identifier, db)
		if err != nil {
			return nil, err
		}
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(plainTextPassword),
	); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, err
		}
		return nil, err
	}
	return user, nil
}

func GetUserByID(id int, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()
	err := db.NewSelect().
		Model(&userModel).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user:", err)
		return nil, err
	}

	fmt.Printf("User: %+v\n", userModel)

	return &userModel, nil
}

func GetUserByEmail(email string, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()

	err := db.NewSelect().
		Model(&userModel).
		Where("email = ?", email).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user:", err)
		return nil, err
	}

	fmt.Printf("User: %+v\n", userModel)

	return &userModel, nil
}

func CheckUserByEmail(email string, db *bun.DB) (bool, error) {
	var userModel User
	ctx := context.Background()

	exists, err := db.NewSelect().
		Model(&userModel).
		Where("email = ?", email).
		Exists(ctx)
	if err != nil {
		fmt.Println("Error querying user:", err)
		return false, err
	}

	fmt.Printf("User: %+v\n", userModel)
	return exists, nil
}

func GetUserByName(name string, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()

	err := db.NewSelect().
		Model(&userModel).
		Where("name = ?", name).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user", err)
		return nil, err
	}

	fmt.Printf("Uder %+v", userModel)

	return &userModel, nil
}

func UpdateUserProfile(db *bun.DB, name string, bio string, pfp string) error {
	ctx := context.Background()
	query := db.NewUpdate().Model((*User)(nil)).Set("bio = ?", bio).Where("name = ?", name)
	if strings.TrimSpace(pfp) != "" {
		query = query.Set("pfp = ?", ResolveProfilePicture(pfp, name))
	}
	_, err := query.Exec(ctx)
	return err
}

func RepairLegacyProfilePicturePaths(db *bun.DB) error {
	ctx := context.Background()
	var users []User
	if err := db.NewSelect().Model(&users).Column("id", "name", "pfp").Scan(ctx); err != nil {
		return err
	}
	for _, user := range users {
		resolved := ResolveProfilePicture(user.PFP, user.Name)
		if resolved != strings.TrimSpace(user.PFP) {
			if _, err := db.NewUpdate().Model((*User)(nil)).Set("pfp = ?", resolved).Where("id = ?", user.ID).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func SearchUsersByName(db *bun.DB, currentUser string, query string, limit int) ([]User, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	q := strings.TrimSpace(query)
	var users []User
	ctx := context.Background()
	selectQuery := db.NewSelect().
		Model(&users).
		Column("name", "bio", "pfp").
		Where("name != ?", currentUser).
		OrderExpr("name ASC").
		Limit(limit)
	if q != "" {
		selectQuery = selectQuery.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(q)+"%")
	}
	err := selectQuery.Scan(ctx)
	return users, err
}

func ResolveProfilePicture(raw string, username string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" || normalized == "/assets/default.png" || normalized == "default.png" || normalized == "/default.png" {
		return "/pfp/default.png"
	}
	if strings.HasPrefix(normalized, "/pfp/") {
		return normalized
	}

	if rel, err := filepath.Rel("/ftr/userData", normalized); err == nil {
		cleanRel := filepath.Clean(rel)
		if cleanRel != ".." && !strings.HasPrefix(cleanRel, "../") {
			return "/pfp/" + filepath.ToSlash(cleanRel)
		}
	}

	legacyRoot := "/ftr/userpfp"
	if rel, err := filepath.Rel(legacyRoot, normalized); err == nil {
		cleanRel := filepath.Clean(rel)
		if cleanRel != ".." && !strings.HasPrefix(cleanRel, "../") {
			return "/pfp/" + filepath.ToSlash(cleanRel)
		}
	}

	if filepath.IsAbs(normalized) {
		cleaned := filepath.Clean(normalized)
		if strings.Contains(cleaned, "/"+username+"/") {
			idx := strings.Index(cleaned, "/"+username+"/")
			if idx != -1 {
				rest := cleaned[idx+len("/"+username+"/"):]
				return "/pfp/" + username + "/" + filepath.ToSlash(rest)
			}
		}
		if filepath.Base(filepath.Dir(cleaned)) == username {
			return "/pfp/" + username + "/" + filepath.ToSlash(filepath.Base(cleaned))
		}
		return "/pfp/default.png"
	}
	if username != "" {
		cleanFile := strings.TrimLeft(normalized, "/")
		return "/pfp/" + username + "/" + filepath.ToSlash(cleanFile)
	}
	return "/pfp/default.png"
}

func RequestContact(db *bun.DB, requester string, recipient string) error {
	requester = strings.TrimSpace(requester)
	recipient = strings.TrimSpace(recipient)
	if requester == "" || recipient == "" || requester == recipient {
		return errors.New("invalid contact request")
	}
	if _, err := GetUserByName(recipient, db); err != nil {
		return errors.New("user not found")
	}

	ctx := context.Background()
	existing, err := GetContactBetween(db, requester, recipient)
	if err == nil && existing != nil {
		if existing.Status == "declined" {
			_, err = db.NewUpdate().
				Model((*ContactRequest)(nil)).
				Set("status = ?", "pending").
				Set("updated_at = ?", time.Now()).
				Where("id = ?", existing.ID).
				Exec(ctx)
			return err
		}
		return nil
	}

	contact := &ContactRequest{Requester: requester, Recipient: recipient, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_, err = db.NewInsert().Model(contact).Exec(ctx)
	return err
}

func RespondToContact(db *bun.DB, id int64, recipient string, accept bool) error {
	status := "declined"
	if accept {
		status = "accepted"
	}
	ctx := context.Background()
	res, err := db.NewUpdate().
		Model((*ContactRequest)(nil)).
		Set("status = ?", status).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Where("recipient = ?", recipient).
		Where("status = ?", "pending").
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("contact request not found")
	}
	return nil
}

func GetContactBetween(db *bun.DB, a string, b string) (*ContactRequest, error) {
	var contact ContactRequest
	ctx := context.Background()
	err := db.NewSelect().
		Model(&contact).
		Where("(requester = ? AND recipient = ?) OR (requester = ? AND recipient = ?)", a, b, b, a).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func ContactStatus(db *bun.DB, requester string, recipient string) string {
	contact, err := GetContactBetween(db, requester, recipient)
	if err != nil || contact == nil {
		return ""
	}
	return contact.Status
}

func CanReadContactDrops(db *bun.DB, requester string, dropOwner string) bool {
	var count int
	ctx := context.Background()
	err := db.NewSelect().
		Model((*ContactRequest)(nil)).
		ColumnExpr("count(*)").
		Where("requester = ?", requester).
		Where("recipient = ?", dropOwner).
		Where("status = ?", "accepted").
		Scan(ctx, &count)
	return err == nil && count > 0
}

func ListAcceptedContactOwners(db *bun.DB, requester string) ([]string, error) {
	var contacts []ContactRequest
	ctx := context.Background()
	err := db.NewSelect().
		Model(&contacts).
		Where("requester = ?", requester).
		Where("status = ?", "accepted").
		OrderExpr("recipient ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	owners := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		owners = append(owners, contact.Recipient)
	}
	return owners, nil
}

func ListMutualContacts(db *bun.DB, username string) ([]string, error) {
	var contacts []ContactRequest
	ctx := context.Background()
	err := db.NewSelect().
		Model(&contacts).
		Where("status = ?", "accepted").
		Where("(requester = ? OR recipient = ?)", username, username).
		OrderExpr("updated_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	names := []string{}
	for _, contact := range contacts {
		other := contact.Recipient
		if other == username {
			other = contact.Requester
		}
		if other != "" && !seen[other] {
			seen[other] = true
			names = append(names, other)
		}
	}
	return names, nil
}

func ListContactRequests(db *bun.DB, username string) (incoming []ContactRequest, outgoing []ContactRequest, err error) {
	ctx := context.Background()
	err = db.NewSelect().
		Model(&incoming).
		Where("recipient = ?", username).
		Where("status = ?", "pending").
		OrderExpr("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}
	err = db.NewSelect().
		Model(&outgoing).
		Where("requester = ?", username).
		Where("status = ?", "pending").
		OrderExpr("created_at DESC").
		Scan(ctx)
	return incoming, outgoing, err
}
