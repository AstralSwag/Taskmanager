package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type Issue struct {
	ID                string
	Name              string
	Description       string
	Priority          string
	State             string
	Point             *int
	Project           string
	ProjectIdentifier string
	SequenceID        int
	CreatedAt         time.Time
	AssignedAt        time.Time
	Estimate          *Estimate
}

type Estimate struct {
	Name        string
	Description string
	Type        string
	Value       string
}

type Project struct {
	ID          string
	Name        string
	Description string
	Identifier  string
}

type State struct {
	ID    string
	Name  string
	Color string
}

type Duration struct {
	Days  int
	Hours int
}

type User struct {
	ID   string
	Name string
}

var db *sql.DB
var users map[string]string
var currentUserID string

func initDB() {
	var err error
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if host == "" || port == "" || user == "" || password == "" || dbname == "" {
		log.Printf("Missing required database environment variables:")
		log.Printf("DB_HOST: %s", host)
		log.Printf("DB_PORT: %s", port)
		log.Printf("DB_USER: %s", user)
		log.Printf("DB_NAME: %s", dbname)
		log.Printf("DB_SSLMODE: %s", sslmode)
		log.Fatal("Missing required database environment variables")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	log.Printf("Connecting to database at %s:%s", host, port)

	// Try to connect with retries
	maxRetries := 30
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Error opening database connection (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}

		// Set connection pool settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Test the connection
		if err = db.Ping(); err == nil {
			log.Printf("Successfully connected to database")
			return
		}

		log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
		db.Close()
		time.Sleep(retryInterval)
	}

	log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
}

func timeSince(t time.Time) Duration {
	duration := time.Since(t)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	return Duration{Days: days, Hours: hours}
}

