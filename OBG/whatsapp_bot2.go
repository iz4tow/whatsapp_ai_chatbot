package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/types"
	"io"
	"net/http"
	"fmt"
	"encoding/json"
	"strings"
	"flag"
	"bytes"
	"net"
)

var WhatsmeowClient *whatsmeow.Client
var wa_contact, password, model string

func main() {
	flag.StringVar(&wa_contact, "number", "", "Whatsapp contact number without +, e.g., 393312345654")
	flag.StringVar(&password, "password", "", "A secret word that allows any contact to receive sensor data")
	flag.StringVar(&model, "model", "llama3", "Select a model, e.g., deepseek-r1")
	flag.Parse()

	WhatsmeowClient = CreateClient()
	ConnectClient(WhatsmeowClient)
	WhatsmeowClient.AddEventHandler(HandleEvent)
	WhatsmeowClient.Connect()

	// Listen for Ctrl+C to gracefully shut down
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	WhatsmeowClient.Disconnect()
}

func CreateClient() *whatsmeow.Client {
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New("sqlite3", "file:accounts.db?_foreign_keys=on", dbLog)
	if err != nil {
		log.Fatalln(err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		log.Fatalln(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	return client
}

func ConnectClient(client *whatsmeow.Client) {
	if client.Store.ID == nil {
		// No ID stored, new login, show a QR code
		qrChan, _ := client.GetQRChannel(context.Background())
		err := client.Connect()
		if err != nil {
			log.Fatalln(err)
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				log.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err := client.Connect()
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func HandleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		go HandleMessage(v)
	}
}




var chatHistory []map[string]string // Stores conversation history

func ChatAI(prompt string) string {
	apiURL := "http://localhost:11434/api/chat"

	// Append the user's new message to the history
	chatHistory = append(chatHistory, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	// Create the full chat history payload
	payload := map[string]interface{}{
		"model":    model,
		"messages": chatHistory, // Send the full conversation history
		"stream":   false,       // Set to false to receive complete response at once
	}

	// Serialize payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Make the POST request
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatalf("Failed to make POST request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Parse response JSON
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Extract the assistant's response
	botResponse := ""
	if message, ok := result["message"].(map[string]interface{}); ok {
		if content, exists := message["content"].(string); exists {
			botResponse = content
		}
	}

	// Append the assistant's response to the chat history
	chatHistory = append(chatHistory, map[string]string{
		"role":    "assistant",
		"content": botResponse,
	})

	return botResponse
}





func IpConf() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("Error getting interfaces: %v\n", err)
		return "Error getting interfaces"
	}
	var response string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Printf("  Error getting addresses: %v\n", err)
			continue
		}
		response += "\n######################\nName: " + iface.Name + "\n"
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				response += "IP Address: " + v.IP.String() + "\n"
			case *net.IPAddr:
				response += "IP Address: " + v.IP.String() + "\n"
			}
		}
	}
	return response
}

func HandleMessage(messageEvent *events.Message) {
	recipientJID := types.NewJID(wa_contact, types.DefaultUserServer)
	var messageContent string

	if messageEvent.Message.Conversation != nil {
		messageContent = messageEvent.Message.GetConversation()
	} else if messageEvent.Message.ExtendedTextMessage != nil {
		messageContent = messageEvent.Message.ExtendedTextMessage.GetText()
	}

	if messageEvent.Info.Chat == recipientJID {
		switch strings.ToLower(messageContent) {
		case "help":
			reply := "Hi, I'm an AI assistant! Ask me anything."
			WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
				Conversation: &reply,
			})
		case "ip":
			reply := IpConf()
			WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
				Conversation: &reply,
			})
			cmd := exec.Command("curl", "ipinfo.io")
			output, _ := cmd.CombinedOutput()
			reply = string(output)
			WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
				Conversation: &reply,
			})
		case "reboot":
			reply := "Rebooting the system... please wait."
			WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
				Conversation: &reply,
			})
			cmd := exec.Command("reboot")
			cmd.Run()
		default:
			if password != "" && strings.HasPrefix(messageContent, password) {
				messageContent = messageContent[len(password):]
			}
			reply := ChatAI(messageContent)
			WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
				Conversation: &reply,
			})
		}
	}
}

