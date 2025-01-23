package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type StreamerMetadata struct {
	ChannelName string    `json:"channelName"`
	Tags        []string  `json:"tags"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	Active      bool      `json:"active"`
	LastActive  bool      `json:"lastActive"`
}

var REDIS_DB_URL = "redis://default@127.0.0.1:6379"

func main() {
	ctx := context.Background()
	opt, err := redis.ParseURL(REDIS_DB_URL)
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(opt)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}

		var (
			ffmpegCmd   *exec.Cmd
			ffmpegIn    io.WriteCloser
			channelName string
		)

		defer func() {
			wsConn.Close()
			if channelName != "" {
				rdb.HSet(ctx, channelName, "active", false, "lastActive", time.Now())
			}
			if ffmpegIn != nil {
				ffmpegIn.Close()
			}
			if ffmpegCmd != nil && ffmpegCmd.Process != nil {
				ffmpegCmd.Process.Kill()
				ffmpegCmd.Wait()
			}

			log.Println("done with the cleanups in the defer function")
		}()

		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			if messageType == websocket.BinaryMessage {
				if ffmpegIn == nil {
					log.Println("FFmpeg not started, ignoring binary data")
					continue
				}
				if _, err := ffmpegIn.Write(data); err != nil {
					log.Println("Pipe write error:", err)
					return
				}
				continue
			}

			var requestData struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}

			if err := json.Unmarshal(data, &requestData); err != nil {
				log.Println("Unmarshal JSON error:", err)
				continue
			}

			switch requestData.Type {
			case "start":
				// Cleanup existing FFmpeg process
				if ffmpegCmd != nil {
					ffmpegIn.Close()
					ffmpegCmd.Wait()
					ffmpegCmd = nil
				}

				var metadata StreamerMetadata
				if err := json.Unmarshal(requestData.Data, &metadata); err != nil {
					log.Println("Metadata unmarshal error:", err)
					wsConn.WriteJSON(map[string]string{"error": "invalid start data"})
					continue
				}

				if metadata.ChannelName == "" {
					wsConn.WriteJSON(map[string]string{"error": "channelName is required"})
					continue
				}

				channelName = metadata.ChannelName

				// Initialize FFmpeg
				ffmpegCmd = exec.Command("ffmpeg",
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
					"rtmp://localhost/live/"+channelName, // use channel name
				)

				ffmpegCmd.Stderr = os.Stderr
				ffmpegIn, err = ffmpegCmd.StdinPipe()
				if err != nil {
					log.Println("StdinPipe error:", err)
					wsConn.WriteJSON(map[string]string{"error": "internal error"})
					return
				}

				if err := ffmpegCmd.Start(); err != nil {
					log.Println("FFmpeg start error:", err)
					wsConn.WriteJSON(map[string]string{"error": "internal error"})
					return
				}

				// Update Redis
				metadataMap := map[string]interface{}{
					"channelName": channelName,
					"title":       metadata.Title,
					"description": metadata.Description,
					"createdAt":   time.Now(),
					"active":      true,
					"startedAt":   time.Now(),
					"viewers":     0,
					"tags":        strings.Join(metadata.Tags, ","),
				}
				if _, err := rdb.HSet(ctx, channelName, metadataMap).Result(); err != nil {
					log.Println("Redis error:", err)
					wsConn.WriteJSON(map[string]string{"error": "server error"})
					return
				}

				wsConn.WriteJSON(map[string]string{"type": "stream-start", "data": "start"})

			case "stop":
				if ffmpegCmd != nil {
					ffmpegIn.Close()
					ffmpegCmd.Wait()
					ffmpegCmd = nil
				}
				if channelName != "" {
					rdb.HDel(ctx, channelName)
				}
				wsConn.WriteJSON(map[string]string{"type": "stream-stop", "data": "stopped"})
			}
		}
	})

	log.Println("Server running on :5000")
	http.ListenAndServe(":5000", nil)
}
