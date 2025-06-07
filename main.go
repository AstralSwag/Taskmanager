package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

func init() {
	// Устанавливаем временную зону для всего приложения
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("Error loading timezone: %v", err)
	}
	time.Local = loc

	// Настраиваем формат логов
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}

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

type Quote struct {
	Text   string
	Author string
}

type AttendanceStatus struct {
	UserID    string
	IsOffice  bool
	UpdatedAt time.Time
}

type UserAttendance struct {
	UserID         string
	TodayStatus    *AttendanceStatus
	TomorrowStatus *AttendanceStatus
}

var db *sql.DB
var users map[string]string
var currentQuote Quote
var quoteMutex sync.RWMutex

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
			// Initialize attendance table
			if err := initAttendanceTable(); err != nil {
				log.Printf("Error initializing attendance table: %v", err)
			}
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

func getIssues(userID string) ([]Issue, error) {
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

	rows, err := db.Query(query, userID)
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

func getQuote() (Quote, error) {
	log.Printf("Getting quote from API...")

	// Создаем URL с параметрами
	apiURL := "http://api.forismatic.com/api/1.0/"
	formData := url.Values{}
	formData.Set("method", "getQuote")
	formData.Set("format", "xml")
	formData.Set("lang", "ru")
	formData.Set("key", "1") // Добавляем ключ для API

	// Отправляем POST запрос
	resp, err := http.PostForm(apiURL, formData)
	if err != nil {
		log.Printf("Error making request to API: %v", err)
		return Quote{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return Quote{}, err
	}

	log.Printf("Received response from API: %s", string(body))

	var result struct {
		Quote struct {
			Text   string `xml:"quoteText"`
			Author string `xml:"quoteAuthor"`
		} `xml:"quote"`
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		log.Printf("Error unmarshaling XML: %v", err)
		return Quote{}, err
	}

	quote := Quote{
		Text:   result.Quote.Text,
		Author: result.Quote.Author,
	}
	log.Printf("Successfully parsed quote: %+v", quote)
	return quote, nil
}

func updateQuote() {
	log.Printf("Updating quote...")
	quote, err := getQuote()
	if err != nil {
		log.Printf("Error getting quote: %v", err)
		return
	}

	quoteMutex.Lock()
	currentQuote = quote
	quoteMutex.Unlock()
	log.Printf("Quote updated successfully: %+v", quote)
}

func startQuoteScheduler() {
	// Обновляем цитату при запуске
	updateQuote()

	// Запускаем горутину для обновления цитаты каждый день в 4:00 AM
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(next.Sub(now))
			updateQuote()
		}
	}()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из параметров запроса
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	issues, err := getIssues(userID)
	if err != nil {
		log.Printf("Error getting issues: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	type PageData struct {
		Issues        []Issue
		Users         map[string]string
		CurrentUserID string
		Quote         Quote
	}

	data := PageData{
		Issues:        issues,
		Users:         users,
		CurrentUserID: userID,
		Quote:         currentQuote,
	}
	log.Printf("PageData being sent to template: %+v", data)

	funcMap := template.FuncMap{
		"timeSince": timeSince,
		"upper":     strings.ToUpper,
	}

	tmpl := template.New("index.html").Funcs(funcMap)
	tmpl, err = tmpl.ParseFiles("templates/index.html")
	if err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	log.Printf("Loaded %d users from users.json", len(users))
	return nil
}

func initAttendanceTable() error {
	log.Printf("Initializing attendance table...")

	// Проверяем текущее время в PostgreSQL
	var pgNow time.Time
	err := db.QueryRow("SELECT CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Moscow'").Scan(&pgNow)
	if err != nil {
		log.Printf("Error getting PostgreSQL current time: %v", err)
	} else {
		log.Printf("Current time in PostgreSQL during table initialization: %s", pgNow.Format("2006-01-02 15:04:05.000000 -0700"))
	}

	query := `
		CREATE TABLE IF NOT EXISTS attendance (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			is_office BOOLEAN NOT NULL DEFAULT false,
			is_today BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			date_part DATE GENERATED ALWAYS AS (DATE(created_at AT TIME ZONE 'Europe/Moscow')) STORED,
			UNIQUE(user_id, date_part, is_today)
		);
		CREATE INDEX IF NOT EXISTS idx_attendance_user_created ON attendance(user_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_attendance_date_part ON attendance(date_part);`

	_, err = db.Exec(query)
	if err != nil {
		log.Printf("Error creating attendance table: %v", err)
		return err
	}

	log.Printf("Attendance table initialized successfully")
	return nil
}

func getLastPlanDate(userID string) (time.Time, error) {
	var lastDate time.Time
	err := db.QueryRow(`
		SELECT created_at 
		FROM plans 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1`, userID).Scan(&lastDate)
	if err != nil {
		return time.Time{}, err
	}
	return lastDate, nil
}

func getAttendanceStatus() ([]UserAttendance, error) {
	// Получаем текущее время в московской временной зоне
	moscowLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return nil, fmt.Errorf("error loading timezone: %v", err)
	}
	now := time.Now().In(moscowLoc)
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")

	// Получаем все записи за сегодня и завтра
	rows, err := db.Query(`
		SELECT user_id, is_office, is_today, created_at
		FROM attendance
		WHERE date_part IN ($1, $2)
		ORDER BY created_at DESC
	`, today, tomorrow)
	if err != nil {
		return nil, fmt.Errorf("error querying attendance: %v", err)
	}
	defer rows.Close()

	// Создаем карту для хранения статусов пользователей
	userStatuses := make(map[string]*UserAttendance)

	for rows.Next() {
		var userID string
		var isOffice bool
		var isToday bool
		var createdAt time.Time

		if err := rows.Scan(&userID, &isOffice, &isToday, &createdAt); err != nil {
			return nil, fmt.Errorf("error scanning attendance row: %v", err)
		}

		// Если пользователя еще нет в карте, создаем новую запись
		if _, exists := userStatuses[userID]; !exists {
			userStatuses[userID] = &UserAttendance{
				UserID: userID,
			}
		}

		// Обновляем соответствующий статус
		status := &AttendanceStatus{
			UserID:    userID,
			IsOffice:  isOffice,
			UpdatedAt: createdAt,
		}

		if isToday {
			userStatuses[userID].TodayStatus = status
		} else {
			userStatuses[userID].TomorrowStatus = status
		}
	}

	// Преобразуем карту в слайс
	result := make([]UserAttendance, 0, len(userStatuses))
	for _, status := range userStatuses {
		result = append(result, *status)
	}

	return result, nil
}

