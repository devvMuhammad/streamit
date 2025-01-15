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
		// Start FFmpeg process
		ffmpegCmd := exec.Command("ffmpeg",
			"-f", "webm",
			"-i", "pipe:0",
			"-c:v", "libx264",
			"-preset", "medium",
			"-tune", "zerolatency",
			"-c:a", "aac",
			"-ar", "44100",
			"-fflags", "nobuffer",
			"-rtbufsize", "1500M",
			"-b:a", "128k",
			"-pix_fmt", "yuv420p",
			"-f", "flv",
			"rtmp://localhost/live/stream",
		)
		ffmpegCmd.Stderr = os.Stderr

		ffmpegIn, err := ffmpegCmd.StdinPipe()

		defer func() {
			ffmpegIn.Close()
			ffmpegCmd.Process.Kill()
			ffmpegCmd.Wait()
		}()

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

			// Binary message means it is for streaming
			if messageType == websocket.BinaryMessage {
				if _, err := ffmpegIn.Write(data); err != nil {
					log.Println("Pipe write error:", err)
					wsConn.Close()
					return
				}

				continue
			}

			// If not binary, assume it is a JSON message so unmarshal it
			var requestData struct {
				Type string      `json:"type"`
				Data interface{} `json:"data"`
			}

			if err := json.Unmarshal(data, &requestData); err != nil {
				log.Println("Unmarshal JSON error:", err)
				continue
			}

			if requestData.Type == "start" {

				// add to the hashmap
				streamerMetadata := requestData.Data.(StreamerMetadata)

				streams[streamerMetadata.UserId] = streamerMetadata

			} else if requestData.Type == "stop" {

				// remove entry from the hashmap
				delete(streams, requestData.Data.(StreamerMetadata).UserId)

			}

		}
	})

	log.Println("Server running on :5000")
	http.ListenAndServe(":5000", nil)
}
