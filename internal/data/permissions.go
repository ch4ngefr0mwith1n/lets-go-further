package data

import (
	"context"
	"database/sql"
	"github.com/lib/pq"
	"time"
)

// unutar "Permissions" slice-a će biti sadržani svi "permission" kodovi (recimo, "movies:read" i "movies:write")
type Permissions []string

// "PermissionModel" će služiti za interakciju sa tabelama "permissions" i "users_permissions"
type PermissionModel struct {
	DB *sql.DB
}

// metoda koja provjerava da li "Permissions" slice sadrži određeni "permission" kod:
func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}

	return false
}

// metoda "GetAllPermissionsForUser" vraća sve "permission" kodove za određenog korisnika
// oni će biti u okviru "Permissions" slice-a
func (m PermissionModel) GetAllPermissionsForUser(userID int64) (Permissions, error) {
	query := `
        SELECT permissions.code
        FROM permissions
        INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
        INNER JOIN users ON users_permissions.user_id = users.id
        WHERE users.id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string

		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

// metoda koja dodaje "permissions" za određenog korisnika
// koristi se "variadic" parametar, preko kog možemo dodati više "permission"-a odjednom
func (m PermissionModel) AddPermissionForUser(userID int64, codes ...string) error {
	query := `
        INSERT INTO users_permissions
        SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, pq.Array(codes))
	return err
}

//-- Set the activated field for alice@example.com to true.
//UPDATE users SET activated = true WHERE email = 'alice@example.com';
//
//-- Give all users the 'movies:read' permission
//INSERT INTO users_permissions
//SELECT id, (SELECT id FROM permissions WHERE code = 'movies:read') FROM users;
//
//-- Give faith@example.com the 'movies:write' permission
//INSERT INTO users_permissions
//VALUES (
//(SELECT id FROM users WHERE email = 'faith@example.com'),
//(SELECT id FROM permissions WHERE  code = 'movies:write')
//);
//
//-- List all activated users and their permissions.
//SELECT email, array_agg(permissions.code) as permissions
//FROM permissions
//INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
//INNER JOIN users ON users_permissions.user_id = users.id
//WHERE users.activated = true
//GROUP BY email;
