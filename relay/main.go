package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type StreamerMetadata struct {
	UserId      string    `json:"userId"`
	Tags        []string  `json:"tags"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	Active      bool      `json:"active"`
	LastActive  bool      `json:"lastActive"`
}

// global variable for ongoing streams
var streams = make(map[string]StreamerMetadata)

func main() {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		defer wsConn.Close()

		// Start FFmpeg process with corrected parameters
		ffmpegCmd := exec.Command("ffmpeg",
			"-i", "pipe:0",
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-tune", "zerolatency",
			"-profile:v", "baseline",
			"-level", "3.0",
			"-pix_fmt", "yuv420p",
			"-r", "30",
			"-g", "60",
			"-c:a", "aac",
			"-ar", "44100",
			"-b:a", "128k",
			"-b:v", "2500k",
			"-maxrate", "2500k",
			"-bufsize", "5000k",
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

		defer func() {
			ffmpegIn.Close()
			if ffmpegCmd.Process != nil {
				ffmpegCmd.Process.Kill()
			}
			ffmpegCmd.Wait()
		}()

		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := ffmpegIn.Write(data); err != nil {
					log.Println("Pipe write error:", err)
					return
				}
				continue
			}

			// Handle metadata messages
			var requestData struct {
				Type string           `json:"type"`
				Data StreamerMetadata `json:"data"`
			}

			if err := json.Unmarshal(data, &requestData); err != nil {
				log.Println("Unmarshal JSON error:", err)
				continue
			}

			switch requestData.Type {
			case "start":
				streams[requestData.Data.UserId] = requestData.Data
			case "stop":
				delete(streams, requestData.Data.UserId)
			}
		}
	})

	log.Println("Server running on :5000")
	http.ListenAndServe(":5000", nil)
}
