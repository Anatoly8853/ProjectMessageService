package repository

import (
	`ProjectMessageService/config`
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

	for _, message := range messages {
		msg := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ", message)

		query := `(
		id SERIAL PRIMARY KEY,
		content TEXT NOT NULL,
		processed BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	    );`

		_, err := db.Exec(context.Background(), msg+query)
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