func updateAttendanceStatus(userID string, isOffice bool, isToday bool) error {
	// Получаем текущее время в московской временной зоне
	moscowLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return fmt.Errorf("error loading timezone: %v", err)
	}
	now := time.Now().In(moscowLoc)
	datePart := now.Format("2006-01-02")

	// Обновляем или создаем запись
	_, err = db.Exec(`
		INSERT INTO attendance (user_id, is_office, is_today, created_at, date_part)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, date_part, is_today) 
		DO UPDATE SET 
			is_office = $2,
			created_at = $4
	`, userID, isOffice, isToday, now, datePart)

	if err != nil {
		return fmt.Errorf("error updating attendance status: %v", err)
	}

	return nil
}

func savePlanHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Получаем текущую дату в московской временной зоне
	moscowLoc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		http.Error(w, fmt.Sprintf("error loading timezone: %v", err), http.StatusInternalServerError)
		return
	}
	now := time.Now().In(moscowLoc)
	today := now.Format("2006-01-02")

	// Сохраняем план
	_, err = db.Exec(`
		INSERT INTO plans (user_id, date, plan, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (user_id, date) 
		DO UPDATE SET 
			plan = $3,
			updated_at = $4
	`, userID, today, content, now)

	if err != nil {
		http.Error(w, fmt.Sprintf("error saving plan: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": content})
}

func getPlanHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	query := `
		SELECT plan 
		FROM plans 
		WHERE user_id = $1 AND date = $2
	`
	var plan string
	err := db.QueryRow(query, userID, today).Scan(&plan)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"content": ""})
			return
		}
		log.Printf("Error getting plan: %v", err)
		http.Error(w, "Error getting plan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": plan})
}

func getNewTasksHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	lastPlanDate, err := getLastPlanDate(userID)
	if err != nil {
		log.Printf("Error getting last plan date: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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
			i.created_at,
			CASE 
				WHEN TRIM(ep.value) = '0' OR ep.value IS NULL THEN '0.5'
				ELSE TRIM(ep.value)
			END as estimate_value
		FROM issues i
		LEFT JOIN projects p ON i.project_id = p.id
		LEFT JOIN estimate_points ep ON i.estimate_point_id = ep.id
		INNER JOIN issue_assignees ia ON i.id = ia.issue_id
		LEFT JOIN last_assignments la ON i.id = la.issue_id
		WHERE i.deleted_at IS NULL
		AND ia.deleted_at IS NULL
		AND ia.assignee_id = $1
		AND (la.last_assigned_at IS NULL OR la.last_assigned_at > $2)
		ORDER BY i.created_at DESC
	`

	rows, err := db.Query(query, userID, lastPlanDate)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type NewTask struct {
		ID                string    `json:"id"`
		Name              string    `json:"name"`
		Project           string    `json:"project"`
		ProjectIdentifier string    `json:"project_identifier"`
		SequenceID        int       `json:"sequence_id"`
		CreatedAt         time.Time `json:"created_at"`
		Estimate          string    `json:"estimate"`
		Link              string    `json:"link"`
	}

	var tasks []NewTask
	for rows.Next() {
		var task NewTask
		err := rows.Scan(
			&task.ID,
			&task.Name,
			&task.Project,
			&task.ProjectIdentifier,
			&task.SequenceID,
			&task.CreatedAt,
			&task.Estimate,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		task.Link = fmt.Sprintf("https://plane.it4retail.tech/it4retail/browse/%s-%d/", task.ProjectIdentifier, task.SequenceID)
		tasks = append(tasks, task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func main() {
	// Создаем директорию для данных если её нет
	dataDir := "/app/data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	// Инициализируем PostgreSQL
	initDB()
	if err := loadUsers(); err != nil {
		log.Fatalf("Error loading users: %v", err)
	}

	// Инициализируем первую цитату
	quote, err := getQuote()
	if err != nil {
		log.Printf("Error getting initial quote: %v", err)
		// Устанавливаем дефолтную цитату в случае ошибки
		currentQuote = Quote{
			Text:   "Александр навайбкодил",
			Author: "Система",
		}
	} else {
		currentQuote = quote
	}

	// Запускаем планировщик обновления цитаты
	startQuoteScheduler()

	// Add new routes for attendance
	http.HandleFunc("/api/attendance", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request to /api/attendance", r.Method)
		log.Printf("Request headers: %v", r.Header)

		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet {
			statuses, err := getAttendanceStatus()
			if err != nil {
				log.Printf("Error getting attendance status: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			log.Printf("Found %d attendance records", len(statuses))
			if err := json.NewEncoder(w).Encode(statuses); err != nil {
				log.Printf("Error encoding attendance response: %v", err)
				http.Error(w, "Error encoding response", http.StatusInternalServerError)
				return
			}
			log.Printf("Successfully sent attendance response")
		} else if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Printf("Received POST data: %s", string(body))

			var data struct {
				UserID   string `json:"user_id"`
				IsOffice bool   `json:"is_office"`
				IsToday  bool   `json:"is_today"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				log.Printf("Error parsing JSON: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if data.UserID == "" {
				log.Printf("Error: user_id is required")
				http.Error(w, "user_id is required", http.StatusBadRequest)
				return
			}

			log.Printf("Updating attendance status for user %s: is_office=%v, is_today=%v",
				data.UserID, data.IsOffice, data.IsToday)

			if err := updateAttendanceStatus(data.UserID, data.IsOffice, data.IsToday); err != nil {
				log.Printf("Error updating attendance status: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			log.Printf("Successfully updated attendance status")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		}
	})

	// Обновляем обработчик для сохранения плана
	http.HandleFunc("/save-plan", savePlanHandler)

	// Обновляем обработчик для получения плана
	http.HandleFunc("/get-plan", getPlanHandler)

	// Добавляем обработчик для получения новых задач
	http.HandleFunc("/get-new-tasks", getNewTasksHandler)

	// Обновляем запросы для работы с планами
	http.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var plan struct {
				UserID string `json:"user_id"`
				Date   string `json:"date"`
				Plan   string `json:"plan"`
			}
			if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			query := `
				INSERT INTO plans (user_id, date, plan, created_at, updated_at)
				VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				ON CONFLICT (user_id, date) 
				DO UPDATE SET plan = $3, updated_at = CURRENT_TIMESTAMP
			`
			_, err := db.Exec(query, plan.UserID, plan.Date, plan.Plan)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else if r.Method == http.MethodGet {
			userID := r.URL.Query().Get("user_id")
			date := r.URL.Query().Get("date")

			var plan string
			err := db.QueryRow(`
				SELECT plan 
				FROM plans 
				WHERE user_id = $1 AND date = $2
			`, userID, date).Scan(&plan)

			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(map[string]string{"plan": plan})
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			userID := r.FormValue("user_id")
			if userID == "" {
				http.Error(w, "User ID is required", http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/?user_id="+userID, http.StatusSeeOther)
			return
		}
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}
		indexHandler(w, r)
	})

	http.HandleFunc("/update-attendance", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		isOffice := r.FormValue("is_office") == "true"
		isToday := r.FormValue("is_today") == "true"
		if err := updateAttendanceStatus(userID, isOffice, isToday); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/?user_id="+userID, http.StatusSeeOther)
	})

	http.HandleFunc("/get-attendance", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}
		status, err := getAttendanceStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	http.HandleFunc("/get-last-plan-date", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "User ID is required", http.StatusBadRequest)
			return
		}
		date, err := getLastPlanDate(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"date": date.Format(time.RFC3339)})
	})

	http.HandleFunc("/get-quote", func(w http.ResponseWriter, r *http.Request) {
		quoteMutex.RLock()
		quote := currentQuote
		quoteMutex.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quote)
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
