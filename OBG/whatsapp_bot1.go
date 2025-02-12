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
	waE2E "go.mau.fi/whatsmeow/proto/waE2E" // Updated import for waE2E
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/types"
	"bufio"
	"net/http"
	"fmt"
	"encoding/json"
	"strings"
	"flag"
	"bytes"
	"net"
)


var WhatsmeowClient *whatsmeow.Client
var wa_contact,password,model string

func main() {
	flag.StringVar(&wa_contact, "number","", "Whatsapp contact number whitout +, es 393312345654")
	flag.StringVar(&password,"password", "", "A secret word that allow any contact to receive sensor data")
	flag.StringVar(&model,"model", "llama3", "Select a model, es deepseek-r1")
	flag.Parse()
	WhatsmeowClient = CreateClient()
	ConnectClient(WhatsmeowClient)
	WhatsmeowClient.AddEventHandler(HandleEvent)
	WhatsmeowClient.Connect()
	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
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
		// No ID stored, new login, show a qr code
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


func ChatAI(prompt string)(string) {
	// Define the API URL
	apiURL := "http://localhost:11434/api/generate"

	// Create the payload
	//prompt := "Hello, how are you?"
	model := model//"llama3" // Specify the model name
	payload := map[string]string{
		"model":  model,
		"prompt": prompt,
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

	// Handle streaming or non-standard JSON response
	scanner := bufio.NewScanner(resp.Body)
	fmt.Println("Response from Ollama:")
	var response string
	for scanner.Scan() {
		line := scanner.Text()
		var parsedLine map[string]interface{}
		err := json.Unmarshal([]byte(line), &parsedLine)
		if err != nil {
			// If parsing fails, assume it's plain text
			fmt.Println(line)
			return "Internal AI error"
		} else if resp, ok := parsedLine["response"]; ok {
			// Print human-readable response if "response" key exists
			response=response+fmt.Sprintf("%v", resp) //resp is an interface{} type, so I must convert to string
		} else {
			// Print entire JSON object as fallback
			fmt.Print(parsedLine)
			return "Internal AI error"
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading response: %v", err)
		return "Fatal AI error"
	}
	return response//send back full response
}



func IpConf()(string) {
	// Get a list of all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("Error getting interfaces: %v\n", err)
		return "Error getting interfaces"
	}
	var response string
	for _, iface := range interfaces {
		fmt.Printf("Name: %s\n", iface.Name)
//		fmt.Printf("  MTU: %d\n", iface.MTU)
//		fmt.Printf("  Hardware Address: %s\n", iface.HardwareAddr)


		// Skip down interfaces or those that don't support multicast
		if iface.Flags&net.FlagUp == 0 {
//			fmt.Println("  Status: Down")
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
//			fmt.Println("  Type: Loopback")
			continue
		}

		// Get interface addresses
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Printf("  Error getting addresses: %v\n", err)
			continue
		}
		response=response+"\n######################\nName: "+iface.Name+"\n"
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				fmt.Printf("  IP Address: %s\n", v.IP.String())
				fmt.Printf("  Subnet Mask: %s\n", v.Mask.String())
				response=response+"IP Address: "+v.IP.String()+"\n"
			case *net.IPAddr:
				fmt.Printf("  IP Address: %s\n", v.IP.String())
				response=response+"IP Address: "+v.IP.String()+"\n"
			}
		}
	}
	return response
}

func HandleMessage(messageEvent *events.Message) {
	recipientJID := types.NewJID(wa_contact, types.DefaultUserServer) //types.DefaultUserServer automatically adds @s.whatsapp.net to the JID. es 393334455666
	var messageContent string
	if messageEvent.Message.Conversation != nil { //old whatsapp version
		messageContent = messageEvent.Message.GetConversation()
	} else if messageEvent.Message.ExtendedTextMessage != nil { //new whatsapp version
		messageContent = messageEvent.Message.ExtendedTextMessage.GetText()
	}
	if (messageEvent.Info.Chat==recipientJID && (messageContent != "ip" && messageContent != "IP") && (messageContent != "ip aqi" && messageContent != "IP AQI") && (messageContent != "reboot" && messageContent != "Reboot") && (messageContent != "help" && messageContent != "Help") && (messageContent != "status" && messageContent != "Status") && !strings.HasPrefix(messageContent,"LIVELLO") && !strings.HasPrefix(messageContent, "Remote sensor IP:") && !strings.HasPrefix(messageContent, "% Total    % Received") && !strings.HasPrefix(messageContent, "######################")){
		log.Println("Request received")
		log.Println(messageContent)
		reply := ChatAI(messageContent)
		WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
			Conversation: &reply,
		})
	}else if((messageContent == "help" || messageContent == "Help") && messageEvent.Info.Chat==recipientJID){
		reply:="Hi I'm an AI assistant, ask me something!"
		WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
			Conversation: &reply,
		})
	}else if((messageContent == "ip" || messageContent == "IP") && messageEvent.Info.Chat==recipientJID){
		reply:=IpConf()
		WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
			Conversation: &reply,
		})
		// Command to execute the reboot
		cmd := exec.Command("curl", "ipinfo.io")
		// Capture standard output and error
		output, _ := cmd.CombinedOutput()
		reply = string(output)
		WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
			Conversation: &reply,
		})
	}else if((messageContent == "reboot" || messageContent == "Reboot") && messageEvent.Info.Chat==recipientJID){
		reply:="Rebooting the system...please wait"
		WhatsmeowClient.SendMessage(context.Background(), recipientJID, &waE2E.Message{
			Conversation: &reply,
		})
		cmd := exec.Command("reboot")
		cmd.Run()
	}else if(strings.HasPrefix(messageContent,password) && password != ""){
		messageContent = messageContent[len(password):]
		reply := ChatAI(messageContent)
		WhatsmeowClient.SendMessage(context.Background(), messageEvent.Info.Chat, &waE2E.Message{
			Conversation: &reply,
		})
	}
}




