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

	switch webhook.EventName {
	case "task.created":
		if err := sendTaskCreated(dg, webhook, channelIDs); err != nil {
		    // http.Error(w, err.Error(), http.StatusInternalServerError)
      //       return
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

	fmt.Println(chanID)
	_, err = dg.ChannelMessageSend(chanID, message)
	if err != nil {
        return errors.New("Failed to send message to Discord")
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
		"**New Task Created <@&%s>**\n\n**Title:** %s\n**Created By:** %s ",
		chanID.RoleID,
		webhook.Data.Task.Title,
		webhook.Data.Doer.Name,
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
			webhookHandler(dg, w, r, &channelIDs) // Pass dg to the handler
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
