package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
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

var db *sql.DB

func initDB() {
	var err error
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if host == "" || port == "" || user == "" || password == "" || dbname == "" {
		log.Fatal("Missing required database environment variables")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
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
			AND assignee_id = '7639588f-d211-4cdc-a57d-697111edaf49'
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
				ep.value as estimate_value,
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
			AND ia.assignee_id = '7639588f-d211-4cdc-a57d-697111edaf49'
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

	rows, err := db.Query(query)
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

	err = tmpl.Execute(w, issues)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	initDB()
	defer db.Close()

	// Подключаемся к SQLite
	sqliteDB, err := sql.Open("sqlite3", "./astralswag.db")
	if err != nil {
		log.Fatal("Failed to open SQLite database:", err)
	}
	defer sqliteDB.Close()

	// Создаем таблицу, если она не существует
	_, err = sqliteDB.Exec(`
		CREATE TABLE IF NOT EXISTS plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			content TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("Failed to create plans table:", err)
	}

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

		// Удаляем все существующие планы
		_, err := sqliteDB.Exec("DELETE FROM plans")
		if err != nil {
			log.Printf("Failed to delete existing plans: %v", err)
			http.Error(w, "Failed to save plan", http.StatusInternalServerError)
			return
		}

		// Вставляем новый план
		result, err := sqliteDB.Exec("INSERT INTO plans (content) VALUES (?)", content)
		if err != nil {
			log.Printf("Failed to save plan: %v", err)
			http.Error(w, "Failed to save plan", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Failed to get rows affected: %v", err)
			http.Error(w, "Failed to save plan", http.StatusInternalServerError)
			return
		}

		if rowsAffected == 0 {
			log.Printf("No rows were affected when saving plan")
			http.Error(w, "Failed to save plan", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Добавляем обработчик для получения плана
	http.HandleFunc("/get-plan", func(w http.ResponseWriter, r *http.Request) {
		var content string
		err := sqliteDB.QueryRow("SELECT content FROM plans ORDER BY created_at DESC LIMIT 1").Scan(&content)
		if err != nil {
			if err == sql.ErrNoRows {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"content": ""}`))
				return
			}
			log.Printf("Error getting plan: %v", err)
			http.Error(w, "Failed to get plan", http.StatusInternalServerError)
			return
		}

		// Экранируем специальные символы
		content = strings.ReplaceAll(content, `"`, `\"`)
		content = strings.ReplaceAll(content, "\n", "\\n")
		content = strings.ReplaceAll(content, "\t", "\\t")

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content": "` + content + `"}`))
	})

	// Добавляем обработчик для получения новых задач
	http.HandleFunc("/get-new-tasks", func(w http.ResponseWriter, r *http.Request) {
		// Получаем дату последнего сохраненного плана
		var lastPlanDate time.Time
		err := sqliteDB.QueryRow("SELECT created_at FROM plans ORDER BY created_at DESC LIMIT 1").Scan(&lastPlanDate)
		if err != nil {
			if err == sql.ErrNoRows {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`[]`))
				return
			}
			http.Error(w, "Failed to get last plan date", http.StatusInternalServerError)
			return
		}

		// Получаем новые задачи из PostgreSQL
		query := `
			WITH last_assignments AS (
				SELECT 
					issue_id,
					MAX(created_at) as last_assigned_at
				FROM issue_assignees
				WHERE deleted_at IS NULL
				AND assignee_id = '7639588f-d211-4cdc-a57d-697111edaf49'
				GROUP BY issue_id
			)
			SELECT DISTINCT
				i.id,
				i.name,
				p.name as project,
				p.identifier as project_identifier,
				i.sequence_id,
				ep.value as estimate_value
			FROM issues i
			LEFT JOIN states s ON i.state_id = s.id
			LEFT JOIN projects p ON i.project_id = p.id
			LEFT JOIN estimate_points ep ON i.estimate_point_id = ep.id
			INNER JOIN issue_assignees ia ON i.id = ia.issue_id
			LEFT JOIN last_assignments la ON i.id = la.issue_id
			WHERE i.deleted_at IS NULL
			AND ia.deleted_at IS NULL
			AND ia.assignee_id = '7639588f-d211-4cdc-a57d-697111edaf49'
			AND s.name IN ('Backlog', 'Todo', 'In Progress')
			AND i.created_at > $1
		`

		rows, err := db.Query(query, lastPlanDate)
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
