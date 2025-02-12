#/bin/bash
read -p "Insert Whatsapp number with country code without + (es: italian number 3334455666 -> 393334455666): " whats_number
sudo apt update
sudo apt upgrade -y
sudo apt install curl -y
curl -fsSL https://ollama.com/install.sh | sh
mkdir -p /opt/chatbot/
mv whatsapp_bot /opt/chatbot/

#Check ping
sudo tee /opt/chatbot/check_ping.sh <<EOF
#!/bin/bash
if curl whatsapp.com > /dev/null; then
    exit 0  # Successful
else
    exit 1  # Failed
fi
EOF
chmod +x /opt/chatbot/check_ping.sh

#Create chatbot service
sudo tee /etc/systemd/system/chatbot.service <<EOF
[Unit]
Description=Franco WhatsApp chatbot
After=network.target auditd.service

[Service]
ExecStartPre=/usr/bin/sleep 10
ExecStartPre=/opt/chatbot/check_ping.sh
WorkingDirectory=/opt/chatbot/
ExecStart=/opt/chatbot/whatsapp_bot -number $whats_number -password "robot "
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
cd /opt/chatbot
timeout 120 ./whatsapp_bot
ollama pull llama3
sudo systemctl enable --now chatbot
