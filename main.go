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
		Task struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			DueDate     string `json:"due_date"`
			Priority    int    `json:"priority"`
			Identifier  string `json:"identifier"`
		} `json:"task"`
		Doer struct {
			Name string `json:"name"`
		} `json:"doer"`
	} `json:"data"`
}


// Handler for incoming webhook requests
func webhookHandler(dg *discordgo.Session, w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	formatMessage(dg, webhook)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Format the message for Discord
	message := fmt.Sprintf(
		"**New Task Created**\n\n**Title:** %s\n**Description:** %s\n**Due Date:** %s\n**Priority:** %d\n**Identifier:** %s\n**Created By:** %s",
		webhook.Data.Task.Title,
		webhook.Data.Task.Description,
		webhook.Data.Task.DueDate,
		webhook.Data.Task.Priority,
		webhook.Data.Task.Identifier,
		webhook.Data.Doer.Name,
	)

	// Send the message to Discord
	discordWebhookURL := "https://discordapp.com/api/webhooks/1280543843485352098/fyGeVmR-iuTjgrJOjAmCnbvaRA0SYfyT9ztUyKTLeVVKokzLiJWIhFBBfto0xJ1ka3pL"
	if err := sendToDiscord(discordWebhookURL, message); err != nil {
		http.Error(w, "Failed to send message to Discord", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Message sent to Discord")
}

func formatMessage(dg *discordgo.Session, webhook VikunjaWebhook) (string, error) {
	var project string
	index := strings.Index(webhook.Data.Task.Identifier, "-")
	if index != -1 {
		// Get the substring up to the found index
		project = webhook.Data.Task.Identifier[:index]
	} else {
		return "", errors.New("Not from known project")
	}
	fmt.Println(project)

	// Send message to a specific Discord channel
	_, err := dg.ChannelMessageSend("1280543736199123068", project)
	if err != nil {
		return "", fmt.Errorf("Failed to send message to Discord channel: %v", err)
	}

	return project, nil
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

func main() {
    err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		return
	}
    
    token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		fmt.Println("No token provided. Please set DISCORD_BOT_TOKEN in your .env file")
		return
	}
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer dg.Close() // Ensure Discord session is closed at the end

	// Start the HTTP server for handling webhooks
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			webhookHandler(dg, w, r) // Pass dg to the handler
		})
		fmt.Println("Server is running on port 4030")
		if err := http.ListenAndServe(":4030", nil); err != nil {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()

	// Send a test message to a specific channel
	dg.ChannelMessageSend("1280543736199123068", "Pong!")

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

