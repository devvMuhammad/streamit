package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var upgraderOld = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func mainOld() {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		defer wsConn.Close()

		pc, err := webrtc.NewPeerConnection(config)
		if err != nil {
			log.Println("PeerConnection error:", err)
			return
		}
		defer pc.Close()

		// Send ICE candidates to client
		pc.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}
			candidateJSON, _ := json.Marshal(c.ToJSON())
			wsConn.WriteJSON(map[string]interface{}{
				"type":      "candidate",
				"candidate": string(candidateJSON),
			})
		})

		// Handle incoming data channel
		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			log.Println("New DataChannel:", dc.Label())

			if dc.Label() != "media" {
				return
			}

			// Start FFmpeg (expecting WebM from stdin)
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

			// Write incoming chunks to FFmpeg
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				if msg.IsString {
					return
				}
				if _, err := ffmpegIn.Write(msg.Data); err != nil {
					log.Println("Pipe write error:", err)
				}
			})

			dc.OnClose(func() {
				ffmpegIn.Close()
				ffmpegCmd.Wait()
			})
		})

		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				continue
			}
			fmt.Println("request type", data["type"])

			switch data["type"] {
			case "offer":
				sdp, ok := data["sdp"].(string)
				if !ok {
					log.Println("No SDP in offer")
					continue
				}
				if err := pc.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.SDPTypeOffer,
					SDP:  sdp,
				}); err != nil {
					log.Println("SetRemoteDescription error:", err)
					continue
				}

				answer, err := pc.CreateAnswer(nil)
				if err != nil {
					log.Println("CreateAnswer error:", err)
					continue
				}
				if err = pc.SetLocalDescription(answer); err != nil {
					log.Println("SetLocalDescription error:", err)
					continue
				}

				wsConn.WriteJSON(map[string]interface{}{
					"type": "answer",
					"sdp":  answer.SDP,
				})
			}
		}
	})

	log.Println("Server running on :5000")
	http.ListenAndServe(":5000", nil)
}