func getIssues() ([]Issue, error) {
	query := `
		WITH last_assignments AS (
			SELECT 
				issue_id,
				MAX(created_at) as last_assigned_at
			FROM issue_assignees
			WHERE deleted_at IS NULL
			AND assignee_id = $1
			GROUP BY issue_id
		),
		ranked_issues AS (
			SELECT DISTINCT
				i.id,
				i.name,
				i.description_stripped,
				i.priority,
				s.name as state,
				i.point,
				p.name as project,
				p.identifier as project_identifier,
				i.sequence_id,
				i.created_at,
				la.last_assigned_at,
				e.name as estimate_name,
				e.description as estimate_description,
				e.type as estimate_type,
				CASE 
					WHEN TRIM(ep.value) = '0' OR ep.value IS NULL THEN '0.5'
					ELSE TRIM(ep.value)
				END as estimate_value,
				CASE i.priority
					WHEN 'urgent' THEN 1
					WHEN 'high' THEN 2
					WHEN 'medium' THEN 3
					WHEN 'low' THEN 4
					ELSE 5
				END as priority_rank
			FROM issues i
			LEFT JOIN states s ON i.state_id = s.id
			LEFT JOIN projects p ON i.project_id = p.id
			LEFT JOIN estimate_points ep ON i.estimate_point_id = ep.id
			LEFT JOIN estimates e ON ep.estimate_id = e.id
			INNER JOIN issue_assignees ia ON i.id = ia.issue_id
			LEFT JOIN last_assignments la ON i.id = la.issue_id
			WHERE i.deleted_at IS NULL
			AND ia.deleted_at IS NULL
			AND ia.assignee_id = $1
			AND s.name IN ('Backlog', 'Todo', 'In Progress')
		)
		SELECT 
			id,
			name,
			description_stripped,
			priority,
			state,
			point,
			project,
			project_identifier,
			sequence_id,
			created_at,
			last_assigned_at,
			estimate_name,
			estimate_description,
			estimate_type,
			estimate_value
		FROM ranked_issues
		ORDER BY priority_rank, created_at DESC
		LIMIT 50
	`

	rows, err := db.Query(query, currentUserID)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var i Issue
		var estimateName, estimateDesc, estimateType, estimateValue sql.NullString
		err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Description,
			&i.Priority,
			&i.State,
			&i.Point,
			&i.Project,
			&i.ProjectIdentifier,
			&i.SequenceID,
			&i.CreatedAt,
			&i.AssignedAt,
			&estimateName,
			&estimateDesc,
			&estimateType,
			&estimateValue,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			return nil, err
		}

		if estimateName.Valid {
			i.Estimate = &Estimate{
				Name:        estimateName.String,
				Description: estimateDesc.String,
				Type:        estimateType.String,
				Value:       estimateValue.String,
			}
		}

		issues = append(issues, i)
	}
	log.Printf("Found %d issues", len(issues))
	return issues, nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if userID := r.FormValue("user_id"); userID != "" {
			currentUserID = userID
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	issues, err := getIssues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	funcMap := template.FuncMap{
		"timeSince": timeSince,
	}

	tmpl := template.New("index.html").Funcs(funcMap)
	tmpl, err = tmpl.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type PageData struct {
		Issues        []Issue
		Users         map[string]string
		CurrentUserID string
	}

	data := PageData{
		Issues:        issues,
		Users:         users,
		CurrentUserID: currentUserID,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loadUsers() error {
	// Проверяем существование файла
	if _, err := os.Stat("users.json"); os.IsNotExist(err) {
		return fmt.Errorf("users.json file not found: %v", err)
	}

	file, err := os.ReadFile("users.json")
	if err != nil {
		return fmt.Errorf("error reading users.json: %v", err)
	}

	users = make(map[string]string) // Инициализируем карту перед использованием
	err = json.Unmarshal(file, &users)
	if err != nil {
		return fmt.Errorf("error parsing users.json: %v", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found in users.json")
	}

	// Устанавливаем первого пользователя как текущего по умолчанию
	for id := range users {
		currentUserID = id
		break
	}

	log.Printf("Loaded %d users from users.json", len(users))
	return nil
}

func main() {
	initDB()
	if err := loadUsers(); err != nil {
		log.Fatalf("Error loading users: %v", err)
	}

	// Создаем директорию для данных если её нет
	dataDir := "/app/data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	// Подключаемся к SQLite
	dbPath := filepath.Join(dataDir, "astralswag.db")
	log.Printf("Connecting to SQLite database at: %s", dbPath)
	sqliteDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Failed to open SQLite database:", err)
	}
	defer sqliteDB.Close()

	// Проверяем структуру таблицы
	rows, err := sqliteDB.Query("PRAGMA table_info(plans)")
	if err != nil {
		log.Fatal("Failed to get table info:", err)
	}
	defer rows.Close()

	log.Printf("Table structure:")
	for rows.Next() {
		var cid int
		var name, type_ string
		var notnull int
		var dflt_value sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			log.Fatal("Failed to scan table info:", err)
		}
		log.Printf("Column: %s, Type: %s, NotNull: %d, Default: %v, PK: %d",
			name, type_, notnull, dflt_value.String, pk)
	}

	log.Printf("Successfully connected to SQLite database")

	http.HandleFunc("/", indexHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Добавляем новый обработчик для сохранения плана
	http.HandleFunc("/save-plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		content := r.FormValue("content")
		if content == "" {
			http.Error(w, "Content is required", http.StatusBadRequest)
			return
		}

		log.Printf("Saving plan for user %s", currentUserID)

		// Используем UPSERT для обновления или вставки плана пользователя
		result, err := sqliteDB.Exec(`
			INSERT INTO plans (user_id, content) 
			VALUES (?, ?) 
			ON CONFLICT(user_id) DO UPDATE SET 
				content = excluded.content,
				created_at = CURRENT_TIMESTAMP
		`, currentUserID, content)

		if err != nil {
			log.Printf("Failed to save plan for user %s: %v", currentUserID, err)
			http.Error(w, "Failed to save plan", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Failed to get rows affected: %v", err)
		} else {
			log.Printf("Plan saved for user %s, rows affected: %d", currentUserID, rowsAffected)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Добавляем обработчик для получения плана
	http.HandleFunc("/get-plan", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Getting plan for user %s", currentUserID)

		var content string
		err := sqliteDB.QueryRow(`
			SELECT content 
			FROM plans 
			WHERE user_id = ? 
			ORDER BY created_at DESC 
			LIMIT 1
		`, currentUserID).Scan(&content)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("No plan found for user %s", currentUserID)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"content": ""}`))
				return
			}
			log.Printf("Error getting plan for user %s: %v", currentUserID, err)
			http.Error(w, "Failed to get plan", http.StatusInternalServerError)
			return
		}

		// Добавляем логирование
		log.Printf("Raw plan content from DB for user %s: %s", currentUserID, content)

		// Создаем структуру для JSON
		type PlanResponse struct {
			Content string `json:"content"`
		}
		response := PlanResponse{Content: content}

		// Отправляем JSON
		w.Header().Set("Content-Type", "application/json")
		jsonData, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling JSON for user %s: %v", currentUserID, err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		log.Printf("Sending JSON response for user %s: %s", currentUserID, string(jsonData))
		w.Write(jsonData)
	})

	// Добавляем обработчик для получения новых задач
	http.HandleFunc("/get-new-tasks", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Getting new tasks for user %s", currentUserID)

		// Получаем дату последнего сохраненного плана
		var lastPlanDate time.Time
		err := sqliteDB.QueryRow(`
			SELECT created_at 
			FROM plans 
			WHERE user_id = ? 
			ORDER BY created_at DESC 
			LIMIT 1
		`, currentUserID).Scan(&lastPlanDate)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("No plan found for user %s, using current time", currentUserID)
				lastPlanDate = time.Now()
			} else {
				log.Printf("Error getting last plan date for user %s: %v", currentUserID, err)
				http.Error(w, "Failed to get last plan date", http.StatusInternalServerError)
				return
			}
		}

		// Получаем новые задачи из PostgreSQL
		query := `
			WITH last_assignments AS (
				SELECT 
					issue_id,
					MAX(created_at) as last_assigned_at
				FROM issue_assignees
				WHERE deleted_at IS NULL
				AND assignee_id = $1
				GROUP BY issue_id
			)
			SELECT DISTINCT
				i.id,
				i.name,
				p.name as project,
				p.identifier as project_identifier,
				i.sequence_id,
				CASE 
					WHEN TRIM(ep.value) = '0' OR ep.value IS NULL THEN '0.5'
					ELSE TRIM(ep.value)
				END as estimate_value
			FROM issues i
			LEFT JOIN states s ON i.state_id = s.id
			LEFT JOIN projects p ON i.project_id = p.id
			LEFT JOIN estimate_points ep ON i.estimate_point_id = ep.id
			INNER JOIN issue_assignees ia ON i.id = ia.issue_id
			LEFT JOIN last_assignments la ON i.id = la.issue_id
			WHERE i.deleted_at IS NULL
			AND ia.deleted_at IS NULL
			AND ia.assignee_id = $1
			AND i.created_at > $2
		`

		rows, err := db.Query(query, currentUserID, lastPlanDate)
		if err != nil {
			log.Printf("Error executing query: %v", err)
			http.Error(w, "Failed to get new tasks", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type NewTask struct {
			ID                string `json:"id"`
			Name              string `json:"name"`
			Project           string `json:"project"`
			ProjectIdentifier string `json:"project_identifier"`
			SequenceID        int    `json:"sequence_id"`
			Estimate          string `json:"estimate"`
			Link              string `json:"link"`
		}

		var newTasks []NewTask
		for rows.Next() {
			var task NewTask
			var estimate sql.NullString
			err := rows.Scan(
				&task.ID,
				&task.Name,
				&task.Project,
				&task.ProjectIdentifier,
				&task.SequenceID,
				&estimate,
			)
			if err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}

			task.Estimate = estimate.String
			task.Link = fmt.Sprintf("https://plane.it4retail.tech/it4retail/browse/%s-%d/", task.ProjectIdentifier, task.SequenceID)
			newTasks = append(newTasks, task)
		}

		json.NewEncoder(w).Encode(newTasks)
	})

	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
