package repository

import (
	`ProjectMessageService/config`
	`ProjectMessageService/internal/api`
	`ProjectMessageService/internal/utils`
	`context`
	`fmt`
	`time`

	`github.com/jackc/pgx/v4/pgxpool`
)

type Repository struct {
	db  *pgxpool.Pool
	app *config.Application
}

type CreateUserParams struct {
	Username       string `json:"username"`
	HashedPassword string `json:"hashed_password"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
}

func NewRepository(db *pgxpool.Pool, app *config.Application) *Repository {
	return &Repository{db: db, app: app}
}

func NewPostgresDB(cfg config.Config, maxAttempts int) (pool *pgxpool.Pool, err error) {
	ctx := context.Background()
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	err = utils.TimeConnect(func() error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		pool, err = pgxpool.Connect(ctx, dsn)
		if err != nil {
			return err
		}

		return nil
	}, maxAttempts, 5*time.Second)

	if err != nil {
		return nil, err
	}

	return pool, nil
}

func (r *Repository) SaveMessage(ctx context.Context, message utils.Message) error {

	tableName := message.Topic
	exists, err := r.tableExists(ctx, tableName)
	if err != nil {
		r.app.Log.Errorf("Error checking if table exists: %s", err)
		return err
	}

	if !exists {
		r.app.Log.Errorf("ошибка нет такой таблицы: %s", tableName)
		err = fmt.Errorf("ошибка нет такой таблицы %s", tableName)
		return err
	}

	var count bool
	// Динамическое имя таблицы встроено в запрос с помощью форматирования строки
	query := fmt.Sprintf(`SELECT EXISTS (SELECT FROM %s WHERE content = $1)`, message.Topic)
	err = r.db.QueryRow(ctx, query, message.Message).Scan(&count)

	if err != nil {
		return err
	}

	if !count {
		// Динамическое имя таблицы встроено в запрос с помощью форматирования строки
		query = fmt.Sprintf(`INSERT INTO %s (content) VALUES ($1)`, message.Topic)
		_, err = r.db.Exec(ctx, query, message.Message)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) GetProcessedMessagesCount(ctx context.Context, topic utils.Messages) (int, error) {
	var count int
	tableName := topic.Topic
	exists, err := r.tableExists(ctx, tableName)
	if err != nil {
		r.app.Log.Errorf("Error checking if table exists: %s", err)
		return count, err
	}

	if !exists {
		err = fmt.Errorf("ошибка нет такой таблицы %s", tableName)
		return count, err
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE processed = true`, topic.Topic)
	err = r.db.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func RunMigrations(db *pgxpool.Pool, messages []string) error {
	ctx := context.Background()

	quser := `CREATE TABLE IF NOT EXISTS users (
    username varchar PRIMARY KEY,
    hashed_password varchar NOT NULL,
    full_name varchar NOT NULL,
    email varchar UNIQUE NOT NULL,
    password_changed_at timestamptz NOT NULL DEFAULT('0001-01-01 00:00:00Z'),
    created_at timestamptz NOT NULL DEFAULT (now()),
    is_email_verified boolean NOT NULL DEFAULT false,
    role varchar NOT NULL DEFAULT 'depositor'    
);`
	_, err := db.Exec(ctx, quser)
	if err != nil {
		return err
	}

	qsessions := `CREATE TABLE IF NOT EXISTS sessions (
		id uuid PRIMARY KEY,
		username varchar NOT NULL,
		refresh_token varchar NOT NULL,
		user_agent varchar NOT NULL,
		client_ip varchar NOT NULL,
		is_blocked boolean NOT NULL DEFAULT false,
		expires_at timestamptz NOT NULL,
		created_at timestamptz NOT NULL DEFAULT (now())
	);`
	_, err = db.Exec(ctx, qsessions)
	if err != nil {
		return err
	}

	qalter := `ALTER TABLE sessions ADD FOREIGN KEY (username) REFERENCES users (username);`
	_, err = db.Exec(ctx, qalter)
	if err != nil {
		return err
	}

	for _, message := range messages {
		msg := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ", message)

		query := `(
		id SERIAL PRIMARY KEY,
		content TEXT NOT NULL,
		processed BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	    );`

		_, err = db.Exec(ctx, msg+query)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) MarkMessageAsProcessed(ctx context.Context, message utils.Message, messageKey int) error {
	query := fmt.Sprintf(`UPDATE %s SET processed = true WHERE id = $1`, message.Topic)
	_, err := r.db.Exec(ctx, query, messageKey)
	return err
}

func (r *Repository) ContentMessagesKey(ctx context.Context, message utils.Message) (int, error) {
	var count int
	query := fmt.Sprintf(`SELECT id FROM %s WHERE content = $1`, message.Topic)
	err := r.db.QueryRow(ctx, query, message.Message).Scan(&count)
	return count, err
}

func (r *Repository) tableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name = $1
    )`
	err := r.db.QueryRow(ctx, query, tableName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

const createUser = `
INSERT INTO users (
    username,
    hashed_password,
    full_name,
    email
) VALUES (
    $1, $2, $3, $4
) RETURNING username, hashed_password, full_name, email, password_changed_at, created_at, is_email_verified, role
`

func (r *Repository) CreateUser(ctx context.Context, arg CreateUserParams) (api.User, error) {

	row := r.db.QueryRow(ctx, createUser,
		arg.Username,
		arg.HashedPassword,
		arg.FullName,
		arg.Email,
	)

	i := api.User{}
	err := row.Scan(
		&i.Username,
		&i.HashedPassword,
		&i.FullName,
		&i.Email,
		&i.PasswordChangedAt,
		&i.CreatedAt,
		&i.IsEmailVerified,
		&i.Role,
	)

	return i, err
}

const getUser = `-- name: GetUser :one
SELECT username, hashed_password, full_name, email, password_changed_at, created_at, is_email_verified, role FROM users
WHERE username = $1 LIMIT 1
`

func (r *Repository) GetUser(ctx context.Context, username string) (api.User, error) {
	row := r.db.QueryRow(ctx, getUser, username)

	i := api.User{}
	err := row.Scan(
		&i.Username,
		&i.HashedPassword,
		&i.FullName,
		&i.Email,
		&i.PasswordChangedAt,
		&i.CreatedAt,
		&i.IsEmailVerified,
		&i.Role,
	)

	return i, err
}
