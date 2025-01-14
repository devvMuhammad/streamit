package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		defer wsConn.Close()

		// Start FFmpeg process
		ffmpegCmd := exec.Command("ffmpeg",
			"-f", "webm",
			"-i", "pipe:0",
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-tune", "zerolatency",
			"-c:a", "aac",
			"-ar", "44100",
			"-b:a", "128k",
			"-pix_fmt", "yuv420p",
			"-f", "flv",
			"rtmp://localhost/live/stream",
		)
		ffmpegCmd.Stderr = os.Stderr

		ffmpegIn, err := ffmpegCmd.StdinPipe()
		if err != nil {
			log.Println("Failed creating pipe:", err)
			return
		}

		if err := ffmpegCmd.Start(); err != nil {
			log.Println("Failed starting FFmpeg:", err)
			return
		}

		// Handle incoming WebSocket messages
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			// Only process binary messages
			if messageType == websocket.BinaryMessage {
				if _, err := ffmpegIn.Write(data); err != nil {
					log.Println("Pipe write error:", err)
					wsConn.Close()
					break
				}
			}
		}
	})

	log.Println("Server running on :5000")
	http.ListenAndServe(":5000", nil)
}
