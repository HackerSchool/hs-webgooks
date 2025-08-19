package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Define the structure for the Vikunja webhook payload
type VikunjaWebhook struct {
	EventName string `json:"event_name"`
	Time      string `json:"time"`
	Data      struct {
		Doer struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"doer"`
		Task struct {
			ID          int     `json:"id"`
			Title       string  `json:"title"`
			Description string  `json:"description"`
			Done        bool    `json:"done"`
			DoneAt      string  `json:"done_at"`
			DueDate     string  `json:"due_date"`
			ProjectID   int     `json:"project_id"`
			Priority    int     `json:"priority"`
			Identifier  string  `json:"identifier"`
			PercentDone float64 `json:"percent_done"`
			Created     string  `json:"created"`
			Updated     string  `json:"updated"`
			Assignees   []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"assignees"`
			CreatedBy struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"created_by"`
		} `json:"task"`
	} `json:"data"`
}

// Define a struct to hold both the channel ID and the role ID
type ChannelInfo struct {
	ChannelID string `json:"channel_id"`
	RoleID    string `json:"role_id"`
}

// Handler for incoming webhook requests
func webhookHandler(dg *discordgo.Session, w http.ResponseWriter, r *http.Request, channelIDs *map[string]ChannelInfo) {
	fmt.Println("Yo, at the webhook handler")
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Read and parse the JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var webhook VikunjaWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		// http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		// return
	}

	fmt.Printf("📨 Received event: %s\n", webhook.EventName)
	fmt.Printf("📋 Full webhook data:\n")
	prettyJSON, _ := json.MarshalIndent(webhook, "", "  ")
	fmt.Printf("%s\n", string(prettyJSON))

	switch webhook.EventName {
	case "task.created":
		if err := sendTaskCreated(dg, webhook, channelIDs); err != nil {
			// http.Error(w, err.Error(), http.StatusInternalServerError)
			//       return
		}
	case "task.updated":
		fmt.Printf("🔍 Task updated - Done status: %v\n", webhook.Data.Task.Done)
		// Check if task was marked as done
		if webhook.Data.Task.Done {
			fmt.Printf("✅ Task marked as completed! Processing...\n")
			if err := handleTaskCompleted(webhook, channelIDs); err != nil {
				fmt.Printf("❌ Error handling task completion: %v\n", err)
			}
		} else {
			fmt.Printf("⏳ Task not completed yet (done = false)\n")
		}
	default:
		// http.Error(w, "Not Implemented", http.StatusInternalServerError)
		// return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Message sent to Discord")
}

func sendTaskCreated(dg *discordgo.Session, webhook VikunjaWebhook, channelIDs *map[string]ChannelInfo) error {
	message, chanID, err := formatMessage(dg, webhook, channelIDs)
	if err != nil {
		return fmt.Errorf("Error reading channel IDs")
	}

	// fmt.Println(chanID)
	if dg == nil {
		// Discord disabled; skip sending message but consider it successful
		return nil
	}
	_, err = dg.ChannelMessageSend(chanID, message)
	if err != nil {
		return errors.New("Failed to send message to Discord")
	}
	return nil

}

// Function to handle completed tasks and send data to Hacker League API
func handleTaskCompleted(webhook VikunjaWebhook, channelIDs *map[string]ChannelInfo) error {
	// Extract project name from task identifier
	project := extractProjectName(webhook.Data.Task.Identifier)
	if project == "" {
		return fmt.Errorf("Could not extract project name from identifier: %s", webhook.Data.Task.Identifier)
	}

	// Get member names from assignees, or use created_by if no assignees
	var memberNames []string
	if len(webhook.Data.Task.Assignees) > 0 {
		for _, assignee := range webhook.Data.Task.Assignees {
			memberNames = append(memberNames, assignee.Name)
		}
	} else {
		memberNames = []string{webhook.Data.Task.CreatedBy.Name}
	}

	// Prepare data for Hacker League API
	apiData := map[string]interface{}{
		"time":        webhook.Time, // Webhook timestamp
		"assignees":   memberNames,  // Names of people to award points to
		"description": webhook.Data.Task.Description,
		"title":       webhook.Data.Task.Title,
		"project":     project, // Project name (hsdev, hsmkt, etc.)
		"points":      30,      // Fixed PCC value per completed task
		"task_id":     webhook.Data.Task.ID,
		"list_id":     webhook.Data.Task.ProjectID,
	}

	// Log what we're sending to the API
	fmt.Printf("🎯 Sending to Hacker League API:\n")
	apiJSON, _ := json.MarshalIndent(apiData, "", "  ")
	fmt.Printf("%s\n", string(apiJSON))

	// Send POST to Hacker League API
	if err := sendToHackerLeagueAPI(apiData); err != nil {
		return fmt.Errorf("Failed to send to Hacker League API: %v", err)
	}

	fmt.Printf("Task completed: %s in project %s by %s - 30 points awarded\n",
		webhook.Data.Task.Title, project, strings.Join(memberNames, ", "))
	return nil
}

// Helper function to extract project name from task identifier
func extractProjectName(identifier string) string {
	index := strings.Index(identifier, "-")
	if index != -1 {
		return identifier[:index]
	}
	return ""
}

// Function to send data to Hacker League API
func sendToHackerLeagueAPI(data map[string]interface{}) error {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Failed to marshal JSON: %v", err)
	}

	// Get API URL from environment variable or use default
	apiURL := os.Getenv("HACKER_LEAGUE_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000/tasks" // Default to our mock API
	}

	// Send POST request to Hacker League API
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	return nil
}

func formatMessage(dg *discordgo.Session, webhook VikunjaWebhook, channelIDs *map[string]ChannelInfo) (string, string, error) {
	var project string

	index := strings.Index(webhook.Data.Task.Identifier, "-")
	if index != -1 {
		// Get the substring up to the found index
		project = webhook.Data.Task.Identifier[:index]
	} else {
		return "", "", errors.New("Not from known project")
	}
	fmt.Println(project)

	// Send message to a specific Discord channel
	chanID, exists := (*channelIDs)[project]
	if !exists {
		return "", "", fmt.Errorf("No project id found")
	}

	// Format the message for Discord
	message := fmt.Sprintf(
		"## **New Task Created <@&%s>**\n\n"+
			"**Task:** [%s](https://tasks.hackerschool.dev/tasks/%d)\n"+
			"[**View Project**](https://tasks.hackerschool.dev/projects/%d)\n"+
			"**Created By:** %s",
		chanID.RoleID,
		webhook.Data.Task.Title,
		webhook.Data.Task.ID,
		webhook.Data.Task.ProjectID,
		webhook.Data.Task.CreatedBy.Name,
	)

	return message, chanID.ChannelID, nil
}

// Function to send a message to Discord
func sendToDiscord(webhookURL, message string) error {
	payload := map[string]string{"content": message}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message, status code: %d", resp.StatusCode)
	}

	return nil
}

// Function to load the channel information from a JSON file and return a map of structs
func loadChannelIDs(filename string) (map[string]ChannelInfo, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Read the file contents
	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Declare the map to hold the channel information (channel_id and role_id)
	channelMap := make(map[string]ChannelInfo)

	// Unmarshal the JSON into the map
	err = json.Unmarshal(byteValue, &channelMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Return the map and nil error if everything is successful
	return channelMap, nil
}

func main() {

	// Call the function to load channel IDs
	channelIDs, err := loadChannelIDs("channels.json")
	if err != nil {
		fmt.Println("Error loading channel IDs:", err)
		return
	}

	err = godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found; continuing without it")
	}

	// COMMENTED OUT DISCORD FUNCTIONALITY FOR NOW
	/*
		token := os.Getenv("DISCORD_BOT_TOKEN")
		if token == "" {
			fmt.Println("No token provided. Discord features disabled. Set DISCORD_BOT_TOKEN in your .env to enable Discord.")
		}
		var dg *discordgo.Session
		var dgErr error
		if token != "" {
			dg, dgErr = discordgo.New("Bot " + token)
			if dgErr != nil {
				fmt.Println("error creating Discord session,", err)
			} else {
				// Open a websocket connection to Discord and begin listening.
				dgErr = dg.Open()
				if dgErr != nil {
					fmt.Println("error opening connection,", dgErr)
				} else {
					defer dg.Close() // Ensure Discord session is closed at the end
				}
			}
		}
	*/
	var dg *discordgo.Session // Set to nil for now
	fmt.Println("Discord functionality commented out - webhook processing only")

	// Start the mock API server for testing
	mockAPI := NewMockAPIServer("3000")
	mockAPI.Start()

	// Start the HTTP server for handling webhooks
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			webhookHandler(dg, w, r, &channelIDs) // Pass dg to the handler
		})
		fmt.Println("🌐 Webhook server running on port 4030")
		fmt.Println("📋 Ready to receive Vikunja webhooks!")
		fmt.Println("   - POST to http://localhost:4030/ to test")
		fmt.Println("   - Mock API running on http://localhost:3000/tasks")
		if err := http.ListenAndServe(":4030", nil); err != nil {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()

	// Send a test message to a specific channel only if Discord is enabled and session is open
	// COMMENTED OUT FOR NOW
	/*
		if dg != nil && dgErr == nil {
			dg.ChannelMessageSend("1280543736199123068", "Pong!")
		}
	*/

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
