package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/patrickmn/go-cache"
	"github.com/robfig/cron/v3"
)

var (
	db         *sql.DB
	appCache   *cache.Cache
	apiURL     = os.Getenv("API_URL")
	xTenant    = os.Getenv("X_Tenant")
	statsWeeks = 8
)

type StatRecord struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
	Weight    int     `json:"weight,omitempty"`
}

func init() {
	if sw := os.Getenv("STATS_WEEKS"); sw != "" {
		if val, err := strconv.Atoi(sw); err == nil {
			statsWeeks = val
		}
	}
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "file:./db/data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize tables
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS stats (timestamp TEXT PRIMARY KEY, value REAL, weight INTEGER);
					  CREATE TABLE IF NOT EXISTS history (timestamp TEXT PRIMARY KEY, value REAL);
					  CREATE INDEX IF NOT EXISTS idx_history_timestamp ON history (timestamp);`)
	if err != nil {
		log.Fatal(err)
	}

	appCache = cache.New(15*time.Minute, 30*time.Minute)

	// Cron job
	c := cron.New()
	c.AddFunc("*/15 * * * *", fetchData)
	c.Start()
	defer c.Stop()

	// Gin setup)
	r := gin.Default()
	r.Use(ETag())

	weekdays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for _, day := range weekdays {
		r.GET("/"+day, handleWeekday(day))
	}
	r.GET("/today", handleWeekday("today"))

	r.GET("/date/:dateString", handleDate)
	r.GET("/live", handleLive)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3030"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		IdleTimeout:  90 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Using %d weeks for stats", statsWeeks)
		log.Printf("GYM Tracker listening at http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	if err := server.Close(); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}

func fetchData() {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return
	}
	req.Header.Set("X-Tenant", xTenant)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error fetching data:", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Value float64 `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("Error decoding response:", err)
		return
	}

	now := time.Now()
	timestamp := now.Format("Monday, 15:04")
	historyTimestamp := now.Format("2006/01/02 15:04")

	var existingValue float64
	var existingWeight int
	err = db.QueryRow("SELECT value, weight FROM stats WHERE timestamp = ?", timestamp).Scan(&existingValue, &existingWeight)

	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO stats (timestamp, value, weight) VALUES (?, ?, ?)", timestamp, result.Value, 1)
	} else if err == nil {
		newWeight := existingWeight + 1
		averageValue := (existingValue*float64(existingWeight) + result.Value) / float64(newWeight)
		_, err = db.Exec("UPDATE stats SET value = ?, weight = ? WHERE timestamp = ?", averageValue, newWeight, timestamp)
	}

	if err != nil {
		log.Println("Error updating stats:", err)
	}

	_, err = db.Exec("INSERT INTO history (timestamp, value) VALUES (?, ?)", historyTimestamp, result.Value)
	if err != nil {
		log.Println("Error inserting history:", err)
	}

	appCache.Flush()
	log.Println("Data stored successfully for timestamp:", timestamp)
}

func handleWeekday(day string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if val, found := appCache.Get(day); found {
			c.JSON(http.StatusOK, val)
			return
		}

		var results []StatRecord
		var err error

		if day == "today" {
			todaysDate := time.Now().Format("2006/01/02")
			results, err = queryHistory(todaysDate + "%")
		} else {
			results, err = getAverageOfLastXWeeks(day)
		}

		if err != nil {
			log.Println("Error fetching data:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal server error occurred."})
			return
		}

		appCache.Set(day, results, cache.DefaultExpiration)
		c.JSON(http.StatusOK, results)
	}
}

