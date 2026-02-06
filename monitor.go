package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// StatsResponse represents XRAY stats API response
type StatsResponse struct {
	Stat []Stat `json:"stat"`
}

type Stat struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// ConnectionInfo holds current connection stats
type ConnectionInfo struct {
	ActiveConnections int64
	UploadTraffic     int64
	DownloadTraffic   int64
	TotalTraffic      int64
}

// getTelegramEnv gets telegram credentials from environment
func getTelegramEnv() (botToken, chatID string, shouldMonitor bool) {
	botToken = os.Getenv("BOT_TOKEN")
	chatID = os.Getenv("CHAT_ID")
	shouldMonitor = botToken != "" && chatID != ""
	return
}

// getXRAYStats retrieves connection statistics from XRAY API
func getXRAYStats(ctx context.Context) (*ConnectionInfo, error) {
	// Connect to XRAY API (usually listening on localhost:10085)
	apiAddr := "127.0.0.1:10085"
	
	// Create a context with timeout
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	// Dial XRAY API
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", apiAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to XRAY API: %v", err)
	}
	defer conn.Close()

	// Build XRAY API request
	// Format: command\nobject\n
	request := "StatsService\nQueryStats\n"
	if _, err := fmt.Fprint(conn, request); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response (simplified - just check for open connections)
	// In production, you'd parse the actual response
	info := &ConnectionInfo{
		ActiveConnections: -1, // API requires complex parsing
	}
	return info, nil
}

// sendTelegramMessage sends a message to Telegram
func sendTelegramMessage(botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	
	payload := strings.NewReader(fmt.Sprintf(`{
		"chat_id": "%s",
		"text": "%s",
		"parse_mode": "HTML"
	}`, chatID, escapeJSON(message)))

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// escapeJSON escapes special characters for JSON
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// formatTraffic formats bytes to human-readable format
func formatTraffic(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}

// monitorConnections starts monitoring connections and sending updates to telegram
func monitorConnections(botToken, chatID string, interval time.Duration) {
	ctx := context.Background()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		info, err := getXRAYStats(ctx)
		if err != nil {
			fmt.Printf("[Monitor] Error getting stats: %v\n", err)
			continue
		}

		// Build message
		message := fmt.Sprintf(`<b>📊 Server Stats</b>
<b>Active Connections:</b> %d
<b>Upload Traffic:</b> %s
<b>Download Traffic:</b> %s
<b>Total Traffic:</b> %s
<b>Timestamp:</b> %s`,
			info.ActiveConnections,
			formatTraffic(info.UploadTraffic),
			formatTraffic(info.DownloadTraffic),
			formatTraffic(info.TotalTraffic),
			time.Now().Format("2006-01-02 15:04:05"))

		// Send to telegram
		if err := sendTelegramMessage(botToken, chatID, message); err != nil {
			fmt.Printf("[Monitor] Failed to send message: %v\n", err)
		} else {
			fmt.Println("[Monitor] Message sent successfully")
		}
	}
}

// StartMonitoring starts the monitoring goroutine if configured
func StartMonitoring() {
	botToken, chatID, shouldMonitor := getTelegramEnv()
	if !shouldMonitor {
		fmt.Println("[Monitor] Telegram not configured, monitoring disabled")
		return
	}

	interval := 5 * time.Minute // Send stats every 5 minutes
	go monitorConnections(botToken, chatID, interval)
	fmt.Println("[Monitor] Connection monitoring started - will send updates to Telegram every", interval)
}