func queryHistory(pattern string) ([]StatRecord, error) {
	rows, err := db.Query("SELECT timestamp, value FROM history WHERE timestamp LIKE ? ORDER BY timestamp", pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StatRecord
	for rows.Next() {
		var r StatRecord
		if err := rows.Scan(&r.Timestamp, &r.Value); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func getAverageOfLastXWeeks(day string) ([]StatRecord, error) {
	targetWeekday := -1
	weekdays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for i, d := range weekdays {
		if d == day {
			targetWeekday = i
			break
		}
	}

	if targetWeekday == -1 {
		return []StatRecord{}, nil
	}

	now := time.Now()
	todayWeekday := int(now.Weekday())
	diff := targetWeekday - todayWeekday
	if diff >= 0 {
		diff -= 7
	}

	lastWeekdayDate := now.AddDate(0, 0, diff)

	var patterns []string
	var args []interface{}
	for i := 0; i < statsWeeks; i++ {
		date := lastWeekdayDate.AddDate(0, 0, -7*i)
		patterns = append(patterns, "timestamp LIKE ?")
		args = append(args, date.Format("2006/01/02")+"%")
	}

	query := fmt.Sprintf("SELECT timestamp, value FROM history WHERE %s ORDER BY timestamp", strings.Join(patterns, " OR "))
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type agg struct {
		sum   float64
		count int
	}
	statsMap := make(map[string]*agg)
	var keys []string

	for rows.Next() {
		var timestamp string
		var value float64
		if err := rows.Scan(&timestamp, &value); err != nil {
			return nil, err
		}

		t, err := time.Parse("2006/01/02 15:04", timestamp)
		if err != nil {
			continue
		}
		groupKey := t.Format("Monday, 15:04")
		if _, ok := statsMap[groupKey]; !ok {
			statsMap[groupKey] = &agg{}
			keys = append(keys, groupKey)
		}
		statsMap[groupKey].sum += value
		statsMap[groupKey].count++
	}

	var results []StatRecord
	// We want to keep some order, but the original was just Object.entries which is usually insertion order or arbitrary.
	// Let's just return them.
	for _, k := range keys {
		results = append(results, StatRecord{
			Timestamp: k,
			Value:     statsMap[k].sum / float64(statsMap[k].count),
		})
	}

	return results, nil
}

func handleDate(c *gin.Context) {
	dateString := c.Param("dateString")
	cacheKey := "date-" + dateString

	if val, found := appCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, val)
		return
	}

	t, err := time.Parse("2006-01-02", dateString)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Please use YYYY-MM-DD."})
		return
	}

	formattedDate := t.Format("2006/01/02")
	results, err := queryHistory(formattedDate + "%")
	if err != nil {
		log.Println("Error fetching data:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal server error occurred."})
		return
	}

	expiration := 10 * time.Minute
	if t.Before(time.Now().Truncate(24 * time.Hour)) {
		expiration = 24 * time.Hour
	}

	appCache.Set(cacheKey, results, expiration)
	c.JSON(http.StatusOK, results)
}

func handleLive(c *gin.Context) {
	if val, found := appCache.Get("live"); found {
		c.JSON(http.StatusOK, val)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal server error occurred."})
		return
	}
	req.Header.Set("X-Tenant", xTenant)

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal server error occurred."})
		return
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal server error occurred."})
		return
	}

	appCache.Set("live", result, 1*time.Minute)
	c.JSON(http.StatusOK, result)
}

func ETag() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}

		// Buffer the response
		bw := &bodyWriter{body: bytes.NewBuffer(nil), ResponseWriter: c.Writer}
		c.Writer = bw
		c.Next()

		if bw.Status() == http.StatusOK && bw.body.Len() > 0 {
			// Calculate ETag from body content
			hash := crc32.ChecksumIEEE(bw.body.Bytes())
			etag := fmt.Sprintf("W/\"%x\"", hash)
			c.Header("ETag", etag)

			// Check if ETag matches If-None-Match header
			if c.GetHeader("If-None-Match") == etag {
				c.Status(http.StatusNotModified)
				bw.body.Reset() // Don't send the body
			}
		}

		// Set the final Content-Length and write the body
		c.Header("Content-Length", strconv.Itoa(bw.body.Len()))
		bw.ResponseWriter.Write(bw.body.Bytes())
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *bodyWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}
